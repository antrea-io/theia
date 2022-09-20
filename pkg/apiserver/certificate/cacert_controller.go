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

package certificate

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"antrea.io/theia/pkg/util/env"
)

const (
	CAConfigMapKey = "ca.crt"
)

// CACertController is responsible for taking the CA certificate from the
// caContentProvider and publishing it to the ConfigMap and the APIServices.
type CACertController struct {
	mutex sync.RWMutex

	// caContentProvider provides the very latest content of the ca bundle.
	caContentProvider dynamiccertificates.CAContentProvider
	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	client   kubernetes.Interface
	caConfig *CAConfig
}

var _ dynamiccertificates.Listener = &CACertController{}

func GetCAConfigMapNamespace() string {
	return env.GetTheiaNamespace()
}

func newCACertController(caContentProvider dynamiccertificates.CAContentProvider,
	client kubernetes.Interface,
	caConfig *CAConfig,
) *CACertController {
	c := &CACertController{
		caContentProvider: caContentProvider,
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CACertController"),
		client:            client,
		caConfig:          caConfig,
	}
	if notifier, ok := caContentProvider.(dynamiccertificates.Notifier); ok {
		notifier.AddListener(c)
	}
	return c
}

func (c *CACertController) UpdateCertificate(ctx context.Context) error {
	if controller, ok := c.caContentProvider.(dynamiccertificates.ControllerRunner); ok {
		if err := controller.RunOnce(ctx); err != nil {
			klog.Warningf("Updating of CA content failed: %v", err)
			c.Enqueue()
			return err
		}
	}

	return nil
}

// getCertificate exposes the certificate for testing.
func (c *CACertController) getCertificate() []byte {
	return c.caContentProvider.CurrentCABundleContent()
}

// Enqueue will be called after CACertController is registered as a listener of CA cert change.
func (c *CACertController) Enqueue() {
	// The key can be anything as we only have single item.
	c.queue.Add("key")
}

func (c *CACertController) syncCACert() error {
	caCert := c.caContentProvider.CurrentCABundleContent()

	if err := c.syncConfigMap(caCert); err != nil {
		return err
	}

	return nil
}

// syncConfigMap updates the ConfigMap that holds the CA bundle, which will be read by API clients.
func (c *CACertController) syncConfigMap(caCert []byte) error {
	klog.InfoS("Syncing CA certificate with ConfigMap")
	// Use the Theia manager Pod Namespace for the CA cert ConfigMap.
	caConfigMapNamespace := GetCAConfigMapNamespace()
	caConfigMap, err := c.client.CoreV1().ConfigMaps(caConfigMapNamespace).Get(context.TODO(), c.caConfig.CAConfigMapName, metav1.GetOptions{})
	exists := true
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("error getting ConfigMap %s: %v", c.caConfig.CAConfigMapName, err)
		}
		exists = false
		caConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.caConfig.CAConfigMapName,
				Namespace: caConfigMapNamespace,
				Labels: map[string]string{
					"app": "theia",
				},
			},
		}
	}
	if caConfigMap.Data != nil && caConfigMap.Data[CAConfigMapKey] == string(caCert) {
		return nil
	}
	caConfigMap.Data = map[string]string{
		CAConfigMapKey: string(caCert),
	}
	if exists {
		if _, err := c.client.CoreV1().ConfigMaps(caConfigMapNamespace).Update(context.TODO(), caConfigMap, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("error updating ConfigMap %s: %v", c.caConfig.CAConfigMapName, err)
		}
	} else {
		if _, err := c.client.CoreV1().ConfigMaps(caConfigMapNamespace).Create(context.TODO(), caConfigMap, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("error creating ConfigMap %s: %v", c.caConfig.CAConfigMapName, err)
		}
	}
	return nil
}

// RunOnce runs a single sync step to ensure that we have a valid starting configuration.
func (c *CACertController) RunOnce(ctx context.Context) error {
	if controller, ok := c.caContentProvider.(dynamiccertificates.ControllerRunner); ok {
		if err := controller.RunOnce(ctx); err != nil {
			klog.Warningf("Initial population of CA content failed: %v", err)
			c.Enqueue()
			return err
		}
	}
	if err := c.syncCACert(); err != nil {
		klog.Warningf("Initial sync of CA content failed: %v", err)
		c.Enqueue()
		return err
	}
	return nil
}

// Run starts the CACertController and blocks until the context is canceled.
func (c *CACertController) Run(ctx context.Context, workers int) {
	defer c.queue.ShutDown()

	klog.InfoS("Starting CACertController")
	defer klog.InfoS("Shutting down CACertController")

	if controller, ok := c.caContentProvider.(dynamiccertificates.ControllerRunner); ok {
		// doesn't matter what workers say, only start one.
		go controller.Run(ctx, 1)
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, ctx.Done())

	<-ctx.Done()
}

func (c *CACertController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *CACertController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncCACert()
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	klog.ErrorS(err, "Error when syncing CA cert, requeuing")
	c.queue.AddRateLimited(key)

	return true
}
