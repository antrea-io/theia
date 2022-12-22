// Copyright 2022 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package supportbundle

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/afero"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	clientrest "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	systemv1alpha1 "antrea.io/theia/pkg/apis/system/v1alpha1"
	"antrea.io/theia/pkg/support"
	"antrea.io/theia/pkg/util/env"
)

const (
	bundleExpireDuration = time.Hour
	bundleName           = "theia-manager"
	clickhouseLabel      = "app=clickhouse"
	flowAggregatorLabel  = "app=flow-aggregator"
	flowAggregatorNS     = "flow-aggregator"
	grafanaLabel         = "app=grafana"
	sparkDriverLabel     = "spark-role=driver"
	sparkExecLabel       = "spark-role=executor"
	sparkOperatorLabel   = "app.kubernetes.io/name=spark-operator"
	zookeeperLabel       = "app=zookeeper"
)

var (
	// Declared as variables for testing.
	defaultFS        = afero.NewOsFs()
	newManagerDumper = support.NewManagerDumper
)

// NewSupportBundleStorage creates a support bundle storage for working on theia manager.
func NewSupportBundleStorage(config *clientrest.Config, clientset kubernetes.Interface) Storage {
	namespace := env.GetTheiaNamespace()
	cpConfig := clientrest.CopyConfig(config)
	cpConfig.APIPath = "/api"
	cpConfig.GroupVersion = &schema.GroupVersion{Version: "v1"}
	cpConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: clientscheme.Codecs}

	bundle := &supportBundleREST{
		cache: &systemv1alpha1.SupportBundle{
			ObjectMeta: metav1.ObjectMeta{Name: bundleName},
			Status:     systemv1alpha1.SupportBundleStatusNone,
		},
		namespace:  namespace,
		restConfig: cpConfig,
		clientSet:  clientset,
	}
	return Storage{
		SupportBundle: bundle,
		Download:      &downloadREST{supportBundle: bundle},
	}
}

// Storage contains REST resources for support bundle, including status query and download.
type Storage struct {
	SupportBundle *supportBundleREST
	Download      *downloadREST
}

var (
	_ rest.Scoper          = &supportBundleREST{}
	_ rest.Getter          = &supportBundleREST{}
	_ rest.Creater         = &supportBundleREST{}
	_ rest.GracefulDeleter = &supportBundleREST{}
)

// supportBundleREST implements REST interfaces for bundle status querying.
type supportBundleREST struct {
	statusLocker sync.RWMutex
	cancelFunc   context.CancelFunc
	cache        *systemv1alpha1.SupportBundle

	namespace  string
	restConfig *clientrest.Config
	clientSet  kubernetes.Interface
}

// Create triggers a bundle generation. Returns metav1.Status if there is any error,
// otherwise it returns the SupportBundle.
func (r *supportBundleREST) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ *metav1.CreateOptions) (runtime.Object, error) {
	requestBundle := obj.(*systemv1alpha1.SupportBundle)
	if requestBundle.Name != bundleName {
		return nil, errors.NewForbidden(systemv1alpha1.SupportBundleResource.GroupResource(), requestBundle.Name,
			fmt.Errorf("only resource name \"%s\" is allowed", bundleName))
	}
	r.statusLocker.Lock()
	defer r.statusLocker.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	ctx, cancelFunc := context.WithCancel(context.Background())
	r.cache = &systemv1alpha1.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{Name: requestBundle.Name},
		Since:      requestBundle.Since,
		Status:     systemv1alpha1.SupportBundleStatusCollecting,
	}
	r.cancelFunc = cancelFunc
	go func(since string) {
		var err error
		var b *systemv1alpha1.SupportBundle
		b, err = r.collectController(ctx, since)
		func() {
			r.statusLocker.Lock()
			defer r.statusLocker.Unlock()
			if err != nil {
				klog.Errorf("Error when collecting supportBundle: %v", err)
				r.cache = &systemv1alpha1.SupportBundle{
					ObjectMeta: metav1.ObjectMeta{Name: bundleName},
					Status:     systemv1alpha1.SupportBundleStatusNone,
				}
				return
			}
			select {
			case <-ctx.Done():
			default:
				r.cache = b
			}
		}()

		if err == nil {
			r.clean(ctx, b.Filepath, bundleExpireDuration)
		}
	}(r.cache.Since)

	return r.cache, nil
}

