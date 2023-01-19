// Copyright 2023 Antrea Authors
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

package anomalydetector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog/v2"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/client/clientset/versioned"
	fakecrd "antrea.io/theia/pkg/client/clientset/versioned/fake"
	crdinformers "antrea.io/theia/pkg/client/informers/externalversions"
	controllerUtil "antrea.io/theia/pkg/controller"
	"antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const informerDefaultResync = 30 * time.Second

var (
	testNamespace = "controller-test"
)

type fakeController struct {
	*AnomalyDetectorController
	crdClient          versioned.Interface
	kubeClient         kubernetes.Interface
	crdInformerFactory crdinformers.SharedInformerFactory
}

func newFakeController() *fakeController {
	kubeClient := fake.NewSimpleClientset()
	createClickHousePod(kubeClient)
	createSparkOperatorPod(kubeClient)
	crdClient := fakecrd.NewSimpleClientset()

	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, informerDefaultResync)
	taDetectorInformer := crdInformerFactory.Crd().V1alpha1().ThroughputAnomalyDetectors()

	tadController := NewAnomalyDetectorController(crdClient, kubeClient, taDetectorInformer)

	return &fakeController{
		tadController,
		crdClient,
		kubeClient,
		crdInformerFactory,
	}
}

func createClickHousePod(kubeClient kubernetes.Interface) {
	clickHousePod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clickhouse",
			Namespace: testNamespace,
			Labels:    map[string]string{"app": "clickhouse"},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
	kubeClient.CoreV1().Pods(testNamespace).Create(context.TODO(), clickHousePod, metav1.CreateOptions{})
}

func createSparkOperatorPod(kubeClient kubernetes.Interface) {
	sparkOperatorPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "spark-operator",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "spark-operator",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
	kubeClient.CoreV1().Pods(testNamespace).Create(context.TODO(), sparkOperatorPod, metav1.CreateOptions{})
}

func createFakeSparkApplicationService(kubeClient kubernetes.Interface, id string) error {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/api/v1/applications":
			responses := []map[string]interface{}{
				{"id": id},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(responses)
		case fmt.Sprintf("/api/v1/applications/%s/stages", id):
			responses := []map[string]interface{}{
				{"status": "COMPLETE"},
				{"status": "COMPLETE"},
				{"status": "SKIPPED"},
				{"status": "PENDING"},
				{"status": "ACTIVE"},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(responses)
		}
	}))

	GetSparkMonitoringSvcDNS = func(id, namespace string, sparkPort int) string {
		return testServer.URL
	}
	return nil
}

// mock Spark Applications
type fakeSparkApplicationClient struct {
	sparkApplications map[apimachinerytypes.NamespacedName]*v1beta2.SparkApplication
	mapMutex          sync.Mutex
}

func (f *fakeSparkApplicationClient) create(client kubernetes.Interface, namespace string, tadetetectorApplication *v1beta2.SparkApplication) error {
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: namespace,
		Name:      tadetetectorApplication.Name,
	}
	klog.InfoS("Spark Application created", "name", tadetetectorApplication.ObjectMeta.Name, "namespace", namespace)
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	f.sparkApplications[namespacedName] = tadetetectorApplication
	return nil
}

func (f *fakeSparkApplicationClient) delete(client kubernetes.Interface, name, namespace string) {
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	delete(f.sparkApplications, namespacedName)
}

func (f *fakeSparkApplicationClient) list(client kubernetes.Interface, namespace string) (*v1beta2.SparkApplicationList, error) {
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	list := make([]v1beta2.SparkApplication, len(f.sparkApplications))
	index := 0
	for _, item := range f.sparkApplications {
		list[index] = *item
		index++
	}
	saList := &v1beta2.SparkApplicationList{
		Items: list,
	}
	return saList, nil
}

func (f *fakeSparkApplicationClient) get(client kubernetes.Interface, name, namespace string) (sparkApp v1beta2.SparkApplication, err error) {
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	return *f.sparkApplications[namespacedName], nil
}

func (f *fakeSparkApplicationClient) step(name, namespace string) {
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	sa, ok := f.sparkApplications[namespacedName]
	if !ok {
		klog.InfoS("Spark Application not created yet", "name", name, "namespace", namespace)
		return
	}
	switch sa.Status.AppState.State {
	case v1beta2.NewState:
		klog.InfoS("Spark Application setting from new to running")
		sa.Status.AppState.State = v1beta2.RunningState
	case v1beta2.RunningState:
		klog.InfoS("Spark Application setting from running to completed")
		sa.Status.AppState.State = v1beta2.CompletedState
	}
}

