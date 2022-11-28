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

package networkpolicyrecommendation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/klog/v2"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/client/clientset/versioned"
	fakecrd "antrea.io/theia/pkg/client/clientset/versioned/fake"
	crdinformers "antrea.io/theia/pkg/client/informers/externalversions"
	"antrea.io/theia/pkg/util"
	"antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const informerDefaultResync = 30 * time.Second

var (
	testNamespace = "controller-test"
)

type fakeController struct {
	*NPRecommendationController
	crdClient          versioned.Interface
	kubeClient         kubernetes.Interface
	crdInformerFactory crdinformers.SharedInformerFactory
}

func newFakeController() *fakeController {
	kubeClient := fake.NewSimpleClientset()
	createClickHousePod(kubeClient)
	createSparkOperatorPod(kubeClient)
	createClickHouseService(kubeClient)
	createClickHouseSecret(kubeClient)
	crdClient := fakecrd.NewSimpleClientset()

	crdClient.PrependReactor("create", "recommendednetworkpolicies", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		rnp := action.(k8stesting.CreateAction).GetObject().(*crdv1alpha1.RecommendedNetworkPolicy)
		rnp.Name = fmt.Sprintf("%s%s", rnp.GenerateName, rand.String(8))
		return false, rnp, nil
	})

	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, informerDefaultResync)
	npRecommendationInformer := crdInformerFactory.Crd().V1alpha1().NetworkPolicyRecommendations()
	recommendedNPInformer := crdInformerFactory.Crd().V1alpha1().RecommendedNetworkPolicies()

	nprController := NewNPRecommendationController(crdClient, kubeClient, npRecommendationInformer, recommendedNPInformer)

	return &fakeController{
		nprController,
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

func createClickHouseService(kubeClient kubernetes.Interface) {
	clickhouseService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clickhouse-clickhouse",
			Namespace: testNamespace,
		},
		Spec: v1.ServiceSpec{
			Ports:     []v1.ServicePort{{Name: "tcp", Port: 9000, Protocol: v1.ProtocolTCP}},
			ClusterIP: "10.98.208.26",
		},
	}
	kubeClient.CoreV1().Services(testNamespace).Create(context.TODO(), clickhouseService, metav1.CreateOptions{})
}

func createClickHouseSecret(kubeClient kubernetes.Interface) {
	clickhouseSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clickhouse-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"username": []byte("clickhouse_operator"),
			"password": []byte("clickhouse_operator_password"),
		},
	}
	kubeClient.CoreV1().Secrets(testNamespace).Create(context.TODO(), clickhouseSecret, metav1.CreateOptions{})
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

	GetSparkMonitoringSvcDNS = func(id, namespace string) string {
		return testServer.URL
	}
	return nil
}

// mock Spark Applications
type fakeSparkApplicationClient struct {
	sparkApplications map[apimachinerytypes.NamespacedName]*v1beta2.SparkApplication
	mapMutex          sync.Mutex
}

func (f *fakeSparkApplicationClient) create(client kubernetes.Interface, namespace string, recommendationApplication *v1beta2.SparkApplication) error {
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: namespace,
		Name:      recommendationApplication.Name,
	}
	klog.InfoS("Spark Application created", "name", recommendationApplication.ObjectMeta.Name, "namespace", namespace)
	f.mapMutex.Lock()
	defer f.mapMutex.Unlock()
	f.sparkApplications[namespacedName] = recommendationApplication
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

