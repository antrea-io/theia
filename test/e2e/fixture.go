// Copyright 2022 Antrea Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ensureAntreaRunning(data *TestData) error {
	log.Println("Applying Antrea YAML")
	if err := data.deployAntrea(); err != nil {
		return err
	}
	log.Println("Waiting for all Antrea DaemonSet Pods")
	if err := data.waitForAntreaDaemonSetPods(defaultTimeout); err != nil {
		return err
	}
	log.Println("Checking CoreDNS deployment")
	if err := data.checkCoreDNSPods(defaultTimeout); err != nil {
		return err
	}
	return nil
}

func teardownTest(tb testing.TB, data *TestData) {
	exportLogs(tb, data, "beforeTeardown", true, false)
	if empty, _ := IsDirEmpty(data.logsDirForTestCase); empty {
		_ = os.Remove(data.logsDirForTestCase)
	}
	tb.Logf("Deleting '%s' K8s Namespace", testNamespace)
	if err := data.deleteTestNamespace(defaultTimeout); err != nil {
		tb.Logf("Error when tearing down test: %v", err)
	}
}

func createAndWaitForPod(t *testing.T, data *TestData, createFunc func(name string, ns string, nodeName string, hostNetwork bool) error, namePrefix string, nodeName string, ns string, hostNetwork bool) (string, *PodIPs, func()) {
	name := randName(namePrefix)
	if err := createFunc(name, ns, nodeName, hostNetwork); err != nil {
		t.Fatalf("Error when creating busybox test Pod: %v", err)
	}
	cleanupFunc := func() {
		deletePodWrapper(t, data, ns, name)
	}
	podIP, err := data.podWaitForIPs(defaultTimeout, name, ns)
	if err != nil {
		cleanupFunc()
		t.Fatalf("Error when waiting for IP for Pod '%s': %v", name, err)
	}
	return name, podIP, cleanupFunc
}

func deletePodWrapper(tb testing.TB, data *TestData, namespace, name string) {
	tb.Logf("Deleting Pod '%s'", name)
	if err := data.DeletePod(namespace, name); err != nil {
		tb.Logf("Error when deleting Pod: %v", err)
	}
}

func (data *TestData) setupLogDirectoryForTest(testName string) error {
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

// forAllMatchingPodsInNamespace invokes the provided function for every Pod currently running on every Node in a given
// namespace and which matches labelSelector criteria.
func (data *TestData) forAllMatchingPodsInNamespace(
	labelSelector, nsName string, fn func(nodeName string, podName string, nsName string) error) error {
	for _, node := range clusterInfo.nodes {
		listOptions := metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", node.name),
		}
		pods, err := data.clientset.CoreV1().Pods(nsName).List(context.TODO(), listOptions)
		if err != nil {
			return fmt.Errorf("failed to list Antrea Pods on Node '%s': %v", node.name, err)
		}
		for _, pod := range pods.Items {
			if err := fn(node.name, pod.Name, nsName); err != nil {
				return err
			}
		}
	}
	return nil
}

func forAllNodes(fn func(nodeName string) error) error {
	for idx := 0; idx < clusterInfo.numNodes; idx++ {
		name := nodeName(idx)
		if name == "" {
			return fmt.Errorf("unexpected empty name for Node %d", idx)
		}
		if err := fn(name); err != nil {
			return err
		}
	}
	return nil
}