func TestTADetection(t *testing.T) {
	fakeSAClient := fakeSparkApplicationClient{
		sparkApplications: make(map[apimachinerytypes.NamespacedName]*v1beta2.SparkApplication),
	}
	CreateSparkApplication = fakeSAClient.create
	DeleteSparkApplication = fakeSAClient.delete
	ListSparkApplication = fakeSAClient.list
	GetSparkApplication = fakeSAClient.get

	// Use a shorter resync period
	anomalyDetectorResyncPeriod = 100 * time.Millisecond

	tadController := newFakeController()
	stopCh := make(chan struct{})

	tadController.crdInformerFactory.Start(stopCh)
	tadController.crdInformerFactory.WaitForCacheSync(stopCh)

	go tadController.Run(stopCh)

	t.Run("NormalAnomalyDetector", func(t *testing.T) {
		tad := &crdv1alpha1.ThroughputAnomalyDetector{
			ObjectMeta: metav1.ObjectMeta{Name: "tad-1234abcd-1234-abcd-12ab-12345678abcd", Namespace: testNamespace},
			Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
				JobType:             "ARIMA",
				ExecutorInstances:   1,
				DriverCoreRequest:   "200m",
				DriverMemory:        "512M",
				ExecutorCoreRequest: "200m",
				ExecutorMemory:      "512M",
				StartInterval:       metav1.NewTime(time.Now()),
				EndInterval:         metav1.NewTime(time.Now().Add(time.Second * 100)),
			},
			Status: crdv1alpha1.ThroughputAnomalyDetectorStatus{},
		}

		tad, err := tadController.CreateThroughputAnomalyDetector(testNamespace, tad)
		assert.NoError(t, err)

		serviceCreated := false
		// The step interval should be larger than resync period to ensure the progress is updated
		stepInterval := 1 * time.Second
		timeout := 30 * time.Second

		wait.PollImmediate(stepInterval, timeout, func() (done bool, err error) {
			tad, err = tadController.GetThroughputAnomalyDetector(testNamespace, "tad-1234abcd-1234-abcd-12ab-12345678abcd")
			if err != nil {
				return false, nil
			}
			// Mocking Spark Monitor service requires the SparkApplication id.
			if !serviceCreated {
				// Create Spark Monitor service
				err = createFakeSparkApplicationService(tadController.kubeClient, tad.Status.SparkApplication)
				assert.NoError(t, err)
				serviceCreated = true
			}
			if tad != nil {
				fakeSAClient.step("tad-"+tad.Status.SparkApplication, testNamespace)
			}
			return tad.Status.State == crdv1alpha1.ThroughputAnomalyDetectorStateCompleted, nil
		})

		assert.Equal(t, crdv1alpha1.ThroughputAnomalyDetectorStateCompleted, tad.Status.State)
		assert.Equal(t, 3, tad.Status.CompletedStages)
		assert.Equal(t, 5, tad.Status.TotalStages)
		assert.True(t, tad.Status.StartTime.Before(&tad.Status.EndTime))

		tadList, err := tadController.ListThroughputAnomalyDetector(testNamespace)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(tadList), "Expected exactly one ThroughputAnomalyDetector, got %d", len(tadList))
		assert.Equal(t, tad, tadList[0])

		err = tadController.DeleteThroughputAnomalyDetector(testNamespace, "tad-1234abcd-1234-abcd-12ab-12345678abcd")
		assert.NoError(t, err)
	})

	testCases := []struct {
		name             string
		tadName          string
		tad              *crdv1alpha1.ThroughputAnomalyDetector
		expectedErrorMsg string
	}{
		{
			name:    "invalid JobType",
			tadName: "tad-invalid-job-type",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-job-type", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType: "nonexistent-job-type",
				},
			},
			expectedErrorMsg: "invalid request: Throughput Anomaly DetectorQuerier type should be 'EWMA' or 'ARIMA' or 'DBSCAN'",
		},
		{
			name:    "invalid EndInterval",
			tadName: "tad-invalid-end-interval",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-end-interval", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:       "ARIMA",
					StartInterval: metav1.NewTime(time.Now().Add(time.Second * 10)),
					EndInterval:   metav1.NewTime(time.Now()),
				},
			},
			expectedErrorMsg: "invalid request: EndInterval should be after StartInterval",
		},
		{
			name:    "invalid ExecutorInstances",
			tadName: "tad-invalid-executor-instances",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-executor-instances", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:           "ARIMA",
					ExecutorInstances: -1,
				},
			},
			expectedErrorMsg: "invalid request: ExecutorInstances should be an integer >= 0",
		},
		{
			name:    "invalid DriverCoreRequest",
			tadName: "tad-invalid-driver-core-request",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-driver-core-request", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:           "ARIMA",
					ExecutorInstances: 1,
					DriverCoreRequest: "m200",
				},
			},
			expectedErrorMsg: "invalid request: DriverCoreRequest should conform to the Kubernetes resource quantity convention",
		},
		{
			name:    "invalid DriverMemory",
			tadName: "tad-invalid-driver-memory",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-driver-memory", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:           "ARIMA",
					ExecutorInstances: 1,
					DriverCoreRequest: "200m",
					DriverMemory:      "m512",
				},
			},
			expectedErrorMsg: "invalid request: DriverMemory should conform to the Kubernetes resource quantity convention",
		},
		{
			name:    "invalid ExecutorCoreRequest",
			tadName: "tad-invalid-executor-core-request",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-executor-core-request", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:             "ARIMA",
					ExecutorInstances:   1,
					DriverCoreRequest:   "200m",
					DriverMemory:        "512M",
					ExecutorCoreRequest: "m200",
				},
			},
			expectedErrorMsg: "invalid request: ExecutorCoreRequest should conform to the Kubernetes resource quantity convention",
		},
		{
			name:    "invalid ExecutorMemory",
			tadName: "tad-invalid-executor-memory",
			tad: &crdv1alpha1.ThroughputAnomalyDetector{
				ObjectMeta: metav1.ObjectMeta{Name: "tad-invalid-executor-memory", Namespace: testNamespace},
				Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
					JobType:             "ARIMA",
					ExecutorInstances:   1,
					DriverCoreRequest:   "200m",
					DriverMemory:        "512M",
					ExecutorCoreRequest: "200m",
					ExecutorMemory:      "m512",
				},
			},
			expectedErrorMsg: "invalid request: ExecutorMemory should conform to the Kubernetes resource quantity convention",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			npr, err := tadController.CreateThroughputAnomalyDetector(testNamespace, tc.tad)
			assert.NoError(t, err)
			stepInterval := 100 * time.Millisecond
			timeout := 30 * time.Second
			wait.PollImmediate(stepInterval, timeout, func() (done bool, err error) {
				npr, err = tadController.GetThroughputAnomalyDetector(testNamespace, tc.tadName)
				if err != nil {
					return false, nil
				}
				if npr.Status.State == crdv1alpha1.ThroughputAnomalyDetectorStateFailed {
					assert.Contains(t, npr.Status.ErrorMsg, tc.expectedErrorMsg)
					return true, nil
				}
				return false, nil
			})
		})
	}
}

