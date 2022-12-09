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

package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func GetServiceAddr(client kubernetes.Interface, serviceName, serviceNamespace string, protocol v1.Protocol) (string, int, error) {
	var serviceIP string
	var servicePort int
	service, err := client.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	serviceIP = service.Spec.ClusterIP
	for _, port := range service.Spec.Ports {
		if port.Protocol == protocol {
			servicePort = int(port.Port)
		}
	}
	if servicePort == 0 {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: no %s service port", serviceName, protocol)
	}
	return serviceIP, servicePort, nil
}

func CreateK8sClient() (client kubernetes.Interface, err error) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return client, fmt.Errorf("error when generating KubeConfig: %v", err)
	}
	client, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return client, fmt.Errorf("error when generating kubernetes client: %v", err)
	}
	return client, nil
}
