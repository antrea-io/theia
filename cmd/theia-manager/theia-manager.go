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
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"antrea.io/antrea/pkg/log"
	"antrea.io/antrea/pkg/signals"
	"antrea.io/antrea/pkg/util/cipher"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"antrea.io/theia/pkg/apiserver"
	"antrea.io/theia/pkg/apiserver/certificate"
	"antrea.io/theia/pkg/apiserver/utils/stats"
	crdclientset "antrea.io/theia/pkg/client/clientset/versioned"
	crdinformers "antrea.io/theia/pkg/client/informers/externalversions"
	"antrea.io/theia/pkg/controller/anomalydetector"
	"antrea.io/theia/pkg/controller/networkpolicyrecommendation"
	"antrea.io/theia/pkg/querier"
)

// informerDefaultResync is the default resync period if a handler doesn't specify one.
// Use the same default value as kube-controller-manager:
// https://github.com/kubernetes/kubernetes/blob/release-1.17/pkg/controller/apis/config/v1alpha1/defaults.go#L120
const informerDefaultResync = 12 * time.Hour

func createAPIServerConfig(
	client kubernetes.Interface,
	kubeConfig *rest.Config,
	selfSignedCert bool,
	bindPort int,
	cipherSuites []uint16,
	tlsMinVersion uint16,
	nprq querier.NPRecommendationQuerier,
	chq querier.ClickHouseStatQuerier,
	tadq querier.ThroughputAnomalyDetectorQuerier,
) (*apiserver.Config, error) {
	secureServing := genericoptions.NewSecureServingOptions().WithLoopback()
	authentication := genericoptions.NewDelegatingAuthenticationOptions()
	authorization := genericoptions.NewDelegatingAuthorizationOptions()

	caCertController, err := certificate.ApplyServerCert(selfSignedCert, client, secureServing, apiserver.DefaultCAConfig())
	if err != nil {
		return nil, fmt.Errorf("error applying server cert: %v", err)
	}

	secureServing.BindAddress = net.IPv4zero
	secureServing.BindPort = bindPort

	authentication.WithRequestTimeout(apiserver.AuthenticationTimeout)

	serverConfig := genericapiserver.NewConfig(apiserver.Codecs)
	if err := secureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return nil, err
	}
	if err := authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, nil); err != nil {
		return nil, err
	}
	if err := authorization.ApplyTo(&serverConfig.Authorization); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(path.Dir(apiserver.TokenPath), os.ModeDir); err != nil {
		return nil, fmt.Errorf("error when creating dirs of token file: %v", err)
	}
	if err := os.WriteFile(apiserver.TokenPath, []byte(serverConfig.LoopbackClientConfig.BearerToken), 0600); err != nil {
		return nil, fmt.Errorf("error when writing loopback access token to file: %v", err)
	}

	serverConfig.SecureServing.CipherSuites = cipherSuites
	serverConfig.SecureServing.MinTLSVersion = tlsMinVersion

	return apiserver.NewConfig(
		serverConfig,
		client,
		kubeConfig,
		caCertController,
		nprq,
		chq,
		tadq), nil
}

func run(o *Options) error {
	klog.InfoS("Theia manager starting...")
	// Set up signal capture: the first SIGTERM / SIGINT signal is handled gracefully and will
	// cause the stopCh channel to be closed; if another signal is received before the program
	// exits, we will force exit.
	stopCh := signals.RegisterSignalHandlers()
	// Generate a context for functions which require one (instead of stopCh).
	// We cancel the context when the function returns, which in the normal case will be when
	// stopCh is closed.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.StartLogFileNumberMonitor(stopCh)

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("error when generating KubeConfig: %v", err)
	}
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when generating kubernetes client: %v", err)
	}
	crdClient, err := crdclientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when generating CRD client: %v", err)
	}
	crdInformerFactory := crdinformers.NewSharedInformerFactory(crdClient, informerDefaultResync)
	npRecommendationInformer := crdInformerFactory.Crd().V1alpha1().NetworkPolicyRecommendations()
	npRecoController := networkpolicyrecommendation.NewNPRecommendationController(crdClient, kubeClient, npRecommendationInformer)
	taDetectorInformer := crdInformerFactory.Crd().V1alpha1().ThroughputAnomalyDetectors()
	taDetectorController := anomalydetector.NewAnomalyDetectorController(crdClient, kubeClient, taDetectorInformer)
	clickHouseStatQuerierImpl := stats.NewClickHouseStatQuerierImpl(kubeClient)

	cipherSuites, err := cipher.GenerateCipherSuitesList(o.config.APIServer.TLSCipherSuites)
	if err != nil {
		return fmt.Errorf("error when generating Cipher Suite list: %v", err)
	}

	apiServerConfig, err := createAPIServerConfig(
		kubeClient,
		kubeConfig,
		*o.config.APIServer.SelfSignedCert,
		o.config.APIServer.APIPort,
		cipherSuites,
		cipher.TLSVersionMap[o.config.APIServer.TLSMinVersion],
		npRecoController,
		clickHouseStatQuerierImpl,
		taDetectorController)
	if err != nil {
		return fmt.Errorf("error creating API server config: %v", err)
	}
	apiServer, err := apiServerConfig.New()
	if err != nil {
		return fmt.Errorf("error when creating API server: %v", err)
	}

	crdInformerFactory.Start(stopCh)
	go npRecoController.Run(stopCh)
	go taDetectorController.Run(stopCh)
	go apiServer.Run(ctx)

	<-stopCh
	klog.InfoS("Stopping theia manager")
	return nil
}
