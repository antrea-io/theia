// Copyright 2023 Antrea Authors
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

package clickhouseBackupRestore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const (
	// Set the namespace and pod name
	namespace  = "flow-visibility"
	podName    = "chi-clickhouse-clickhouse-0-0-0"
	backupName = "clickhouse_backup"
)

func backupClickhouseData() {
	// Set the commands to run
	deletecmd := fmt.Sprintf("./build/linux/amd64/clickhouse-backup delete local %v", backupName)
	createcmd := fmt.Sprintf("./build/linux/amd64/clickhouse-backup create %v", backupName)
	commands := []string{
		"apt -y upgrade",
		"apt install -y wget",
		"wget https://github.com/AlexAkulov/clickhouse-backup/releases/download/v2.2.5/clickhouse-backup-linux-amd64.tar.gz",
		"tar -zxvf clickhouse-backup-linux-amd64.tar.gz",
		deletecmd,
		createcmd,
	}
	for _, cmd := range commands {
		ret, err := runCommandOnPod(cmd)
		if !strings.Contains(cmd, "delete") && err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}
}

func saveClickhouseData() {
	command := fmt.Sprintf("kubectl cp flow-visibility/chi-clickhouse-clickhouse-0-0-0:/var/lib/clickhouse/backup/%v/ %v", backupName, backupName)
	ret, err := runCommand(command)
	if err != nil {
		klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
	}
}

func deleteFlowvisibilityNamespace(yaml string) {
	deleteCmd := fmt.Sprintf("kubectl delete -f %s --ignore-not-found=true", yaml)
	commands := []string{
		"kubectl delete clickhouseinstallation.clickhouse.altinity.com clickhouse -n flow-visibility",
		deleteCmd,
	}
	for _, command := range commands {
		ret, err := runCommand(command)
		if err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}
}

func applyNewFlowvisibilityNamespace(toYaml string) {
	cmd := fmt.Sprintf("kubectl apply -f %s ", toYaml)
	ret, err := runCommand(cmd)
	if err != nil {
		klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
	}
}

func waitForClickHousePod(fromYaml, toYaml string) {
	// Build the path to the kubeconfig file.
	kubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Load the kubeconfig file and create a Kubernetes client.
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		klog.Fatal("Error building kubeconfig: ", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal("Error creating Kubernetes client: ", err)
	}

	// Define the timeout and interval for pod readiness check.
	timeout := 5 * time.Minute
	interval := 5 * time.Second

	// Wait for the pod to become ready in the specified namespace.
	err = waitForPodReady(clientset, namespace, podName, timeout, interval, fromYaml, toYaml)
	if err != nil {
		klog.Fatal("Error waiting for pod readiness: ", err)
	}

	klog.InfoS("Pod is ready!")
}

func waitForPodReady(clientset *kubernetes.Clientset, namespace, podName string, timeout, interval time.Duration, fromYaml, toYaml string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	totalInterval := time.Duration(0)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for pod %s in namespace %s to become ready", podName, namespace)
		case <-time.After(interval):
			pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				klog.InfoS("Error getting pod %s in namespace %s: %v", podName, namespace, err)
				continue
			}
			// Check if the pod is in the ready state.
			if isPodReady(pod) {
				return nil
			}
			totalInterval += interval
			if totalInterval > time.Second*60 {
				klog.InfoS("Clickhouse not ready in 60 seconds, reapplying Yaml")
				deleteFlowvisibilityNamespace(fromYaml)
				applyNewFlowvisibilityNamespace(toYaml)
				totalInterval = 0
			}
		}
	}
}

func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func recreateClickhouseDir() {
	commands := []string{
		"rm -rf /data/clickhouse",
		"mkdir -p /data/clickhouse",
	}
	for _, command := range commands {
		ret, err := runCommandOnDocker(command)
		if err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}
}

