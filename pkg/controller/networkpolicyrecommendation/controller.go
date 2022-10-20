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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
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
	"antrea.io/theia/pkg/util/policyrecommendation"
	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const (
	controllerName = "NetworkPolicyRecommendationController"
	// Set resyncPeriod to 0 to disable resyncing.
	resyncPeriod time.Duration = 0
	// How long to wait before retrying the processing of an Service change.
	minRetryDelay = 5 * time.Second
	maxRetryDelay = 300 * time.Second
	// Default number of workers processing an Service change.
	defaultWorkers = 4
	// Time format for parsing input time
	inputTimeFormat = "2006-01-02 15:04:05"
	// Time format for parsing time from ClickHouse
	clickHouseTimeFormat = "2006-01-02T15:04:05Z"
	// SparkApplication id index name for RecommendedNetworkPolicy
	idIndex          = "id"
	k8sQuantitiesReg = "^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	// Spark related parameters
	sparkImage           = "projects.registry.vmware.com/antrea/theia-policy-recommendation:latest"
	sparkImagePullPolicy = "IfNotPresent"
	sparkAppFile         = "local:///opt/spark/work-dir/policy_recommendation_job.py"
	sparkServiceAccount  = "policy-recommendation-spark"
	sparkVersion         = "3.1.1"
	sparkPort            = 4040
)

var (
	// Spark Application CRUD functions, for unit tests
	CreateSparkApplication   = createSparkApplication
	DeleteSparkApplication   = deleteSparkApplication
	ListSparkApplication     = listSparkApplication
	GetSparkApplication      = getSparkApplication
	GetSparkMonitoringSvcDNS = getSparkMonitoringSvcDNS
	// For NPR in scheduled or running state, check its status periodically
	npRecommendationResyncPeriod = 10 * time.Second
)

type NPRecommendationController struct {
	crdClient  versioned.Interface
	kubeClient kubernetes.Interface

	npRecommendationInformer cache.SharedIndexInformer
	npRecommendationLister   v1alpha1.NetworkPolicyRecommendationLister
	npRecommendationSynced   cache.InformerSynced
	recommendedNPInformer    cache.SharedIndexInformer
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

func NewNPRecommendationController(
	crdClient versioned.Interface,
	kubeClient kubernetes.Interface,
	npRecommendationInformer crdv1a1informers.NetworkPolicyRecommendationInformer,
	recommendedNPInformer crdv1a1informers.RecommendedNetworkPolicyInformer,
) *NPRecommendationController {
	c := &NPRecommendationController{
		crdClient:                crdClient,
		kubeClient:               kubeClient,
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "npRecommendation"),
		deletionQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "npRecommendationCleanup"),
		npRecommendationInformer: npRecommendationInformer.Informer(),
		npRecommendationLister:   npRecommendationInformer.Lister(),
		npRecommendationSynced:   npRecommendationInformer.Informer().HasSynced,
		recommendedNPInformer:    recommendedNPInformer.Informer(),
		periodicResyncSet:        make(map[apimachinerytypes.NamespacedName]struct{}),
	}

	c.npRecommendationInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addNPRecommendation,
			UpdateFunc: c.updateNPRecommendation,
			DeleteFunc: c.deleteNPRecommendation,
		},
		resyncPeriod,
	)

	// Add SparkApplication ID index for RecommendedNetworkPolicy
	c.recommendedNPInformer.AddIndexers(cache.Indexers{idIndex: rnpIdIndexFunc})

	return c
}

func rnpIdIndexFunc(obj interface{}) ([]string, error) {
	recommendedNP, ok := obj.(*crdv1alpha1.RecommendedNetworkPolicy)
	if !ok {
		return nil, fmt.Errorf("obj is not RecommendedNetworkPolicy: %v", obj)
	}
	return []string{recommendedNP.Spec.Id}, nil
}

func (c *NPRecommendationController) addNPRecommendation(obj interface{}) {
	npReco, ok := obj.(*crdv1alpha1.NetworkPolicyRecommendation)
	if !ok {
		klog.ErrorS(nil, "fail to convert to NetworkPolicyRecommendation", "object", obj)
		return
	}
	klog.V(2).InfoS("Processing NP Recommendation ADD event", "name", npReco.Name, "labels", npReco.Labels)
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: npReco.Namespace,
		Name:      npReco.Name,
	}
	c.queue.Add(namespacedName)
}