func (r *supportBundleREST) New() runtime.Object {
	return &systemv1alpha1.SupportBundle{}
}

// Get returns current status of the bundle. It only allows querying the resource
// whose name is equal to the mode.
func (r *supportBundleREST) Get(_ context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	r.statusLocker.RLock()
	defer r.statusLocker.RUnlock()
	if name != bundleName {
		return nil, errors.NewNotFound(systemv1alpha1.Resource("supportBundle"), name)
	}
	if r.cache == nil {
		r.cache = &systemv1alpha1.SupportBundle{
			ObjectMeta: metav1.ObjectMeta{Name: bundleName},
			Status:     systemv1alpha1.SupportBundleStatusNone,
		}
	}
	return r.cache, nil
}

// Delete can remove the current finished bundle or cancel a running bundle
// collecting. It only allows querying the resource whose name is equal to the mode.
func (r *supportBundleREST) Delete(_ context.Context, name string, _ rest.ValidateObjectFunc, _ *metav1.DeleteOptions) (runtime.Object, bool, error) {
	if name != bundleName {
		return nil, false, errors.NewNotFound(systemv1alpha1.Resource("supportBundle"), name)
	}
	r.statusLocker.Lock()
	defer r.statusLocker.Unlock()
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	r.cache = &systemv1alpha1.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{Name: bundleName},
		Status:     systemv1alpha1.SupportBundleStatusNone,
	}
	return nil, true, nil
}

func (r *supportBundleREST) NamespaceScoped() bool {
	return false
}

func (r *supportBundleREST) collect(ctx context.Context, dumpers ...func(string) error) (*systemv1alpha1.SupportBundle, error) {
	basedir, err := afero.TempDir(defaultFS, "", "bundle_tmp_")
	if err != nil {
		return nil, fmt.Errorf("error when creating tempdir: %w", err)
	}
	defer defaultFS.RemoveAll(basedir)
	for _, dumper := range dumpers {
		if err := dumper(basedir); err != nil {
			return nil, err
		}
	}
	outputFile, err := afero.TempFile(defaultFS, "", "bundle_*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("error when creating output tarfile: %w", err)
	}
	defer outputFile.Close()
	hashSum, err := packDir(basedir, outputFile)
	if err != nil {
		return nil, fmt.Errorf("error when packing supportBundle: %w", err)
	}

	select {
	case <-ctx.Done():
		_ = defaultFS.Remove(outputFile.Name())
		return nil, fmt.Errorf("collecting is canceled")
	default:
	}
	stat, err := outputFile.Stat()
	var fileSize int64
	if err == nil {
		fileSize = stat.Size()
	}
	creationTime := metav1.Now()
	deletionTime := metav1.NewTime(creationTime.Add(bundleExpireDuration))
	return &systemv1alpha1.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:              bundleName,
			CreationTimestamp: creationTime,
			DeletionTimestamp: &deletionTime,
		},
		Status:   systemv1alpha1.SupportBundleStatusCollected,
		Sum:      fmt.Sprintf("%x", hashSum),
		Size:     uint32(fileSize),
		Filepath: outputFile.Name(),
	}, nil
}