func TestNPRecommendation(t *testing.T) {
	fakeSAClient := fakeSparkApplicationClient{
		sparkApplications: make(map[apimachinerytypes.NamespacedName]*v1beta2.SparkApplication),
	}
	CreateSparkApplication = fakeSAClient.create
	DeleteSparkApplication = fakeSAClient.delete
	ListSparkApplication = fakeSAClient.list
	GetSparkApplication = fakeSAClient.get

	// Use a shorter resync period
	npRecommendationResyncPeriod = 500 * time.Millisecond

	nprController := newFakeController()
	stopCh := make(chan struct{})

	nprController.crdInformerFactory.Start(stopCh)
	nprController.crdInformerFactory.WaitForCacheSync(stopCh)

	go nprController.Run(stopCh)

	t.Run("NormalNetworkPolicyRecommendation", func(t *testing.T) {
		npr := &crdv1alpha1.NetworkPolicyRecommendation{
			ObjectMeta: metav1.ObjectMeta{Name: "pr-364a180e-2d83-4502-8063-0c3db36cbcd3", Namespace: testNamespace},
			Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
				JobType:             "initial",
				PolicyType:          "anp-deny-applied",
				ExecutorInstances:   1,
				DriverCoreRequest:   "200m",
				DriverMemory:        "512M",
				ExecutorCoreRequest: "200m",
				ExecutorMemory:      "512M",
				ExcludeLabels:       true,
				ToServices:          true,
				StartInterval:       metav1.NewTime(time.Now()),
				EndInterval:         metav1.NewTime(time.Now().Add(time.Second * 10)),
				NSAllowList:         []string{"kube-system", "flow-visibility"},
			},
			Status: crdv1alpha1.NetworkPolicyRecommendationStatus{},
		}

		npr, err := nprController.CreateNetworkPolicyRecommendation(testNamespace, npr)
		assert.NoError(t, err)

		serviceCreated := false
		// The step interval should be larger than resync period to ensure the progress is updated
		stepInterval := 1 * time.Second
		timeout := 30 * time.Second

		wait.PollImmediate(stepInterval, timeout, func() (done bool, err error) {
			npr, err = nprController.GetNetworkPolicyRecommendation(testNamespace, "pr-364a180e-2d83-4502-8063-0c3db36cbcd3")
			if err != nil {
				return false, nil
			}
			// Mocking ClickHouse results and Spark Monitor service requires
			// the SparkApplication id.
			if !serviceCreated {
				// Mock ClickHouse database
				util.SqlOpenFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
					db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
					if err != nil {
						return db, err
					}
					mock.ExpectPing()
					recommendationRow := sqlmock.NewRows([]string{"type", "timeCreated", "yamls"}).AddRow("initial", "2022-10-01T12:30:10Z", "recommendations")
					mock.ExpectQuery("SELECT type, timeCreated, yamls FROM recommendations WHERE id = (?);").WithArgs(npr.Status.SparkApplication).WillReturnRows(recommendationRow)
					return db, err
				}
				// Create Spark Monitor service
				err = createFakeSparkApplicationService(nprController.kubeClient, npr.Status.SparkApplication)
				assert.NoError(t, err)
				serviceCreated = true
			}
			if npr != nil {
				fakeSAClient.step("pr-"+npr.Status.SparkApplication, testNamespace)
			}
			return !(npr.Status.RecommendedNP == nil), nil
		})

		assert.Equal(t, crdv1alpha1.NPRecommendationStateCompleted, npr.Status.State)
		assert.Equal(t, 3, npr.Status.CompletedStages)
		assert.Equal(t, 5, npr.Status.TotalStages)
		assert.True(t, npr.Status.StartTime.Before(&npr.Status.EndTime))
		assert.Equal(t, npr.Status.SparkApplication, npr.Status.RecommendedNP.Spec.Id)
		assert.Equal(t, "initial", npr.Status.RecommendedNP.Spec.Type)
		expectedTimeCreated, err := time.Parse(clickHouseTimeFormat, "2022-10-01T12:30:10Z")
		assert.NoError(t, err)
		assert.Equal(t, metav1.NewTime(expectedTimeCreated), npr.Status.RecommendedNP.Spec.TimeCreated)
		assert.Equal(t, "recommendations", npr.Status.RecommendedNP.Spec.Yamls)

		nprList, err := nprController.ListNetworkPolicyRecommendation(testNamespace)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(nprList), "Expected exactly one NetworkPolicyRecommendation, got %d", len(nprList))
		assert.Equal(t, npr, nprList[0])

		util.SqlOpenFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
			if err != nil {
				return db, err
			}
			mock.ExpectPing()
			mock.ExpectExec("ALTER TABLE recommendations_local ON CLUSTER '{cluster}' DELETE WHERE id = (?);").WithArgs(npr.Status.SparkApplication).WillReturnResult(sqlmock.NewResult(0, 1))
			return db, err
		}
		err = nprController.DeleteNetworkPolicyRecommendation(testNamespace, "pr-364a180e-2d83-4502-8063-0c3db36cbcd3")
		assert.NoError(t, err)
	})

	testCases := []struct {
		name             string
		nprName          string
		npr              *crdv1alpha1.NetworkPolicyRecommendation
		expectedErrorMsg string
	}{
		{
			name:    "invalid JobType",
			nprName: "npr-invalid-job-type",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-job-type", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType: "nonexistent-job-type",
				},
			},
			expectedErrorMsg: "invalid request: recommendation type should be 'initial' or 'subsequent'",
		},
		{
			name:    "invalid Limit",
			nprName: "npr-invalid-limit",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-limit", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType: "initial",
					Limit:   -1,
				},
			},
			expectedErrorMsg: "invalid request: limit should be an integer >= 0",
		},
		{
			name:    "invalid PolicyType",
			nprName: "npr-invalid-policy-type",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-policy-type", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:    "initial",
					PolicyType: "nonexistent-policy-type",
				},
			},
			expectedErrorMsg: "invalid request: type of generated NetworkPolicy should be anp-deny-applied or anp-deny-all or k8s-np",
		},
		{
			name:    "invalid EndInterval",
			nprName: "npr-invalid-end-interval",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-end-interval", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:       "initial",
					PolicyType:    "anp-deny-all",
					StartInterval: metav1.NewTime(time.Now().Add(time.Second * 10)),
					EndInterval:   metav1.NewTime(time.Now()),
				},
			},
			expectedErrorMsg: "invalid request: EndInterval should be after StartInterval",
		},
		{
			name:    "invalid ExecutorInstances",
			nprName: "npr-invalid-executor-instances",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-executor-instances", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:           "initial",
					PolicyType:        "k8s-np",
					ExecutorInstances: -1,
				},
			},
			expectedErrorMsg: "invalid request: ExecutorInstances should be an integer >= 0",
		},
		{
			name:    "invalid DriverCoreRequest",
			nprName: "npr-invalid-driver-core-request",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-driver-core-request", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:           "initial",
					PolicyType:        "k8s-np",
					ExecutorInstances: 1,
					DriverCoreRequest: "m200",
				},
			},
			expectedErrorMsg: "invalid request: DriverCoreRequest should conform to the Kubernetes resource quantity convention",
		},
		{
			name:    "invalid DriverMemory",
			nprName: "npr-invalid-driver-memory",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-driver-memory", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:           "initial",
					PolicyType:        "k8s-np",
					ExecutorInstances: 1,
					DriverCoreRequest: "200m",
					DriverMemory:      "m512",
				},
			},
			expectedErrorMsg: "invalid request: DriverMemory should conform to the Kubernetes resource quantity convention",
		},
		{
			name:    "invalid ExecutorCoreRequest",
			nprName: "npr-invalid-executor-core-request",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-executor-core-request", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:             "initial",
					PolicyType:          "k8s-np",
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
			nprName: "npr-invalid-executor-memory",
			npr: &crdv1alpha1.NetworkPolicyRecommendation{
				ObjectMeta: metav1.ObjectMeta{Name: "npr-invalid-executor-memory", Namespace: testNamespace},
				Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
					JobType:             "initial",
					PolicyType:          "k8s-np",
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
			npr, err := nprController.CreateNetworkPolicyRecommendation(testNamespace, tc.npr)
			assert.NoError(t, err)
			stepInterval := 100 * time.Millisecond
			timeout := 30 * time.Second
			wait.PollImmediate(stepInterval, timeout, func() (done bool, err error) {
				npr, err = nprController.GetNetworkPolicyRecommendation(testNamespace, tc.nprName)
				if err != nil {
					return false, nil
				}
				if npr.Status.State == crdv1alpha1.NPRecommendationStateFailed {
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
			err := validateCluster(kubeClient, testNamespace)
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestGetPolicyRecommendationProgress(t *testing.T) {
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
				_, _, err = getPolicyRecommendationProgress(tc.testServer.URL)
			} else {
				_, _, err = getPolicyRecommendationProgress("http://127.0.0.1")
			}
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestGetPolicyRecommendationResult(t *testing.T) {
	sparkAppID := "spark-application-id"
	var db *sql.DB

	testCases := []struct {
		name             string
		setup            func(kubernetes.Interface)
		expectedErrorMsg string
	}{
		{
			name:             "no ClickHouse service",
			setup:            func(client kubernetes.Interface) {},
			expectedErrorMsg: "error when getting the ClickHouse Service address: error when finding the Service clickhouse-clickhouse",
		},
		{
			name: "no ClickHouse secret",
			setup: func(client kubernetes.Interface) {
				createClickHouseService(client)
			},
			expectedErrorMsg: "error when finding the ClickHouse secret",
		},
		{
			name: "connection error for ClickHouse",
			setup: func(client kubernetes.Interface) {
				createClickHouseService(client)
				createClickHouseSecret(client)
				util.SqlOpenFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
					return nil, fmt.Errorf("connection error")
				}
			},
			expectedErrorMsg: "failed to open ClickHouse: connection error",
		},
		{
			name: "ping error for ClickHouse",
			setup: func(client kubernetes.Interface) {
				createClickHouseService(client)
				createClickHouseSecret(client)
				util.SqlOpenFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
					var err error
					db, _, err = sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
					return db, err
				}
			},
			expectedErrorMsg: "error when connecting to ClickHouse, failed to ping ClickHouse",
		},
		{
			name: "no result in ClickHouse",
			setup: func(client kubernetes.Interface) {
				createClickHouseService(client)
				createClickHouseSecret(client)
				util.SqlOpenFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
					var err error
					var mock sqlmock.Sqlmock
					db, mock, err = sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
					if err != nil {
						return db, err
					}
					mock.ExpectPing()
					mock.ExpectQuery("SELECT type, timeCreated, yamls FROM recommendations WHERE id = (?);").WithArgs(sparkAppID).WillReturnError(sql.ErrNoRows)
					return db, err
				}
			},
			expectedErrorMsg: fmt.Sprintf("failed to get recommendation result with id %s", sparkAppID),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			tc.setup(kubeClient)
			if db != nil {
				defer db.Close()
			}
			connect, err := util.SetupClickHouseConnection(kubeClient, testNamespace)
			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			} else {
				_, err := getPolicyRecommendationResult(connect, sparkAppID)
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			}
		})
	}
}