func (c *NPRecommendationController) updateNPRecommendation(_, new interface{}) {
	npReco, ok := new.(*crdv1alpha1.NetworkPolicyRecommendation)
	if !ok {
		klog.ErrorS(nil, "fail to convert to NetworkPolicyRecommendation", "object", new)
		return
	}
	klog.V(2).InfoS("Processing NP Recommendation UPDATE event", "name", npReco.Name, "labels", npReco.Labels)
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: npReco.Namespace,
		Name:      npReco.Name,
	}
	c.queue.Add(namespacedName)
}

func (c *NPRecommendationController) deleteNPRecommendation(old interface{}) {
	npReco, ok := old.(*crdv1alpha1.NetworkPolicyRecommendation)
	if !ok {
		tombstone, ok := old.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.ErrorS(nil, "Error decoding object when deleting NP Recommendation", "oldObject", old)
			return
		}
		npReco, ok = tombstone.Obj.(*crdv1alpha1.NetworkPolicyRecommendation)
		if !ok {
			klog.ErrorS(nil, "Error decoding object tombstone when deleting NP Recommendation", "tombstone", tombstone.Obj)
			return
		}
	}
	klog.V(2).InfoS("Processing NP Recommendation DELETE event", "name", npReco.Name, "labels", npReco.Labels)
	// remove NPRecommendation from periodic synchronization list in case it is deleted before completing
	c.stopPeriodicSync(apimachinerytypes.NamespacedName{
		Namespace: npReco.Namespace,
		Name:      npReco.Name,
	})
	// Add SparkApplication and Namespace information to deletionQueue for cleanup
	if npReco.Status.SparkApplication != "" {
		namespacedId := NamespacedId{
			Namespace: npReco.Namespace,
			Id:        npReco.Status.SparkApplication,
		}
		c.deletionQueue.Add(namespacedId)
	}
}

// Run will create defaultWorkers workers (go routines) which will process the Service events from the
// workqueue.
func (c *NPRecommendationController) Run(stopCh <-chan struct{}) {
	defer c.queue.ShutDown()

	klog.InfoS("Starting controller", "name", controllerName)
	defer klog.InfoS("Shutting down controller", "name", controllerName)

	if !cache.WaitForNamedCacheSync(controllerName, stopCh, c.npRecommendationSynced) {
		return
	}

	go func() {
		wait.Until(c.resyncNPRecommendation, npRecommendationResyncPeriod, stopCh)
	}()

	go wait.Until(c.deletionworker, time.Second, stopCh)

	for i := 0; i < defaultWorkers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *NPRecommendationController) deletionworker() {
	for c.processNextDeletionWorkItem() {
	}
}

func (c *NPRecommendationController) processNextDeletionWorkItem() bool {
	obj, quit := c.deletionQueue.Get()
	if quit {
		return false
	}
	defer c.deletionQueue.Done(obj)
	if key, ok := obj.(NamespacedId); !ok {
		c.queue.Forget(obj)
		klog.ErrorS(nil, "Expected Spark Application namespaced id in work queue", "got", obj)
		return true
	} else if err := c.cleanupNPRecommendation(key.Namespace, key.Id); err == nil {
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
func (c *NPRecommendationController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *NPRecommendationController) resyncNPRecommendation() {
	c.periodicResyncSetMutex.Lock()
	nprs := make([]apimachinerytypes.NamespacedName, 0, len(c.periodicResyncSet))
	for nprNamespacedName := range c.periodicResyncSet {
		nprs = append(nprs, nprNamespacedName)
	}
	c.periodicResyncSetMutex.Unlock()
	for _, nprNamespacedName := range nprs {
		c.queue.Add(nprNamespacedName)
	}
}

func (c *NPRecommendationController) processNextWorkItem() bool {
	obj, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(obj)
	if key, ok := obj.(apimachinerytypes.NamespacedName); !ok {
		c.queue.Forget(obj)
		klog.ErrorS(nil, "Expected NP Recommendation in work queue", "got", obj)
		return true
	} else if err := c.syncNPRecommendation(key); err == nil {
		// If no error occurs we forget this item so it does not get queued again until
		// another change happens.
		c.queue.Forget(key)
	} else {
		// Put the item back on the workqueue to handle any transient errors.
		c.queue.AddRateLimited(key)
		klog.ErrorS(err, "Error when syncing NP Recommendation, requeuing", "key", key)
	}
	return true
}

func (c *NPRecommendationController) syncNPRecommendation(key apimachinerytypes.NamespacedName) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).InfoS("Finished syncing NP Recommendation", "key", key, "time", time.Since(startTime))
	}()

	npReco, err := c.npRecommendationLister.NetworkPolicyRecommendations(key.Namespace).Get(key.Name)
	if err != nil {
		// NetworkPolicyRecommendation already deleted
		if apimachineryerrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	klog.V(4).Infof("Syncing NP Recommendation", "npReco", npReco)

	switch npReco.Status.State {
	case "", crdv1alpha1.NPRecommendationStateNew:
		err = c.startJob(npReco)
	case crdv1alpha1.NPRecommendationStateScheduled:
		_, err = c.checkSparkApplicationStatus(npReco)
	case crdv1alpha1.NPRecommendationStateRunning:
		err = c.updateProgress(npReco)
	case crdv1alpha1.NPRecommendationStateCompleted:
		err = c.updateResult(npReco)
	}
	return err
}