func restoreBackup() {
	// Set the commands to run
	commands := []string{
		"apt -y upgrade",
		"apt install -y wget",
		"wget https://github.com/AlexAkulov/clickhouse-backup/releases/download/v2.2.5/clickhouse-backup-linux-amd64.tar.gz",
		"tar -zxvf clickhouse-backup-linux-amd64.tar.gz",
		"./build/linux/amd64/clickhouse-backup create ",
	}
	for _, cmd := range commands {
		ret, err := runCommandOnPod(cmd)
		if !strings.Contains(cmd, "delete") && err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}

	ret, err := runCommand(fmt.Sprintf("kubectl cp %v flow-visibility/chi-clickhouse-clickhouse-0-0-0:/var/lib/clickhouse/backup/%v/ ", backupName, backupName))
	if err != nil {
		klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
	}

	ret, err = runCommandOnPod(fmt.Sprintf("./build/linux/amd64/clickhouse-backup restore %v", backupName))
	if err != nil {
		klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
	}
}

func runCommandOnPod(command string) (*exec.Cmd, error) {
	// Run the commands inside the pod
	cmd := exec.Command("kubectl", "exec", "-n", namespace, podName, "--", "bash", "-c", command)
	cmd, err := runcmd(cmd)
	if err != nil {
		return cmd, err
	}
	return cmd, nil
}

func runCommand(command string) (*exec.Cmd, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd, err := runcmd(cmd)
	if err != nil {
		return cmd, err
	}
	return cmd, nil
}

func runCommandOnDocker(command string) (*exec.Cmd, error) {
	cmd := exec.Command("docker", "exec", "kind-worker", "/bin/sh", "-c", command)
	cmd, err := runcmd(cmd)
	if err != nil {
		return cmd, err
	}
	return cmd, nil
}

func runcmd(cmd *exec.Cmd) (*exec.Cmd, error) {
	outputBuffer := &bytes.Buffer{}
	cmd.Stdout = outputBuffer
	cmd.Stderr = outputBuffer
	err := cmd.Run()
	if err != nil {
		return cmd, err
	}
	if klog.V(1).Enabled() { // Check if log level is set to debug
		klog.Infof("Command output: %s", outputBuffer.String())
	}
	return cmd, nil
}

func deleteCHBackup() {
	klog.InfoS("Deleting backup ", "clickhouse_backup", backupName)
	command := fmt.Sprintf("rm -rf %v", backupName)
	ret, err := runCommand(command)
	if err != nil {
		klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
	}
}

func storeYamllocally(fromYaml, toYaml string) {
	commands := []string{
		fmt.Sprintf("docker cp kind-control-plane:/root/%v .", fromYaml),
		fmt.Sprintf("docker cp kind-control-plane:/root/%v .", toYaml),
	}
	for _, cmd := range commands {
		ret, err := runCommand(cmd)
		if err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}
}

func deleteYamlslocally(fromYaml, toYaml string) {
	commands := []string{
		fmt.Sprintf("rm -rf %v ", fromYaml),
		fmt.Sprintf("rm -rf %v ", toYaml),
	}
	for _, cmd := range commands {
		ret, err := runCommand(cmd)
		if err != nil {
			klog.Fatal("Error during execution: stdout: %v\nstderr: %v\nerr: %v", ret.Stdout, ret.Stderr, err)
		}
	}
}

func BackupAndRestoreClickhouse(fromYaml, toYaml string, deleteBackup bool, loglevel string, deleteYamls bool) {
	if deleteBackup {
		defer deleteCHBackup()
	}
	if deleteYamls {
		storeYamllocally(fromYaml, toYaml)
		defer deleteYamlslocally(fromYaml, toYaml)
	}
	klog.InfoS("Backing up Clickhouse data")
	backupClickhouseData()
	klog.InfoS("Saving Clickhouse Data locally")
	saveClickhouseData()
	klog.InfoS("Deleting Flowvisibility Namespace")
	deleteFlowvisibilityNamespace(fromYaml)
	klog.InfoS("Waiting for graceful deletion for 30 seconds")
	time.Sleep(30 * time.Second)
	klog.InfoS("Recreating Clickhouse Directory for new PV if exists")
	recreateClickhouseDir()
	klog.InfoS("Applying new flowvisibility Yaml")
	applyNewFlowvisibilityNamespace(toYaml)
	klog.InfoS("Waiting for Clickhouse Pod to be Ready")
	waitForClickHousePod(fromYaml, toYaml)
	klog.InfoS("Restoring Data back to Clickhouse")
	restoreBackup()
}
