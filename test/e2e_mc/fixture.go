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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"antrea.io/theia/test/e2e"
)

func createDirectory(path string) error {
	return os.Mkdir(path, 0700)
}

func (data *MCTestData) setupLogDirectoryForTest(testName string) error {
	path := filepath.Join(testOptions.logsExportDir, testName)
	// remove directory if it already exists. This ensures that we start with an empty
	// directory
	_ = os.RemoveAll(path)
	err := createDirectory(path)
	if err != nil {
		return err
	}
	data.logsDirForTestCase = path
	return nil
}

func setupTest(tb testing.TB) (*MCTestData, error) {
	if err := testData.setupLogDirectoryForTest(tb.Name()); err != nil {
		tb.Errorf("Error creating logs directory '%s': %v", testData.logsDirForTestCase, err)
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			tb.Fail()
			exportLogs(tb, testData, "afterSetupTest", true, false)
		}
	}()
	tb.Logf("Creating '%s' K8s Namespace", multiClusterTestNamespace)
	if err := testData.createTestNamespaces(); err != nil {
		return nil, err
	}

	success = true
	return testData, nil
}

func exportLogs(tb testing.TB, data *MCTestData, logsSubDir string, writeNodeLogs bool, testMain bool) {
	for cluster, d := range data.clusterTestDataMap {
		e2e.ExportLogs(tb, d, logsSubDir+"-"+cluster, writeNodeLogs, testMain, data.controlPlaneNames[cluster])
	}
}

func teardownTest(tb testing.TB, data *MCTestData) {
	exportLogs(tb, data, "beforeTeardown", true, false)
	if empty, _ := e2e.IsDirEmpty(data.logsDirForTestCase); empty {
		_ = os.Remove(data.logsDirForTestCase)
	}
	tb.Logf("Deleting '%s' K8s Namespace", multiClusterTestNamespace)
	if err := data.deleteTestNamespaces(); err != nil {
		tb.Logf("Error when tearing down test: %v", err)
	}
}

func setupTestForMultiCluster(tb testing.TB) (*MCTestData, bool, bool, error) {
	testData, err := setupTest(tb)
	if err != nil {
		return testData, false, false, err
	}
	eastTD := testData.clusterTestDataMap[eastCluster]
	v4Enabled := eastTD.GetPodV4NetworkCIDR() != ""
	v6Enabled := eastTD.GetPodV6NetworkCIDR() != ""
	for key, td := range testData.clusterTestDataMap {
		tb.Logf("Applying changes in cluster %s", key)
		_, _, _, err = td.RunCommandOnNode(testData.controlPlaneNames[key], "kubectl taint nodes --all node-role.kubernetes.io/master:NoSchedule-")
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
		_, _, _, err = td.RunCommandOnNode(testData.controlPlaneNames[key], "kubectl taint nodes --all node.cluster.x-k8s.io/uninitialized-")
		if err != nil {
			fmt.Printf("Error: %v", err)
		}
	}
	tb.Logf("Applying flow visibility YAML in cluster east")
	eastClusterTD := testData.clusterTestDataMap[eastCluster]
	chSvcIP, nodePort, err := testData.deployClickHouse(eastClusterTD)
	if err != nil {
		return testData, v4Enabled, v6Enabled, err
	}
	nodeIP, err := eastClusterTD.GetControlPlaneNodeIP(labelNodeRoleControlPlane)
	if err != nil {
		return nil, v4Enabled, v6Enabled, fmt.Errorf("cannot get Node IP: %v", err)
	}

	tb.Logf("ClickHouse Service created with ClusterIP: %v, NodePort:%v", chSvcIP, strconv.Itoa(int(nodePort)))
	for key, clusterTestData := range testData.clusterTestDataMap {
		tb.Logf("Applying flow aggregator YAML in cluster %s", key)
		databaseURL := fmt.Sprintf("https://%s:%s", nodeIP, strconv.Itoa(int(nodePort)))
		if err := testData.deployFlowAggregator(clusterTestData, databaseURL, true); err != nil {
			return nil, v4Enabled, v6Enabled, fmt.Errorf("cannot deploy Flow Aggregator in cluster %s: %v", key, err)
		}
	}
	return testData, v4Enabled, v6Enabled, nil
}
