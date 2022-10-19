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

package portforwarder

import (
	"context"
	"fmt"
	"io"
	"net/http"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/klog/v2"
)

type PortForwarder struct {
	config        *rest.Config
	clientset     kubernetes.Interface
	namespace     string
	name          string
	targetPort    int
	listenAddress string
	listenPort    int
	stopCh        chan struct{}
}

// This function creates Port Forwarder for a Pod
// After creating Port Forwarder object, call Start() on it to start forwarding
// channel and Stop() to terminate it
func NewPortForwarder(config *rest.Config, namespace string, pod string, targetPort int, listenAddress string, listenPort int) (*PortForwarder, error) {
	klog.V(2).Infof("Port forwarder requested for pod %s/%s: %s:%d -> %d", namespace, pod, listenAddress, listenPort, targetPort)

	pf := &PortForwarder{
		config:        config,
		namespace:     namespace,
		name:          pod,
		targetPort:    targetPort,
		listenAddress: listenAddress,
		listenPort:    listenPort,
	}

	var err error
	pf.clientset, err = kubernetes.NewForConfig(pf.config)
	if err != nil {
		return pf, fmt.Errorf("could not create kubernetes client: %v", err)
	}

	return pf, nil
}

// This function creates Port Forwarder for a Service by finding first Pod
// that belongs to the Service, and target Port for requested Service Port.
// This code is based upon kubectl port-forward implementation
// After creating Port Forwarder object, call Start() on it to start forwarding
// channel and Stop() to terminate it
func NewServicePortForwarder(config *rest.Config, namespace string, service string, servicePort int, listenAddress string, listenPort int) (*PortForwarder, error) {
	pf := &PortForwarder{
		config:        config,
		namespace:     namespace,
		listenAddress: listenAddress,
		listenPort:    listenPort,
	}

	var err error
	pf.clientset, err = kubernetes.NewForConfig(pf.config)
	if err != nil {
		return pf, fmt.Errorf("could not create kubernetes client: %v", err)
	}

	serviceObj, err := pf.clientset.CoreV1().Services(pf.namespace).Get(context.TODO(), service, metav1.GetOptions{})
	if err != nil {
		return pf, fmt.Errorf("failed to read Service %s: %v", service, err)
	}

	klog.V(2).Infof("Port forwarder requested for service %s/%s: %s:%d -> %d", namespace, service, listenAddress, listenPort, pf.targetPort)

	selector := labels.SelectorFromSet(serviceObj.Spec.Selector)
	listOptions := metav1.ListOptions{
		LabelSelector: selector.String(),
	}

	// for target Pod - take first Pod for the Service
	pods, err := pf.clientset.CoreV1().Pods(pf.namespace).List(context.TODO(), listOptions)

	if err != nil {
		return pf, fmt.Errorf("failed to read Pods for Service %s: %v", service, err)
	}
	if len(pods.Items) == 0 {
		return pf, fmt.Errorf("no Pods found for Service %s: %v", service, err)
	}

	pod := pods.Items[0]
	pf.name = pod.Name

	// find container port that corresponds to requested service port
	pf.targetPort, err = getContainerPortByServicePort(serviceObj, servicePort, &pod)
	if err != nil {
		return pf, err
	}
	return pf, nil
}

// get Container Port by Service Port, based on Service configuration
// This code is based upon kubectl port-forward implementation
func getContainerPortByServicePort(svc *v1.Service, port int, pod *v1.Pod) (int, error) {
	for _, portspec := range svc.Spec.Ports {
		if int(portspec.Port) != port {
			continue
		}
		if svc.Spec.ClusterIP == v1.ClusterIPNone {
			return port, nil
		}
		if portspec.TargetPort.Type == intstr.Int {
			if portspec.TargetPort.IntValue() == 0 {
				return int(portspec.Port), nil
			}
			return portspec.TargetPort.IntValue(), nil
		} else if portspec.TargetPort.Type == intstr.String && portspec.TargetPort.String() != "" {
			for _, container := range pod.Spec.Containers {
				for _, containerPortSpec := range container.Ports {
					if containerPortSpec.Name == portspec.TargetPort.String() {
						return int(containerPortSpec.ContainerPort), nil
					}
				}
			}
		}
	}
	return port, fmt.Errorf("service %s does not have Port %d", svc.Name, port)
}

// Start Port Forwarding channel
func (p *PortForwarder) Start() error {
	p.stopCh = make(chan struct{}, 1)
	readyCh := make(chan struct{})
	errCh := make(chan error, 1)

	url := p.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(p.namespace).
		Name(p.name).
		SubResource("portforward").URL()

	transport, upgrader, err := spdy.RoundTripperFor(p.config)
	if err != nil {
		return fmt.Errorf("failed to create dialer: %v", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", url)

	ports := []string{
		fmt.Sprintf("%d:%d", p.listenPort, p.targetPort),
	}

	addresses := []string{
		p.listenAddress,
	}

	pf, err := portforward.NewOnAddresses(dialer, addresses, ports, p.stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("port forward request failed: %v", err)
	}

	go func() {
		errCh <- pf.ForwardPorts()
	}()

	select {
	case err = <-errCh:
		return fmt.Errorf("port forward request failed: %v", err)
	case <-readyCh:
		return nil
	}
}

// Stop Port Forwarding channel
func (p *PortForwarder) Stop() {
	p.stopCh <- struct{}{}
}
