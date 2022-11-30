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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clientrest "k8s.io/client-go/rest"

	system "antrea.io/theia/pkg/apis/system/v1alpha1"
	"antrea.io/theia/pkg/support"
)

func TestREST(t *testing.T) {
	r := &supportBundleREST{}
	assert.Equal(t, &system.SupportBundle{}, r.New())
	assert.False(t, r.NamespaceScoped())
}

func TestClean(t *testing.T) {
	defaultFS = afero.NewMemMapFs()

	for name, tc := range map[string]struct {
		needCancel bool
		duration   time.Duration
	}{
		"CleanByCancellation": {
			needCancel: true,
			duration:   time.Hour,
		},
		"CleanByTimeout": {
			duration: 10 * time.Millisecond,
		},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := defaultFS.Create("test.tar.gz")
			require.NoError(t, err)
			defer defaultFS.Remove(f.Name())
			require.NoError(t, f.Close())
			storage := NewSupportBundleStorage(&clientrest.Config{}, fake.NewSimpleClientset())
			ctx, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()
			if tc.needCancel {
				cancelFunc()
			}
			go storage.SupportBundle.clean(ctx, f.Name(), tc.duration)
			time.Sleep(200 * time.Millisecond)
			exist, err := afero.Exists(defaultFS, f.Name())
			require.NoError(t, err)
			require.False(t, exist)
			require.Equal(t, system.SupportBundleStatusNone, storage.SupportBundle.cache.Status)
		})
	}
}

func TestCollect(t *testing.T) {
	defaultFS = afero.NewMemMapFs()
	defer func() {
		defaultFS = afero.NewOsFs()
	}()

	storage := NewSupportBundleStorage(&clientrest.Config{}, fake.NewSimpleClientset())
	dumper1Executed := false
	dumper1 := func(string) error {
		dumper1Executed = true
		return nil
	}
	dumper2Executed := false
	dumper2 := func(string) error {
		dumper2Executed = true
		return nil
	}
	collectedBundle, err := storage.SupportBundle.collect(context.TODO(), dumper1, dumper2)
	require.NoError(t, err)
	require.NotEmpty(t, collectedBundle.Filepath)
	defer defaultFS.Remove(collectedBundle.Filepath)
	assert.Equal(t, system.SupportBundleStatusCollected, collectedBundle.Status)
	assert.NotEmpty(t, collectedBundle.Sum)
	assert.Greater(t, collectedBundle.Size, uint32(0))
	assert.True(t, dumper1Executed)
	assert.True(t, dumper2Executed)
	exist, err := afero.Exists(defaultFS, collectedBundle.Filepath)
	require.NoError(t, err)
	require.True(t, exist)
}

type fakeManagerDumper struct {
	returnErr error
}

func (f *fakeManagerDumper) DumpLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpClickHouseServerLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpFlowAggregatorLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpGrafanaLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpSparkDriverLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpSparkExecutorLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpSparkOperatorLog(basedir string) error {
	return f.returnErr
}

func (f *fakeManagerDumper) DumpZookeeperLog(basedir string) error {
	return f.returnErr
}

func TestSupportBundleStorage(t *testing.T) {
	defaultFS = afero.NewMemMapFs()
	newManagerDumper = func(fs afero.Fs, restConfig *clientrest.Config, clientSet kubernetes.Interface, since string,
		namespace string, clickhousePodNames, grafanaPodNames, flowAggregatorPodNames, sparkDriverPodNames,
		sparkExecutorPodNames, sparkOperatorPodNames, zookeeperPodNames []string) support.ManagerDumper {
		return &fakeManagerDumper{}
	}
	defer func() {
		defaultFS = afero.NewOsFs()
		newManagerDumper = support.NewManagerDumper
	}()

	ctx := context.Background()
	storage := NewSupportBundleStorage(&clientrest.Config{}, fake.NewSimpleClientset())
	_, err := storage.SupportBundle.Create(ctx, &system.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: bundleName,
		},
		Status: system.SupportBundleStatusNone,
		Since:  "-1h",
	}, nil, nil)
	require.NoError(t, err)

	var collectedBundle *system.SupportBundle
	assert.Eventually(t, func() bool {
		object, err := storage.SupportBundle.Get(ctx, bundleName, nil)
		require.NoError(t, err)
		collectedBundle = object.(*system.SupportBundle)
		return collectedBundle.Status == system.SupportBundleStatusCollected
	}, time.Second*2, time.Millisecond*100)
	require.NotEmpty(t, collectedBundle.Filepath)
	filePath := collectedBundle.Filepath
	defer defaultFS.Remove(filePath)
	assert.NotEmpty(t, collectedBundle.Sum)
	assert.Greater(t, collectedBundle.Size, uint32(0))
	exist, err := afero.Exists(defaultFS, filePath)
	require.NoError(t, err)
	require.True(t, exist)

	_, deleted, err := storage.SupportBundle.Delete(ctx, bundleName, nil, nil)
	assert.NoError(t, err)
	assert.True(t, deleted)
	object, err := storage.SupportBundle.Get(ctx, bundleName, nil)
	assert.NoError(t, err)
	assert.Equal(t, &system.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{Name: bundleName},
		Status:     system.SupportBundleStatusNone,
	}, object)
	assert.Eventuallyf(t, func() bool {
		exist, err := afero.Exists(defaultFS, filePath)
		require.NoError(t, err)
		return !exist
	}, time.Second*2, time.Millisecond*100, "Supportbundle file %s was not deleted after deleting the Supportbundle object", filePath)
}

func TestSupportBundleStorageFailure(t *testing.T) {
	defaultFS = afero.NewMemMapFs()
	newManagerDumper = func(fs afero.Fs, restConfig *clientrest.Config, clientSet kubernetes.Interface, since string,
		namespace string, clickhousePodNames, grafanaPodNames, flowAggregatorPodNames, sparkDriverPodNames,
		sparkExecutorPodNames, sparkOperatorPodNames, zookeeperPodNames []string) support.ManagerDumper {
		return &fakeManagerDumper{returnErr: fmt.Errorf("error faked by test")}
	}
	defer func() {
		defaultFS = afero.NewOsFs()
		newManagerDumper = support.NewManagerDumper
	}()

	ctx := context.Background()
	storage := NewSupportBundleStorage(&clientrest.Config{}, fake.NewSimpleClientset())
	_, err := storage.SupportBundle.Create(ctx, &system.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name: bundleName,
		},
		Status: system.SupportBundleStatusNone,
		Since:  "-1h",
	}, nil, nil)
	require.NoError(t, err)

	var collectedBundle *system.SupportBundle
	assert.Eventually(t, func() bool {
		object, err := storage.SupportBundle.Get(ctx, bundleName, nil)
		require.NoError(t, err)
		collectedBundle = object.(*system.SupportBundle)
		return collectedBundle.Status == system.SupportBundleStatusNone
	}, time.Second*2, time.Millisecond*100)
	assert.Empty(t, collectedBundle.Filepath)
	assert.Empty(t, collectedBundle.Sum)
	assert.Empty(t, collectedBundle.Size)
}
