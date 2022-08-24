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

	"antrea.io/antrea/pkg/log"
	"antrea.io/antrea/pkg/signals"
	"antrea.io/antrea/pkg/util/cipher"
	"k8s.io/klog/v2"

	"antrea.io/theia/pkg/apiserver"
)

func run(o *Options) error {
	klog.InfoS("Theia manager starting...")
	// Set up signal capture: the first SIGTERM / SIGINT signal is handled gracefully and will
	// cause the stopCh channel to be closed; if another signal is received before the program
	// exits, we will force exit.
	stopCh := signals.RegisterSignalHandlers()

	log.StartLogFileNumberMonitor(stopCh)

	cipherSuites, err := cipher.GenerateCipherSuitesList(o.config.APIServer.TLSCipherSuites)
	if err != nil {
		return fmt.Errorf("error when generating Cipher Suite list: %v", err)
	}
	apiServer, err := apiserver.New(
		o.config.APIServer.APIPort,
		cipherSuites,
		cipher.TLSVersionMap[o.config.APIServer.TLSMinVersion])
	if err != nil {
		return fmt.Errorf("error when creating API server: %v", err)
	}
	go apiServer.Run(stopCh)

	<-stopCh
	klog.InfoS("Stopping theia manager")
	return nil
}