func exportLogs(tb testing.TB, data *TestData, logsSubDir string, writeNodeLogs bool, testMain bool) {
	if !testMain {
		if tb.Skipped() {
			return
		}
		// if test was successful and --logs-export-on-success was not provided, we do not export
		// any logs.
		if !tb.Failed() && !testOptions.logsExportOnSuccess {
			return
		}
	}
	const timeFormat = "Jan02-15-04-05"
	timeStamp := time.Now().Format(timeFormat)
	logsDir := filepath.Join(data.logsDirForTestCase, fmt.Sprintf("%s.%s", logsSubDir, timeStamp))
	err := createDirectory(logsDir)
	if err != nil {
		log.Printf("Error when creating logs directory '%s': %v", logsDir, err)
		return
	}
	log.Printf("Exporting test logs to '%s'", logsDir)
	// for now we just retrieve the logs for the Antrea Pods, but maybe we can find a good way to
	// retrieve the logs for the test Pods in the future (before deleting them) if it is useful
	// for debugging.

	// getPodWriter creates the file with name nodeName-podName-suffix. It returns nil if the
	// file cannot be created. File must be closed by the caller.
	getPodWriter := func(nodeName, podName, suffix string) *os.File {
		logFile := filepath.Join(logsDir, fmt.Sprintf("%s-%s-%s", nodeName, podName, suffix))
		f, err := os.Create(logFile)
		if err != nil {
			log.Printf("Error when creating log file '%s': '%v'", logFile, err)
			return nil
		}
		return f
	}

	// runKubectl runs the provided kubectl command on the control-plane Node and returns the
	// output. It returns an empty string in case of error.
	runKubectl := func(cmd string) string {
		rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if err != nil || rc != 0 {
			log.Printf("Error when running kubectl command on control-plane Node, cmd:%s\nstdout:%s\nstderr:%s", cmd, stdout, stderr)
			return ""
		}
		return stdout
	}

	// dump the logs for Antrea Pods to disk.
	writePodLogs := func(nodeName, podName, nsName string) error {
		w := getPodWriter(nodeName, podName, "logs")
		if w == nil {
			return nil
		}
		defer w.Close()
		cmd := fmt.Sprintf("kubectl -n %s logs --all-containers %s", nsName, podName)
		stdout := runKubectl(cmd)
		if stdout == "" {
			return nil
		}
		w.WriteString(stdout)
		return nil
	}
	data.forAllMatchingPodsInNamespace("k8s-app=kube-proxy", kubeNamespace, writePodLogs)

	data.forAllMatchingPodsInNamespace("app=antrea", antreaNamespace, writePodLogs)

	if !testMain {
		// dump the logs for flow-aggregator Pods to disk.
		data.forAllMatchingPodsInNamespace("", flowAggregatorNamespace, writePodLogs)

		// dump the logs for flow-visibility Pods to disk.
		data.forAllMatchingPodsInNamespace("", flowVisibilityNamespace, writePodLogs)

		// dump the logs for clickhouse operator Pods to disk.
		data.forAllMatchingPodsInNamespace("app=clickhouse-operator", kubeNamespace, writePodLogs)
	}

	// dump the output of "kubectl describe" for Antrea pods to disk.
	data.forAllMatchingPodsInNamespace("app=antrea", antreaNamespace, func(nodeName, podName, nsName string) error {
		w := getPodWriter(nodeName, podName, "describe")
		if w == nil {
			return nil
		}
		defer w.Close()
		cmd := fmt.Sprintf("kubectl -n %s describe pod %s", nsName, podName)
		stdout := runKubectl(cmd)
		if stdout == "" {
			return nil
		}
		w.WriteString(stdout)
		return nil
	})

	if !writeNodeLogs {
		return
	}
	// getNodeWriter creates the file with name nodeName-suffix. It returns nil if the file
	// cannot be created. File must be closed by the caller.
	getNodeWriter := func(nodeName, suffix string) *os.File {
		logFile := filepath.Join(logsDir, fmt.Sprintf("%s-%s", nodeName, suffix))
		f, err := os.Create(logFile)
		if err != nil {
			log.Printf("Error when creating log file '%s': '%v'", logFile, err)
			return nil
		}
		return f
	}
	// export kubelet logs with journalctl for each Node. If the Nodes do not use journalctl we
	// print a log message. If kubelet is not running with systemd, the log file will be empty.
	if err := forAllNodes(func(nodeName string) error {
		const numLines = 100
		// --no-pager ensures the command does not hang.
		cmd := fmt.Sprintf("journalctl -u kubelet -n %d --no-pager", numLines)
		if clusterInfo.nodesOS[nodeName] == "windows" {
			cmd = "Get-EventLog -LogName \"System\" -Source \"Service Control Manager\" | grep kubelet ; Get-EventLog -LogName \"Application\" -Source \"nssm\" | grep kubelet"
		}
		rc, stdout, _, err := data.RunCommandOnNode(nodeName, cmd)
		if err != nil || rc != 0 {
			// return an error and skip subsequent Nodes
			return fmt.Errorf("error when running journalctl on Node '%s', is it available? Error: %v", nodeName, err)
		}
		w := getNodeWriter(nodeName, "kubelet")
		if w == nil {
			// move on to the next Node
			return nil
		}
		defer w.Close()
		w.WriteString(stdout)
		return nil
	}); err != nil {
		log.Printf("Error when exporting kubelet logs: %v", err)
	}
}

func setupTest(tb testing.TB) (*TestData, error) {
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
	tb.Logf("Creating '%s' K8s Namespace", testNamespace)
	if err := ensureAntreaRunning(testData); err != nil {
		return nil, err
	}
	if err := testData.createTestNamespace(); err != nil {
		return nil, err
	}
	success = true
	return testData, nil
}

func setupTestForFlowVisibility(tb testing.TB, config FlowVisibiltiySetUpConfig) (*TestData, bool, bool, error) {
	v4Enabled := clusterInfo.podV4NetworkCIDR != ""
	v6Enabled := clusterInfo.podV6NetworkCIDR != ""
	testData, err := setupTest(tb)
	if err != nil {
		return testData, v4Enabled, v6Enabled, err
	}

	tb.Logf("Applying flow visibility YAML")
	chSvcIP, err := testData.deployFlowVisibility(config)
	if err != nil {
		return testData, v4Enabled, v6Enabled, err
	}
	tb.Logf("ClickHouse Service created with ClusterIP: %v", chSvcIP)
	if config.withFlowAggregator {
		tb.Logf("Applying flow aggregator YAML")
		if err := testData.deployFlowAggregator(); err != nil {
			return testData, v4Enabled, v6Enabled, err
		}
	}
	return testData, v4Enabled, v6Enabled, nil
}
