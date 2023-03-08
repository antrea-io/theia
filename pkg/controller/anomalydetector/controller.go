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
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/client/clientset/versioned"
	crdv1a1informers "antrea.io/theia/pkg/client/informers/externalversions/crd/v1alpha1"
	"antrea.io/theia/pkg/client/listers/crd/v1alpha1"
	controllerutil "antrea.io/theia/pkg/controller"
	"antrea.io/theia/pkg/util"
	"antrea.io/theia/pkg/util/clickhouse"
	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const (
	controllerName = "AnomalyDetectorController"
	// Spark related parameters
	sparkAppFile = "local:///opt/spark/work-dir/anomaly_detection.py"
)

var (
	// Spark Application CRUD functions, for unit tests
	CreateSparkApplication   = controllerutil.CreateSparkApplication
	DeleteSparkApplication   = controllerutil.DeleteSparkApplication
	GetSparkApplication      = controllerutil.GetSparkApplication
	ListSparkApplication     = controllerutil.ListSparkApplication
	GetSparkMonitoringSvcDNS = controllerutil.GetSparkMonitoringSvcDNS
	// For TAD in scheduled or running state, check its status periodically
	anomalyDetectorResyncPeriod = 10 * time.Second
)

type AnomalyDetectorController struct {
	crdClient  versioned.Interface
	kubeClient kubernetes.Interface

	anomalyDetectorInformer cache.SharedIndexInformer
	anomalyDetectorLister   v1alpha1.ThroughputAnomalyDetectorLister
	anomalyDetectorSynced   cache.InformerSynced
	// queue maintains the Service objects that need to be synced.
	queue                  workqueue.RateLimitingInterface
	deletionQueue          workqueue.RateLimitingInterface
	periodicResyncSetMutex sync.Mutex
	periodicResyncSet      map[apimachinerytypes.NamespacedName]struct{}
	clickhouseConnect      *sql.DB
}

type NamespacedId struct {
	Namespace string
	Id        string
}

func NewAnomalyDetectorController(
	crdClient versioned.Interface,
	kubeClient kubernetes.Interface,
	taDetectorInformer crdv1a1informers.ThroughputAnomalyDetectorInformer,
) *AnomalyDetectorController {
	c := &AnomalyDetectorController{
		crdClient:               crdClient,
		kubeClient:              kubeClient,
		queue:                   workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(controllerutil.MinRetryDelay, controllerutil.MaxRetryDelay), "taDetector"),
		deletionQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(controllerutil.MinRetryDelay, controllerutil.MaxRetryDelay), "taDetectorCleanup"),
		anomalyDetectorInformer: taDetectorInformer.Informer(),
		anomalyDetectorLister:   taDetectorInformer.Lister(),
		anomalyDetectorSynced:   taDetectorInformer.Informer().HasSynced,
		periodicResyncSet:       make(map[apimachinerytypes.NamespacedName]struct{}),
	}

	c.anomalyDetectorInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addTADetector,
			UpdateFunc: c.updateTADetector,
			DeleteFunc: c.deleteTADetector,
		},
		controllerutil.ResyncPeriod,
	)

	return c
}

func (c *AnomalyDetectorController) addTADetector(obj interface{}) {
	newTAD, ok := obj.(*crdv1alpha1.ThroughputAnomalyDetector)
	if !ok {
		klog.ErrorS(nil, "fail to convert to Throughput Anomaly Detector", "object", obj)
		return
	}
	klog.V(2).InfoS("Processing Throughput Anomaly Detector ADD event", "name", newTAD.Name, "labels", newTAD.Labels)
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: newTAD.Namespace,
		Name:      newTAD.Name,
	}
	c.queue.Add(namespacedName)
}

func (c *AnomalyDetectorController) updateTADetector(_, new interface{}) {
	newTAD, ok := new.(*crdv1alpha1.ThroughputAnomalyDetector)
	if !ok {
		klog.ErrorS(nil, "fail to convert to Throughput Anomaly Detector", "object", new)
		return
	}
	klog.V(2).InfoS("Processing Throughput Anomaly Detector UPDATE event", "name", newTAD.Name, "labels", newTAD.Labels)
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: newTAD.Namespace,
		Name:      newTAD.Name,
	}
	c.queue.Add(namespacedName)
}

