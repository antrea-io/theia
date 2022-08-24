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

package main

import (
	"fmt"
	"time"

	"antrea.io/antrea/pkg/log"
	"antrea.io/antrea/pkg/signals"
	"antrea.io/antrea/pkg/util/cipher"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"antrea.io/theia/pkg/apiserver"
	crdclientset "antrea.io/theia/pkg/client/clientset/versioned"
	crdinformers "antrea.io/theia/pkg/client/informers/externalversions"
	"antrea.io/theia/pkg/controller/networkpolicyrecommendation"
)

// informerDefaultResync is the default resync period if a handler doesn't specify one.
// Use the same default value as kube-controller-manager:
// https://github.com/kubernetes/kubernetes/blob/release-1.17/pkg/controller/apis/config/v1alpha1/defaults.go#L120
const informerDefaultResync = 12 * time.Hour

func run(o *Options) error {
	klog.InfoS("Theia manager starting...")
	// Set up signal capture: the first SIGTERM / SIGINT signal is handled gracefully and will
	// cause the stopCh channel to be closed; if another signal is received before the program
	// exits, we will force exit.
	stopCh := signals.RegisterSignalHandlers()

	log.StartLogFileNumberMonitor(stopCh)

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("error when generating KubeConfig: %v", err)
	}
	crdClient, err := crdclientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when generating CRD client: %v", err)
	}
	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, informerDefaultResync)
	npRecommendationInformer := crdInformerFactory.Crd().V1alpha1().NetworkPolicyRecommendations()
	npRecoController := networkpolicyrecommendation.NewNPRecommendationController(crdClient, npRecommendationInformer)

	cipherSuites, err := cipher.GenerateCipherSuitesList(o.config.APIServer.TLSCipherSuites)
	if err != nil {
		return fmt.Errorf("error when generating Cipher Suite list: %v", err)
	}
	apiServer, err := apiserver.New(
		npRecoController,
		o.config.APIServer.APIPort,
		cipherSuites,
		cipher.TLSVersionMap[o.config.APIServer.TLSMinVersion])
	if err != nil {
		return fmt.Errorf("error when creating API server: %v", err)
	}

	crdInformerFactory.Start(stopCh)
	go npRecoController.Run(stopCh)
	go apiServer.Run(stopCh)

	<-stopCh
	klog.InfoS("Stopping theia manager")
	return nil
}