func (r *supportBundleREST) collectController(ctx context.Context, since string) (*systemv1alpha1.SupportBundle, error) {
	clickhousePodNames, err := r.fetchPodNameByLabel(ctx, clickhouseLabel, r.namespace)
	if err != nil {
		return nil, err
	}
	grafanaPodNames, err := r.fetchPodNameByLabel(ctx, grafanaLabel, r.namespace)
	if err != nil {
		return nil, err
	}
	flowAggregatorPodNames, err := r.fetchPodNameByLabel(ctx, flowAggregatorLabel, flowAggregatorNS)
	if err != nil {
		return nil, err
	}
	sparkDriverPodNames, err := r.fetchPodNameByLabel(ctx, sparkDriverLabel, r.namespace)
	if err != nil {
		return nil, err
	}
	sparkExecPodNames, err := r.fetchPodNameByLabel(ctx, sparkExecLabel, r.namespace)
	if err != nil {
		return nil, err
	}
	sparkOperatorPodNames, err := r.fetchPodNameByLabel(ctx, sparkOperatorLabel, r.namespace)
	if err != nil {
		return nil, err
	}
	zookeeperPodNames, err := r.fetchPodNameByLabel(ctx, zookeeperLabel, r.namespace)
	if err != nil {
		return nil, err
	}

	dumper := newManagerDumper(defaultFS, r.restConfig, r.clientSet, since, r.namespace,
		clickhousePodNames, grafanaPodNames, flowAggregatorPodNames, sparkDriverPodNames,
		sparkExecPodNames, sparkOperatorPodNames, zookeeperPodNames)
	return r.collect(
		ctx,
		dumper.DumpLog,
		dumper.DumpClickHouseServerLog,
		dumper.DumpFlowAggregatorLog,
		dumper.DumpGrafanaLog,
		dumper.DumpSparkDriverLog,
		dumper.DumpSparkExecutorLog,
		dumper.DumpSparkOperatorLog,
		dumper.DumpZookeeperLog,
	)
}

func (r *supportBundleREST) fetchPodNameByLabel(ctx context.Context, label, namespace string) ([]string, error) {
	pods, err := r.clientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list Pods with label %s: %v", label, err)
	}
	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil {
			continue
		}
		podNames = append(podNames, pod.Name)
	}
	return podNames, nil
}

func (r *supportBundleREST) clean(ctx context.Context, bundlePath string, duration time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(duration):
		func() {
			r.statusLocker.Lock()
			defer r.statusLocker.Unlock()
			select { // check the context again in case of cancellation when acquiring the lock.
			case <-ctx.Done():
			default:
				if r.cache != nil && r.cache.Status == systemv1alpha1.SupportBundleStatusCollected {
					r.cache = &systemv1alpha1.SupportBundle{
						ObjectMeta: metav1.ObjectMeta{Name: bundleName},
						Status:     systemv1alpha1.SupportBundleStatusNone,
					}
				}
			}
		}()
	}
	defaultFS.Remove(bundlePath)
}

func packDir(dir string, writer io.Writer) ([]byte, error) {
	hash := sha256.New()
	gzWriter := gzip.NewWriter(io.MultiWriter(hash, writer))
	targzWriter := tar.NewWriter(gzWriter)
	err := afero.Walk(defaultFS, dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.IsDir() {
			return nil
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = strings.TrimPrefix(strings.ReplaceAll(filePath, dir, ""), string(filepath.Separator))
		err = targzWriter.WriteHeader(header)
		if err != nil {
			return err
		}
		f, err := defaultFS.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(targzWriter, f)
		return err
	})
	if err != nil {
		return nil, err
	}
	targzWriter.Close()
	gzWriter.Close()
	return hash.Sum(nil), nil
}

var (
	_ rest.Storage         = new(downloadREST)
	_ rest.Getter          = new(downloadREST)
	_ rest.StorageMetadata = new(downloadREST)
)

// downloadREST implements the REST for downloading the bundle.
type downloadREST struct {
	supportBundle *supportBundleREST
}

func (d *downloadREST) New() runtime.Object {
	return &systemv1alpha1.SupportBundle{}
}

func (d *downloadREST) Get(_ context.Context, _ string, _ *metav1.GetOptions) (runtime.Object, error) {
	return &bundleStream{d.supportBundle.cache}, nil
}

func (d *downloadREST) ProducesMIMETypes(_ string) []string {
	return []string{"application/tar+gz"}
}

func (d *downloadREST) ProducesObject(_ string) interface{} {
	return ""
}

var (
	_ rest.ResourceStreamer = new(bundleStream)
	_ runtime.Object        = new(bundleStream)
)

type bundleStream struct {
	cache *systemv1alpha1.SupportBundle
}

func (b *bundleStream) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (b *bundleStream) DeepCopyObject() runtime.Object {
	panic("bundleStream does not have DeepCopyObject")
}

func (b *bundleStream) InputStream(_ context.Context, _, _ string) (stream io.ReadCloser, flush bool, mimeType string, err error) {
	// f will be closed by invoker, no need to close in this function.
	f, err := defaultFS.Open(b.cache.Filepath)
	if err != nil {
		return nil, false, "", err
	}
	return f, true, "application/tar+gz", nil
}