func (c *NPRecommendationController) cleanupNPRecommendation(namespace string, sparkApplicationId string) error {
	// Delete the Spark Application if exists
	err := deleteSparkApplicationIfExists(c.kubeClient, namespace, sparkApplicationId)
	if err != nil {
		return fmt.Errorf("failed to delete the SparkApplication %s, error: %v", sparkApplicationId, err)
	}
	// Delete the RNP if exists
	rnps, err := c.recommendedNPInformer.GetIndexer().ByIndex(idIndex, sparkApplicationId)
	if err != nil {
		return fmt.Errorf("failed to get the RecommendedNetworkPolicy when deleting, error: %v", err)
	}
	var undeletedRnpNames []string
	var errorList []error
	for _, obj := range rnps {
		recommendedNetworkPolicy := obj.(*crdv1alpha1.RecommendedNetworkPolicy)
		err = c.crdClient.CrdV1alpha1().RecommendedNetworkPolicies(namespace).Delete(context.TODO(), recommendedNetworkPolicy.Name, metav1.DeleteOptions{})
		if err != nil {
			undeletedRnpNames = append(undeletedRnpNames, recommendedNetworkPolicy.Name)
			errorList = append(errorList, err)
		}
	}
	if len(errorList) > 0 {
		return fmt.Errorf("failed to deleted RecommendedNetworkPolicies: %v, error: %v", undeletedRnpNames, errorList)
	}
	// Delete the result from the ClickHouse
	if c.clickhouseConnect == nil {
		c.clickhouseConnect, err = setupClickHouseConnection(c.kubeClient, namespace)
		if err != nil {
			return err
		}
	}
	return deletePolicyRecommendationResult(c.clickhouseConnect, sparkApplicationId)
}

func (c *NPRecommendationController) updateResult(npReco *crdv1alpha1.NetworkPolicyRecommendation) error {
	namespacedName := apimachinerytypes.NamespacedName{
		Name:      npReco.Name,
		Namespace: npReco.Namespace,
	}
	// Stop periodical job
	c.stopPeriodicSync(namespacedName)
	// Delete related SparkApplication CR
	if npReco.Status.SparkApplication == "" {
		return c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateFailed,
				ErrorMsg: "Spark Application should be started before updating results",
			},
		)
	}

	if npReco.Status.RecommendedNP == nil {
		err := deleteSparkApplicationIfExists(c.kubeClient, npReco.Namespace, npReco.Status.SparkApplication)
		if err != nil {
			return fmt.Errorf("fail to delete Spark Application: %s, error: %v", npReco.Status.SparkApplication, err)
		}
		var recommendedNetworkPolicy *crdv1alpha1.RecommendedNetworkPolicy
		// Check if RecommendedNetworkPolicy CR is created
		rnps, err := c.recommendedNPInformer.GetIndexer().ByIndex(idIndex, npReco.Status.SparkApplication)
		if err == nil && len(rnps) > 0 {
			// Use the existing RecommendedNetworkPolicy CR
			recommendedNetworkPolicy = rnps[0].(*crdv1alpha1.RecommendedNetworkPolicy)
			if len(rnps) > 1 {
				klog.V(4).InfoS("More than 1 RecommendedNetworkPolicy", "id", npReco.Status.SparkApplication)
			}
		} else {
			if err != nil {
				klog.V(4).InfoS("Failed to find RecommendedNetworkPolicy", "id", npReco.Status.SparkApplication)
			}
			// Get result from database
			if c.clickhouseConnect == nil {
				c.clickhouseConnect, err = setupClickHouseConnection(c.kubeClient, npReco.Namespace)
				if err != nil {
					return err
				}
			}
			recommendedNetworkPolicy, err = getPolicyRecommendationResult(c.clickhouseConnect, npReco.Status.SparkApplication)
			if err != nil {
				return err
			}
			// Create RecommendedNetworkPolicy CR
			recommendedNetworkPolicy, err = c.crdClient.CrdV1alpha1().RecommendedNetworkPolicies(npReco.Namespace).Create(context.TODO(), recommendedNetworkPolicy, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			klog.V(2).InfoS("Created RecommendedNetworkPolicy", "RecommendedNetworkPolicy", recommendedNetworkPolicy.Name, "NetworkRecommendationPolicy", npReco.Name)
		}
		return c.updateNPRecommendationStatus(npReco, crdv1alpha1.NetworkPolicyRecommendationStatus{
			State:         crdv1alpha1.NPRecommendationStateCompleted,
			RecommendedNP: recommendedNetworkPolicy,
			EndTime:       metav1.NewTime(time.Now()),
		})
	}
	return nil
}