func (c *AnomalyDetectorController) deleteTADetector(old interface{}) {
	newTAD, ok := old.(*crdv1alpha1.ThroughputAnomalyDetector)
	if !ok {
		tombstone, ok := old.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.ErrorS(nil, "Error decoding object when deleting Throughput Anomaly Detector", "oldObject", old)
			return
		}
		newTAD, ok = tombstone.Obj.(*crdv1alpha1.ThroughputAnomalyDetector)
		if !ok {
			klog.ErrorS(nil, "Error decoding object tombstone when deleting Throughput Anomaly Detector", "tombstone", tombstone.Obj)
			return
		}
	}
	klog.V(2).InfoS("Processing Throughput Anomaly Detector DELETE event", "name", newTAD.Name, "labels", newTAD.Labels)
	// remove Throughput Anomaly Detector from periodic synchronization list in case it is deleted before completing
	c.stopPeriodicSync(apimachinerytypes.NamespacedName{
		Namespace: newTAD.Namespace,
		Name:      newTAD.Name,
	})
	// Add SparkApplication and Namespace information to deletionQueue for cleanup
	if newTAD.Status.SparkApplication != "" {
		namespacedId := NamespacedId{
			Namespace: newTAD.Namespace,
			Id:        newTAD.Status.SparkApplication,
		}
		c.deletionQueue.Add(namespacedId)
	}
}

// Run will create defaultWorkers workers (go routines) which will process the Service events from the
// workqueue.
func (c *AnomalyDetectorController) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	klog.InfoS("Starting controller", "name", controllerName)
	defer klog.InfoS("Shutting down controller", "name", controllerName)

	if !cache.WaitForNamedCacheSync(controllerName, stopCh, c.anomalyDetectorSynced) {
		return
	}

	go func() {
		wait.Until(c.resyncTADetector, anomalyDetectorResyncPeriod, stopCh)
	}()

	go wait.Until(c.deletionworker, time.Second, stopCh)

	for i := 0; i < controllerutil.DefaultWorkers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *AnomalyDetectorController) deletionworker() {
	for c.processNextDeletionWorkItem() {
	}
}

func (c *AnomalyDetectorController) processNextDeletionWorkItem() bool {
	obj, quit := c.deletionQueue.Get()
	if quit {
		return false
	}
	defer c.deletionQueue.Done(obj)
	if key, ok := obj.(NamespacedId); !ok {
		c.queue.Forget(obj)
		klog.ErrorS(nil, "Expected Spark Application namespaced id in work queue", "got", obj)
		return true
	} else if err := c.cleanupTADetector(key.Namespace, key.Id); err == nil {
		// If no error occurs we forget this item so it does not get queued again until
		// another change happens.
		c.deletionQueue.Forget(key)
	} else {
		// Put the item back on the workqueue to handle any transient errors.
		c.deletionQueue.AddRateLimited(key)
		klog.ErrorS(err, "Error when cleaning Spark Application, requeuing", "key", key)
	}
	return true
}

// worker is a long-running function that will continually call the processNextWorkItem function in
// order to read and process a message on the workqueue.
func (c *AnomalyDetectorController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *AnomalyDetectorController) resyncTADetector() {
	c.periodicResyncSetMutex.Lock()
	tads := make([]apimachinerytypes.NamespacedName, 0, len(c.periodicResyncSet))
	for tadNamespacedName := range c.periodicResyncSet {
		tads = append(tads, tadNamespacedName)
	}
	c.periodicResyncSetMutex.Unlock()
	for _, tadNamespacedName := range tads {
		c.queue.Add(tadNamespacedName)
	}
}

func (c *AnomalyDetectorController) processNextWorkItem() bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(obj)
	if key, ok := obj.(apimachinerytypes.NamespacedName); !ok {
		c.queue.Forget(obj)
		klog.ErrorS(nil, "Expected Throughput Anomaly Detector in work queue", "got", obj)
		return true
	} else if err := c.syncTADetector(key); err == nil {
		// If no error occurs we forget this item so it does not get queued again until
		// another change happens.
		c.queue.Forget(key)
	} else {
		// Put the item back on the workqueue to handle any transient errors.
		c.queue.AddRateLimited(key)
		klog.ErrorS(err, "Error when syncing Throughput Anomaly Detector, requeuing", "key", key)
	}
	return true
}