func TestValidateCluster(t *testing.T) {
	testCases := []struct {
		name             string
		setupClient      func(kubernetes.Interface)
		expectedErrorMsg string
	}{
		{
			name:             "clickhouse pod not found",
			setupClient:      func(i kubernetes.Interface) {},
			expectedErrorMsg: "failed to find the ClickHouse Pod, please check the deployment",
		},
		{
			name: "spark operator pod not found",
			setupClient: func(client kubernetes.Interface) {
				createClickHousePod(client)
			},
			expectedErrorMsg: "failed to find the Spark Operator Pod, please check the deployment",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			tc.setupClient(kubeClient)
			err := controllerUtil.ValidateCluster(kubeClient, testNamespace)
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestGetTADetectorProgress(t *testing.T) {
	sparkAppID := "spark-application-id"
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedErrorMsg string
	}{
		{
			name: "more than one spark application",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedErrorMsg: "wrong Spark Application number, expected 1, got 2",
		},
		{
			name:             "no spark monitor service",
			testServer:       nil,
			expectedErrorMsg: "failed to get response from the Spark Monitoring Service",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.testServer != nil {
				defer tc.testServer.Close()
				_, _, err = controllerUtil.GetSparkAppProgress(tc.testServer.URL)
			} else {
				_, _, err = controllerUtil.GetSparkAppProgress("http://127.0.0.1")
			}
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}