func (c *NPRecommendationController) updateProgress(npReco *crdv1alpha1.NetworkPolicyRecommendation) error {
	// Check the status before checking the progress in case the job is failed or completed
	state, err := c.checkSparkApplicationStatus(npReco)
	if err != nil {
		return err
	}
	if state != crdv1alpha1.NPRecommendationStateRunning {
		return nil
	}
	endpoint := GetSparkMonitoringSvcDNS(npReco.Status.SparkApplication, npReco.Namespace)
	completedStages, totalStages, err := getPolicyRecommendationProgress(endpoint)
	if err != nil {
		// The Spark Monitoring Service may not start or closed at this point due to the async
		// between Spark operator and this controller.
		// As we periodically check the progress, we do not need to requeue this failure.
		klog.V(4).ErrorS(err, "Failed to get the progress of the policy recommendation job")
		return nil
	}
	klog.V(4).InfoS("Got Spark Application progress", "completedStages", completedStages, "totalStages", totalStages, "NetworkRecommendationPolicy", npReco.Name)
	return c.updateNPRecommendationStatus(
		npReco,
		crdv1alpha1.NetworkPolicyRecommendationStatus{
			State:           crdv1alpha1.NPRecommendationStateRunning,
			CompletedStages: completedStages,
			TotalStages:     totalStages,
		},
	)
}

func (c *NPRecommendationController) checkSparkApplicationStatus(npReco *crdv1alpha1.NetworkPolicyRecommendation) (string, error) {
	if npReco.Status.SparkApplication == "" {
		return "", c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateFailed,
				ErrorMsg: "Spark Application should be started before status checking",
			},
		)
	}

	state, errorMessage, err := getPolicyRecommendationStatus(c.kubeClient, npReco.Status.SparkApplication, npReco.Namespace)
	if err != nil {
		return state, err
	}
	klog.V(4).InfoS("Got Spark Application state", "state", state, "NetworkRecommendationPolicy", npReco.Name)
	if state == "RUNNING" {
		return state, c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateRunning,
				ErrorMsg: errorMessage,
			},
		)
	} else if state == "COMPLETED" {
		return state, c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateCompleted,
				ErrorMsg: errorMessage,
			},
		)
	} else if state == "FAILED" || state == "SUBMISSION_FAILED" || state == "FAILING" || state == "INVALIDATING" {
		return state, c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateFailed,
				ErrorMsg: fmt.Sprintf("policy recommendation job failed, state: %s, error message: %v", state, errorMessage),
			},
		)
	}
	return state, nil
}

func (c *NPRecommendationController) startJob(npReco *crdv1alpha1.NetworkPolicyRecommendation) error {
	// Validate Cluster readiness
	if err := validateCluster(c.kubeClient, npReco.Namespace); err != nil {
		return err
	}
	err := c.startSparkApplication(npReco)
	// Mark the NetworkPolicyRecommendation as failed and not retry if it failed due to illegal arguments in request
	if err != nil && reflect.TypeOf(err) == reflect.TypeOf(IlleagelArguementError{}) {
		return c.updateNPRecommendationStatus(
			npReco,
			crdv1alpha1.NetworkPolicyRecommendationStatus{
				State:    crdv1alpha1.NPRecommendationStateFailed,
				ErrorMsg: fmt.Sprintf("error in creating NetworkPolicyRecommendation: %v", err),
			},
		)
	}
	// Schedule periodical resync for successful starting
	if err == nil {
		c.addPeriodicSync(apimachinerytypes.NamespacedName{
			Name:      npReco.Name,
			Namespace: npReco.Namespace,
		})
	}
	return err
}