func (c *AnomalyDetectorController) syncTADetector(key apimachinerytypes.NamespacedName) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).InfoS("Finished syncing Throughput Anomaly Detector", "key", key, "time", time.Since(startTime))
	}()

	newTAD, err := c.anomalyDetectorLister.ThroughputAnomalyDetectors(key.Namespace).Get(key.Name)
	if err != nil {
		// Throughput Anomaly Detector already deleted
		if apimachineryerrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	klog.V(4).Infof("Syncing Throughput Anomaly Detector", "newTAD", newTAD)

	switch newTAD.Status.State {
	case "", crdv1alpha1.ThroughputAnomalyDetectorStateNew:
		err = c.startJob(newTAD)
	case crdv1alpha1.ThroughputAnomalyDetectorStateScheduled:
		_, err = c.checkSparkApplicationStatus(newTAD)
	case crdv1alpha1.ThroughputAnomalyDetectorStateRunning:
		err = c.updateProgress(newTAD)
	case crdv1alpha1.ThroughputAnomalyDetectorStateCompleted:
		if newTAD.Status.EndTime.IsZero() {
			err = c.finishJob(newTAD)
		}
	}
	return err
}

func (c *AnomalyDetectorController) cleanupTADetector(namespace string, sparkApplicationId string) error {
	// Delete the Spark Application if exists
	DeleteSparkApplication(c.kubeClient, "tad-"+sparkApplicationId, namespace)
	// Delete the result from the ClickHouse
	if c.clickhouseConnect == nil {
		var err error
		c.clickhouseConnect, err = clickhouse.SetupConnection(c.kubeClient)
		if err != nil {
			return err
		}
	}
	query := "ALTER TABLE tadetector_local ON CLUSTER '{cluster}' DELETE WHERE id = (" + sparkApplicationId + ");"
	return controllerutil.DeleteSparkResult(c.clickhouseConnect, query, sparkApplicationId)
}

func (c *AnomalyDetectorController) finishJob(newTAD *crdv1alpha1.ThroughputAnomalyDetector) error {
	namespacedName := apimachinerytypes.NamespacedName{
		Name:      newTAD.Name,
		Namespace: newTAD.Namespace,
	}
	// Stop periodical job
	c.stopPeriodicSync(namespacedName)
	if newTAD.Status.SparkApplication == "" {
		return c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateFailed,
				ErrorMsg: "Spark Application should be started before updating results",
			},
		)
	}
	// Delete related SparkApplication CR
	DeleteSparkApplication(c.kubeClient, "tad-"+newTAD.Status.SparkApplication, newTAD.Namespace)
	return c.updateTADetectorStatus(
		newTAD,
		crdv1alpha1.ThroughputAnomalyDetectorStatus{
			State:   crdv1alpha1.ThroughputAnomalyDetectorStateCompleted,
			EndTime: metav1.NewTime(time.Now()),
		})
}

func (c *AnomalyDetectorController) updateProgress(newTAD *crdv1alpha1.ThroughputAnomalyDetector) error {
	// Check the status before checking the progress in case the job is failed or completed
	state, err := c.checkSparkApplicationStatus(newTAD)
	if err != nil {
		return err
	}
	if state != crdv1alpha1.ThroughputAnomalyDetectorStateRunning {
		return nil
	}
	endpoint := GetSparkMonitoringSvcDNS(newTAD.Status.SparkApplication, newTAD.Namespace, controllerutil.SparkPort)
	completedStages, totalStages, err := controllerutil.GetSparkAppProgress(endpoint)
	if err != nil {
		// The Spark Monitoring Service may not start or closed at this point due to the async
		// between Spark operator and this controller.
		// As we periodically check the progress, we do not need to requeue this failure.
		klog.V(4).ErrorS(err, "Failed to get the progress of the Throughput Anomaly Detector job")
		return nil
	}
	klog.V(4).InfoS("Got Spark Application progress", "completedStages", completedStages, "totalStages", totalStages, "ThroughputAnomalyDetector", newTAD.Name)
	return c.updateTADetectorStatus(
		newTAD,
		crdv1alpha1.ThroughputAnomalyDetectorStatus{
			State:           crdv1alpha1.ThroughputAnomalyDetectorStateRunning,
			CompletedStages: completedStages,
			TotalStages:     totalStages,
		},
	)
}

