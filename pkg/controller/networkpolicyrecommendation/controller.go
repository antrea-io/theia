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
	"time"

	apimachineryerrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/client/clientset/versioned"
	crdv1a1informers "antrea.io/theia/pkg/client/informers/externalversions/crd/v1alpha1"
	"antrea.io/theia/pkg/client/listers/crd/v1alpha1"
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
)

type NPRecommendationController struct {
	crdClient versioned.Interface

	npRecommendationInformer cache.SharedIndexInformer
	npRecommendationLister   v1alpha1.NetworkPolicyRecommendationLister
	npRecommendationSynced   cache.InformerSynced
	// queue maintains the Service objects that need to be synced.
	queue workqueue.RateLimitingInterface
}

func NewNPRecommendationController(
	crdClient versioned.Interface,
	npRecommendationInformer crdv1a1informers.NetworkPolicyRecommendationInformer,
) *NPRecommendationController {
	c := &NPRecommendationController{
		crdClient:                crdClient,
		queue:                    workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "npRecommendation"),
		npRecommendationInformer: npRecommendationInformer.Informer(),
		npRecommendationLister:   npRecommendationInformer.Lister(),
		npRecommendationSynced:   npRecommendationInformer.Informer().HasSynced,
	}

	c.npRecommendationInformer.AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addNPRecommendation,
			DeleteFunc: c.deleteNPRecommendation,
		},
		resyncPeriod,
	)

	return c
}

func (c *NPRecommendationController) addNPRecommendation(obj interface{}) {
	npReco, _ := obj.(*crdv1alpha1.NetworkPolicyRecommendation)
	klog.V(2).Infof("Processing NP Recommendation %s ADD event, labels: %v", npReco.Name, npReco.Labels)
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
			klog.Errorf("Error decoding object when deleting NP Recommendation, invalid type: %v", old)
			return
		}
		npReco, ok = tombstone.Obj.(*crdv1alpha1.NetworkPolicyRecommendation)
		if !ok {
			klog.Errorf("Error decoding object tombstone when deleting NP Recommendation, invalid type: %v", tombstone.Obj)
			return
		}
	}
	klog.V(2).Infof("Processing NP Recommendation %s DELETE event, labels: %v", npReco.Name, npReco.Labels)
	namespacedName := apimachinerytypes.NamespacedName{
		Namespace: npReco.Namespace,
		Name:      npReco.Name,
	}
	c.queue.Add(namespacedName)
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

	for i := 0; i < defaultWorkers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker is a long-running function that will continually call the processNextWorkItem function in
// order to read and process a message on the workqueue.
func (c *NPRecommendationController) worker() {
	for c.processNextWorkItem() {
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
		klog.Errorf("Expected NP Recommendation in work queue but got %#v", obj)
		return true
	} else if err := c.syncNPRecommendation(key); err == nil {
		// If no error occurs we Forget this item so it does not get queued again until
		// another change happens.
		c.queue.Forget(key)
	} else {
		// Put the item back on the workqueue to handle any transient errors.
		c.queue.AddRateLimited(key)
		klog.Errorf("Error syncing NP Recommendation %s, requeuing. Error: %v", key, err)
	}
	return true
}

func (c *NPRecommendationController) syncNPRecommendation(key apimachinerytypes.NamespacedName) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing NP Recommendation for %s. (%v)", key, time.Since(startTime))
	}()

	npReco, err := c.npRecommendationLister.NetworkPolicyRecommendations(key.Namespace).Get(key.Name)
	if err != nil {
		// NetworkPolicyRecommendation already deleted
		if apimachineryerrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	klog.V(4).Infof("Syncing NP Recommendation %v", npReco)
	// TODO (wshaoquan): add handling logic here
	return nil
}

func (c *NPRecommendationController) GetNetworkPolicyRecommendation(namespace, name string) (*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return c.npRecommendationLister.NetworkPolicyRecommendations(namespace).Get(name)
}