func (c *NPRecommendationController) startSparkApplication(npReco *crdv1alpha1.NetworkPolicyRecommendation) error {
	var recoJobArgs []string
	if npReco.Spec.JobType != "initial" && npReco.Spec.JobType != "subsequent" {
		return IlleagelArguementError{fmt.Errorf("invalid request: recommendation type should be 'initial' or 'subsequent'")}
	}
	recoJobArgs = append(recoJobArgs, "--type", npReco.Spec.JobType)

	if npReco.Spec.Limit < 0 {
		return IlleagelArguementError{fmt.Errorf("invalid request: limit should be an integer >= 0")}
	}
	recoJobArgs = append(recoJobArgs, "--limit", strconv.Itoa(npReco.Spec.Limit))

	var policyTypeArg int
	if npReco.Spec.PolicyType == "anp-deny-applied" {
		policyTypeArg = 1
	} else if npReco.Spec.PolicyType == "anp-deny-all" {
		policyTypeArg = 2
	} else if npReco.Spec.PolicyType == "k8s-np" {
		policyTypeArg = 3
	} else {
		return IlleagelArguementError{fmt.Errorf("invalid request: type of generated NetworkPolicy should be anp-deny-applied or anp-deny-all or k8s-np")}
	}
	recoJobArgs = append(recoJobArgs, "--option", strconv.Itoa(policyTypeArg))

	if !npReco.Spec.StartInterval.IsZero() {
		recoJobArgs = append(recoJobArgs, "--start_time", npReco.Spec.StartInterval.Format(inputTimeFormat))
	}
	if !npReco.Spec.EndInterval.IsZero() {
		endAfterStart := npReco.Spec.EndInterval.After(npReco.Spec.StartInterval.Time)
		if !endAfterStart {
			return IlleagelArguementError{fmt.Errorf("invalid request: EndInterval should be after StartInterval")}
		}
		recoJobArgs = append(recoJobArgs, "--end_time", npReco.Spec.EndInterval.Format(inputTimeFormat))
	}

	if len(npReco.Spec.NSAllowList) > 0 {
		nsAllowListStr := strings.Join(npReco.Spec.NSAllowList, "\",\"")
		nsAllowListStr = "[\"" + nsAllowListStr + "\"]"
		recoJobArgs = append(recoJobArgs, "--ns_allow_list", nsAllowListStr)
	}

	recoJobArgs = append(recoJobArgs, "--rm_labels", strconv.FormatBool(npReco.Spec.ExcludeLabels))
	recoJobArgs = append(recoJobArgs, "--to_services", strconv.FormatBool(npReco.Spec.ToServices))

	sparkResourceArgs := struct {
		executorInstances   int32
		driverCoreRequest   string
		driverMemory        string
		executorCoreRequest string
		executorMemory      string
	}{}

	if npReco.Spec.ExecutorInstances < 0 {
		return IlleagelArguementError{fmt.Errorf("invalid request: ExecutorInstances should be an integer >= 0")}
	}
	sparkResourceArgs.executorInstances = int32(npReco.Spec.ExecutorInstances)

	matchResult, err := regexp.MatchString(k8sQuantitiesReg, npReco.Spec.DriverCoreRequest)
	if err != nil || !matchResult {
		return IlleagelArguementError{fmt.Errorf("invalid request: DriverCoreRequest should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.driverCoreRequest = npReco.Spec.DriverCoreRequest

	matchResult, err = regexp.MatchString(k8sQuantitiesReg, npReco.Spec.DriverMemory)
	if err != nil || !matchResult {
		return IlleagelArguementError{fmt.Errorf("invalid request: DriverMemory should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.driverMemory = npReco.Spec.DriverMemory

	matchResult, err = regexp.MatchString(k8sQuantitiesReg, npReco.Spec.ExecutorCoreRequest)
	if err != nil || !matchResult {
		return IlleagelArguementError{fmt.Errorf("invalid request: ExecutorCoreRequest should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.executorCoreRequest = npReco.Spec.ExecutorCoreRequest

	matchResult, err = regexp.MatchString(k8sQuantitiesReg, npReco.Spec.ExecutorMemory)
	if err != nil || !matchResult {
		return IlleagelArguementError{fmt.Errorf("invalid request: ExecutorMemory should conform to the Kubernetes resource quantity convention")}
	}
	sparkResourceArgs.executorMemory = npReco.Spec.ExecutorMemory

	err = policyrecommendation.ParseRecommendationName(npReco.Name)
	if err != nil {
		return IlleagelArguementError{fmt.Errorf("invalid request: Policy recommendation job name is invalid: %s", err)}
	}
	recommendationID := npReco.Name[3:]
	recoJobArgs = append(recoJobArgs, "--id", recommendationID)
	recommendationApplication := &sparkv1.SparkApplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "sparkoperator.k8s.io/v1beta2",
			Kind:       "SparkApplication",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      npReco.Name,
			Namespace: npReco.Namespace,
		},
		Spec: sparkv1.SparkApplicationSpec{
			Type:                "Python",
			SparkVersion:        sparkVersion,
			Mode:                "cluster",
			Image:               constStrToPointer(sparkImage),
			ImagePullPolicy:     constStrToPointer(sparkImagePullPolicy),
			MainApplicationFile: constStrToPointer(sparkAppFile),
			Arguments:           recoJobArgs,
			Driver: sparkv1.DriverSpec{
				CoreRequest: &npReco.Spec.DriverCoreRequest,
				SparkPodSpec: sparkv1.SparkPodSpec{
					Memory: &npReco.Spec.DriverMemory,
					Labels: map[string]string{
						"version": sparkVersion,
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
					ServiceAccount: constStrToPointer(sparkServiceAccount),
				},
			},
			Executor: sparkv1.ExecutorSpec{
				CoreRequest: &npReco.Spec.ExecutorCoreRequest,
				SparkPodSpec: sparkv1.SparkPodSpec{
					Memory: &npReco.Spec.ExecutorMemory,
					Labels: map[string]string{
						"version": sparkVersion,
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
	err = CreateSparkApplication(c.kubeClient, npReco.Namespace, recommendationApplication)
	if err != nil {
		return fmt.Errorf("failed to create Spark Application: %v", err)
	}
	klog.V(2).InfoS("Start SparkApplication", "id", recommendationID, "NetworkPolicyRecommendation", npReco.Name)

	return c.updateNPRecommendationStatus(
		npReco,
		crdv1alpha1.NetworkPolicyRecommendationStatus{
			State:            crdv1alpha1.NPRecommendationStateScheduled,
			SparkApplication: recommendationID,
			StartTime:        metav1.NewTime(time.Now()),
		},
	)
}

func (c *NPRecommendationController) updateNPRecommendationStatus(npReco *crdv1alpha1.NetworkPolicyRecommendation, status crdv1alpha1.NetworkPolicyRecommendationStatus) error {
	update := npReco.DeepCopy()
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
	if status.RecommendedNP != nil {
		update.Status.RecommendedNP = status.RecommendedNP
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
	_, err := c.crdClient.CrdV1alpha1().NetworkPolicyRecommendations(npReco.Namespace).UpdateStatus(context.TODO(), update, metav1.UpdateOptions{})
	return err
}

func (c *NPRecommendationController) addPeriodicSync(key apimachinerytypes.NamespacedName) {
	c.periodicResyncSetMutex.Lock()
	defer c.periodicResyncSetMutex.Unlock()
	c.periodicResyncSet[key] = struct{}{}
}

func (c *NPRecommendationController) stopPeriodicSync(key apimachinerytypes.NamespacedName) {
	c.periodicResyncSetMutex.Lock()
	defer c.periodicResyncSetMutex.Unlock()
	delete(c.periodicResyncSet, key)
}

func (c *NPRecommendationController) GetNetworkPolicyRecommendation(namespace, name string) (*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return c.npRecommendationLister.NetworkPolicyRecommendations(namespace).Get(name)
}

func (c *NPRecommendationController) ListNetworkPolicyRecommendation(namespace string) ([]*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return c.npRecommendationLister.NetworkPolicyRecommendations(namespace).List(labels.Everything())
}

func (c *NPRecommendationController) DeleteNetworkPolicyRecommendation(namespace, name string) error {
	return c.crdClient.CrdV1alpha1().NetworkPolicyRecommendations(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (c *NPRecommendationController) CreateNetworkPolicyRecommendation(namespace string, networkPolicyRecommendation *crdv1alpha1.NetworkPolicyRecommendation) (*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return c.crdClient.CrdV1alpha1().NetworkPolicyRecommendations(namespace).Create(context.TODO(), networkPolicyRecommendation, metav1.CreateOptions{})
}