func (c *AnomalyDetectorController) checkSparkApplicationStatus(newTAD *crdv1alpha1.ThroughputAnomalyDetector) (string, error) {
	if newTAD.Status.SparkApplication == "" {
		return "", c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateFailed,
				ErrorMsg: "Spark Application should be started before status checking",
			},
		)
	}

	state, errorMessage, err := getTADetectorStatus(c.kubeClient, newTAD.Status.SparkApplication, newTAD.Namespace)
	if err != nil {
		return state, err
	}
	klog.V(4).InfoS("Got Spark Application state", "state", state, "ThroughputAnomalyDetector", newTAD.Name)
	if state == "RUNNING" {
		return state, c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateRunning,
				ErrorMsg: errorMessage,
			},
		)
	} else if state == "COMPLETED" {
		return state, c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateCompleted,
				ErrorMsg: errorMessage,
			},
		)
	} else if state == "FAILED" || state == "SUBMISSION_FAILED" || state == "FAILING" || state == "INVALIDATING" {
		return state, c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateFailed,
				ErrorMsg: fmt.Sprintf("Throughput Anomaly Detector job failed, state: %s, error message: %v", state, errorMessage),
			},
		)
	}
	return state, nil
}

func (c *AnomalyDetectorController) startJob(newTAD *crdv1alpha1.ThroughputAnomalyDetector) error {
	// Validate Cluster readiness
	if err := controllerutil.ValidateCluster(c.kubeClient, newTAD.Namespace); err != nil {
		return err
	}
	err := c.startSparkApplication(newTAD)
	// Mark the ThroughputAnomalyDetector as failed and not retry if it failed due to illegal arguments in request
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(illeagelArguementError{}) {
		return c.updateTADetectorStatus(
			newTAD,
			crdv1alpha1.ThroughputAnomalyDetectorStatus{
				State:    crdv1alpha1.ThroughputAnomalyDetectorStateFailed,
				ErrorMsg: fmt.Sprintf("error in creating AnomalyDetector: %v", err),
			},
		)
	}
	// Schedule periodical resync for successful starting
	if err == nil {
		c.addPeriodicSync(apimachinerytypes.NamespacedName{
			Name:      newTAD.Name,
			Namespace: newTAD.Namespace,
		})
	}
	return err
}

