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

package e2e_mc

import (
	"flag"
	"log"
	"os"
	"path"
	"testing"

	"antrea.io/theia/test/e2e"
)

// setupLogging creates a temporary directory to export the test logs if necessary. If a directory
// was provided by the user, it checks that the directory exists.
func (tOptions *TestOptions) setupLogging() func() {
	if tOptions.logsExportDir == "" {
		name, err := os.MkdirTemp("", "antrea-multicluster-test-")
		if err != nil {
			log.Fatalf("Error when creating temporary directory to export logs: %v", err)
		}
		log.Printf("Test logs (if any) will be exported under the '%s' directory", name)
		tOptions.logsExportDir = name
		// we will delete the temporary directory if no logs are exported
		return func() {
			if empty, _ := e2e.IsDirEmpty(name); empty {
				log.Printf("Removing empty logs directory '%s'", name)
				_ = os.Remove(name)
			} else {
				log.Printf("Logs exported under '%s', it is your responsibility to delete the directory when you no longer need it", name)
			}
		}
	}
	fInfo, err := os.Stat(tOptions.logsExportDir)
	if err != nil {
		log.Fatalf("Cannot stat provided directory '%s': %v", tOptions.logsExportDir, err)
	}
	if !fInfo.Mode().IsDir() {
		log.Fatalf("'%s' is not a valid directory", tOptions.logsExportDir)
	}
	// no-op cleanup function
	return func() {}
}

// testMain is meant to be called by TestMain and enables the use of defer statements.
func testMain(m *testing.M) int {
	flag.StringVar(&testOptions.logsExportDir, "logs-export-dir", "", "Export directory for test logs")
	flag.StringVar(&testOptions.eastClusterKubeConfigPath, "east-cluster-kubeconfig-path", path.Join(homedir, ".kube", "east"), "Kubeconfig Path of the east cluster")
	flag.StringVar(&testOptions.westClusterKubeConfigPath, "west-cluster-kubeconfig-path", path.Join(homedir, ".kube", "west"), "Kubeconfig Path of the west cluster")
	flag.BoolVar(&testOptions.logsExportOnSuccess, "logs-export-on-success", false, "Export logs even when a test is successful")
	flag.StringVar(&testOptions.providerName, "provider", "", "K8s test cluster provider")
	flag.Parse()

	cleanupLogging := testOptions.setupLogging()
	defer cleanupLogging()

	testData = &MCTestData{}
	log.Println("Creating K8s clientsets for ClusterSet")
	if err := testData.createClients(); err != nil {
		log.Fatalf("Error when creating K8s ClientSet: %v", err)
		return 1
	}
	if err := testData.initProviders(); err != nil {
		log.Fatalf("Error when initializing providers for ClusterSet: %v", err)
	}
	if err := testData.collectPodCIDRInfo(); err != nil {
		log.Fatalf("Error when collecting PodCIDR information about K8s cluster: %v", err)
	}
	ret := m.Run()
	return ret
}

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}