func (c *AnomalyDetectorController) startSparkApplication(newTAD *crdv1alpha1.ThroughputAnomalyDetector) error {
	var newTADJobArgs []string
	if newTAD.Spec.JobType != "EWMA" && newTAD.Spec.JobType != "ARIMA" && newTAD.Spec.JobType != "DBSCAN" {
		return illeagelArguementError{fmt.Errorf("invalid request: Throughput Anomaly DetectorQuerier type should be 'EWMA' or 'ARIMA' or 'DBSCAN'")}
	}
	newTADJobArgs = append(newTADJobArgs, "--algo", newTAD.Spec.JobType)

	if !newTAD.Spec.StartInterval.IsZero() {
		newTADJobArgs = append(newTADJobArgs, "--start_time", newTAD.Spec.StartInterval.Format(controllerutil.InputTimeFormat))
	}
	if !newTAD.Spec.EndInterval.IsZero() {
		endAfterStart := newTAD.Spec.EndInterval.After(newTAD.Spec.StartInterval.Time)
		if !endAfterStart {
			return illeagelArguementError{fmt.Errorf("invalid request: EndInterval should be after StartInterval")}
		}
		newTADJobArgs = append(newTADJobArgs, "--end_time", newTAD.Spec.EndInterval.Format(controllerutil.InputTimeFormat))
	}

	sparkResourceArgs := struct {
		executorInstances   int32
		driverCoreRequest   string
		driverMemory        string
		executorCoreRequest string
		executorMemory      string
	}{}

	if newTAD.Spec.ExecutorInstances < 0 {
		return illeagelArguementError{fmt.Errorf("invalid request: ExecutorInstances should be an integer >= 0")}
	}
	sparkResourceArgs.executorInstances = int32(newTAD.Spec.ExecutorInstances)

	matchResult, err := regexp.MatchString(controllerutil.K8sQuantitiesReg, newTAD.Spec.DriverCoreRequest)
	if err != nil || !matchResult {
		return illeagelArguementError{fmt.Errorf("invalid request: DriverCoreRequest should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.driverCoreRequest = newTAD.Spec.DriverCoreRequest

	matchResult, err = regexp.MatchString(controllerutil.K8sQuantitiesReg, newTAD.Spec.DriverMemory)
	if err != nil || !matchResult {
		return illeagelArguementError{fmt.Errorf("invalid request: DriverMemory should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.driverMemory = newTAD.Spec.DriverMemory

	matchResult, err = regexp.MatchString(controllerutil.K8sQuantitiesReg, newTAD.Spec.ExecutorCoreRequest)
	if err != nil || !matchResult {
		return illeagelArguementError{fmt.Errorf("invalid request: ExecutorCoreRequest should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.executorCoreRequest = newTAD.Spec.ExecutorCoreRequest

	matchResult, err = regexp.MatchString(controllerutil.K8sQuantitiesReg, newTAD.Spec.ExecutorMemory)
	if err != nil || !matchResult {
		return illeagelArguementError{fmt.Errorf("invalid request: ExecutorMemory should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.executorMemory = newTAD.Spec.ExecutorMemory

	err = util.ParseADAlgorithmID(newTAD.Name)
	if err != nil {
		return illeagelArguementError{fmt.Errorf("invalid request: Throughput Anomaly Detector Querier job name is invalid: %s", err)}
	}
	taDetectorID := newTAD.Name[4:]
	newTADJobArgs = append(newTADJobArgs, "--id", taDetectorID)
	taDetectorApplication := &sparkv1.SparkApplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sparkoperator.k8s.io/v1beta2",
			Kind:       "SparkApplication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      newTAD.Name,
			Namespace: newTAD.Namespace,
		},
		Spec: sparkv1.SparkApplicationSpec{
			Type:                "Python",
			SparkVersion:        controllerutil.SparkVersion,
			Mode:                "cluster",
			Image:               controllerutil.ConstStrToPointer(controllerutil.SparkImage),
			ImagePullPolicy:     controllerutil.ConstStrToPointer(controllerutil.SparkImagePullPolicy),
			MainApplicationFile: controllerutil.ConstStrToPointer(sparkAppFile),
			Arguments:           newTADJobArgs,
			Driver: sparkv1.DriverSpec{
				CoreRequest: &newTAD.Spec.DriverCoreRequest,
				SparkPodSpec: sparkv1.SparkPodSpec{
					Memory: &newTAD.Spec.DriverMemory,
					Labels: map[string]string{
						"version": controllerutil.SparkVersion,
					},
					EnvSecretKeyRefs: map[string]sparkv1.NameKey{
						"CH_USERNAME": {
							Name: "clickhouse-secret",
							Key:  "username",
						},
						"CH_PASSWORD": {
							Name: "clickhouse-secret",
							Key:  "password",
						},
					},
					ServiceAccount: controllerutil.ConstStrToPointer(controllerutil.SparkServiceAccount),
				},
			},
			Executor: sparkv1.ExecutorSpec{
				CoreRequest: &newTAD.Spec.ExecutorCoreRequest,
				SparkPodSpec: sparkv1.SparkPodSpec{
					Memory: &newTAD.Spec.ExecutorMemory,
					Labels: map[string]string{
						"version": controllerutil.SparkVersion,
					},
					EnvSecretKeyRefs: map[string]sparkv1.NameKey{
						"CH_USERNAME": {
							Name: "clickhouse-secret",
							Key:  "username",
						},
						"CH_PASSWORD": {
							Name: "clickhouse-secret",
							Key:  "password",
						},
					},
				},
				Instances: &sparkResourceArgs.executorInstances,
			},
		},
	}
	err = CreateSparkApplication(c.kubeClient, newTAD.Namespace, taDetectorApplication)
	if err != nil {
		return fmt.Errorf("failed to create Spark Application: %v", err)
	}
	klog.V(2).InfoS("Start SparkApplication", "id", taDetectorID, "ThroughputAnomalyDetector", newTAD.Name)

	return c.updateTADetectorStatus(
		newTAD,
		crdv1alpha1.ThroughputAnomalyDetectorStatus{
			State:            crdv1alpha1.ThroughputAnomalyDetectorStateScheduled,
			SparkApplication: taDetectorID,
			StartTime:        metav1.NewTime(time.Now()),
		},
	)
}

func (c *AnomalyDetectorController) updateTADetectorStatus(newTAD *crdv1alpha1.ThroughputAnomalyDetector, status crdv1alpha1.ThroughputAnomalyDetectorStatus) error {
	update := newTAD.DeepCopy()
	update.Status.State = status.State
	if status.SparkApplication != "" {
		update.Status.SparkApplication = status.SparkApplication
	}
	if status.CompletedStages != 0 {
		update.Status.CompletedStages = status.CompletedStages
	}
	if status.TotalStages != 0 {
		update.Status.TotalStages = status.TotalStages
	}
	if status.ErrorMsg != "" {
		update.Status.ErrorMsg = status.ErrorMsg
	}
	if !status.StartTime.IsZero() {
		update.Status.StartTime = status.StartTime
	}
	if !status.EndTime.IsZero() {
		update.Status.EndTime = status.EndTime
	}
	_, err := c.crdClient.CrdV1alpha1().ThroughputAnomalyDetectors(newTAD.Namespace).UpdateStatus(context.TODO(), update, metav1.UpdateOptions{})
	return err
}

func (c *AnomalyDetectorController) addPeriodicSync(key apimachinerytypes.NamespacedName) {
	c.periodicResyncSetMutex.Lock()
	defer c.periodicResyncSetMutex.Unlock()
	c.periodicResyncSet[key] = struct{}{}
}

func (c *AnomalyDetectorController) stopPeriodicSync(key apimachinerytypes.NamespacedName) {
	c.periodicResyncSetMutex.Lock()
	defer c.periodicResyncSetMutex.Unlock()
	delete(c.periodicResyncSet, key)
}

func (c *AnomalyDetectorController) GetThroughputAnomalyDetector(namespace, name string) (*crdv1alpha1.ThroughputAnomalyDetector, error) {
	return c.anomalyDetectorLister.ThroughputAnomalyDetectors(namespace).Get(name)
}

func (c *AnomalyDetectorController) ListThroughputAnomalyDetector(namespace string) ([]*crdv1alpha1.ThroughputAnomalyDetector, error) {
	return c.anomalyDetectorLister.ThroughputAnomalyDetectors(namespace).List(labels.Everything())
}

func (c *AnomalyDetectorController) DeleteThroughputAnomalyDetector(namespace, name string) error {
	return c.crdClient.CrdV1alpha1().ThroughputAnomalyDetectors(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (c *AnomalyDetectorController) CreateThroughputAnomalyDetector(namespace string, ThroughputAnomalyDetector *crdv1alpha1.ThroughputAnomalyDetector) (*crdv1alpha1.ThroughputAnomalyDetector, error) {
	return c.crdClient.CrdV1alpha1().ThroughputAnomalyDetectors(namespace).Create(context.TODO(), ThroughputAnomalyDetector, metav1.CreateOptions{})
}

func getTADetectorStatus(client kubernetes.Interface, id string, namespace string) (state string, errorMessage string, err error) {
	sparkApplication, err := GetSparkApplication(client, "tad-"+id, namespace)
	if err != nil {
		return state, errorMessage, err
	}
	state = strings.TrimSpace(string(sparkApplication.Status.AppState.State))
	errorMessage = strings.TrimSpace(string(sparkApplication.Status.AppState.ErrorMessage))

	return state, errorMessage, nil
}

type illeagelArguementError struct {
	error
}
