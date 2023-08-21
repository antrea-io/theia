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
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	"antrea.io/theia/test/e2e"
)

var (
	homedir, _        = os.UserHomeDir()
	clickHousePodName = fmt.Sprintf("%s-0-0-0", clickHousePodNamePrefix)
)

const (
	iperfPort              int32  = 5201
	flowAggregatorCHSecret string = "clickhouse-ca"

	perftoolImage = "projects.registry.vmware.com/antrea/perftool"

	defaultTimeout                   = 90 * time.Second
	defaultInterval                  = 1 * time.Second
	labelNodeRoleControlPlane string = "node-role.kubernetes.io/control-plane"
	flowAggregatorNamespace   string = "flow-aggregator"
	flowVisibilityNamespace   string = "flow-visibility"
	clickHouseHTTPPort        string = "8123"

	clickHouseOperatorYML   string = "clickhouse-operator-install-bundle.yaml"
	clickHousePodNamePrefix string = "chi-clickhouse-clickhouse"
	flowAggregatorYML       string = "flow-aggregator.yml"

	flowVisibilityChOnlyYML  string = "flow-visibility-ch-only.yml"
	flowAggregatorDeployment string = "flow-aggregator"

	// #nosec G101: false positive triggered by variable name which includes "test"
	multiClusterTestNamespace string = "antrea-multicluster-test"
	eastCluster               string = "east-cluster"
	westCluster               string = "west-cluster"
)

type TestOptions struct {
	westClusterKubeConfigPath string
	eastClusterKubeConfigPath string
	providerName              string
	logsExportDir             string
	logsExportOnSuccess       bool
}

var testOptions TestOptions

type MCTestData struct {
	clusters           []string
	clusterTestDataMap map[string]*e2e.TestData
	controlPlaneNames  map[string]string
	logsDirForTestCase string
}

var testData *MCTestData

func (data *MCTestData) createClients() error {
	kubeConfigPaths := []string{
		testOptions.eastClusterKubeConfigPath,
		testOptions.westClusterKubeConfigPath,
	}
	data.clusters = []string{
		eastCluster, westCluster,
	}
	data.clusterTestDataMap = map[string]*e2e.TestData{}
	for i, cluster := range data.clusters {
		testData := e2e.TestData{}
		testData.SetClusterName(cluster)
		if err := testData.CreateClient(kubeConfigPaths[i]); err != nil {
			return fmt.Errorf("error initializing clients for cluster %s: %v", cluster, err)
		}
		data.clusterTestDataMap[cluster] = &testData
	}
	data.controlPlaneNames = map[string]string{
		"east-cluster": "east-control-plane",
		"west-cluster": "west-control-plane",
	}
	return nil
}

func (data *MCTestData) initProviders() error {
	providerName := "remote"
	if testOptions.providerName == "kind" {
		providerName = testOptions.providerName
	}
	for cluster, d := range data.clusterTestDataMap {
		if err := d.InitProvider(providerName, ""); err != nil {
			log.Errorf("Failed to initialize provider for cluster %s", cluster)
			return err
		}
	}
	return nil
}

func (data *MCTestData) createTestNamespaces() error {
	for cluster, d := range data.clusterTestDataMap {
		if err := d.CreateNamespace(multiClusterTestNamespace, nil); err != nil {
			log.Errorf("Failed to create Namespace %s in cluster %s", multiClusterTestNamespace, cluster)
			return err
		}
	}
	return nil
}

func (data *MCTestData) deleteTestNamespaces() error {
	for cluster, d := range data.clusterTestDataMap {
		if err := d.DeleteNamespace(multiClusterTestNamespace, defaultTimeout); err != nil {
			log.Errorf("Failed to delete Namespace %s in cluster %s", multiClusterTestNamespace, cluster)
			return err
		}
	}
	return nil
}

func (data *MCTestData) podWaitFor(timeout time.Duration, clusterName, name, namespace string, condition e2e.PodCondition) (*corev1.Pod, error) {
	if d, ok := data.clusterTestDataMap[clusterName]; ok {
		return d.PodWaitFor(timeout, name, namespace, condition)
	}
	return nil, fmt.Errorf("clusterName %s not found", clusterName)
}

func (data *MCTestData) podWaitForIPs(timeout time.Duration, clusterName, name, namespace string) (*e2e.PodIPs, error) {
	var clusterTD *e2e.TestData
	if cluster, ok := data.clusterTestDataMap[clusterName]; !ok {
		return nil, fmt.Errorf("cluster %s doesn't exist in clusterTestDataMap", clusterName)
	} else {
		clusterTD = cluster
	}
	pod, err := data.podWaitFor(timeout, clusterName, name, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == corev1.PodRunning, nil
	})
	if err != nil {
		return nil, err
	}
	// According to the K8s API documentation (https://godoc.org/k8s.io/api/core/v1#PodStatus),
	// the PodIP field should only be empty if the Pod has not yet been scheduled, and "running"
	// implies scheduled.
	if pod.Status.PodIP == "" {
		return nil, fmt.Errorf("pod is running but has no assigned IP, which should never happen")
	}
	podIPStrings := sets.New[string](pod.Status.PodIP)
	for _, podIP := range pod.Status.PodIPs {
		ipStr := strings.TrimSpace(podIP.IP)
		if ipStr != "" {
			podIPStrings.Insert(ipStr)
		}
	}
	ips, err := e2e.ParsePodIPs(podIPStrings)
	if err != nil {
		return nil, err
	}
	if !pod.Spec.HostNetwork {
		if clusterTD.GetPodV4NetworkCIDR() != "" && ips.GetIPv4() == nil {
			return nil, fmt.Errorf("no IPv4 address is assigned while cluster was configured with IPv4 Pod CIDR %s", clusterTD.GetPodV4NetworkCIDR())
		}
		if clusterTD.GetPodV6NetworkCIDR() != "" && ips.GetIPv6() == nil {
			return nil, fmt.Errorf("no IPv6 address is assigned while cluster was configured with IPv6 Pod CIDR %s", clusterTD.GetPodV6NetworkCIDR())
		}
	}
	return ips, nil
}

func (data *MCTestData) deployClickHouse(td *e2e.TestData) (string, int32, error) {
	err := data.deployFlowVisibilityCommon(td, clickHouseOperatorYML, flowVisibilityChOnlyYML)
	if err != nil {
		return "", 0, err
	}
	// check for ClickHouse Pod ready. Wait for 2x timeout as ch operator needs to be running first to handle chi
	if err = td.PodWaitForReady(2*defaultTimeout, clickHousePodName, flowVisibilityNamespace); err != nil {
		return "", 0, err
	}
	var chSvc *corev1.Service
	if err := wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		chSvc, err = td.GetService(flowVisibilityNamespace, "clickhouse-clickhouse")
		if err != nil {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		return "", 0, fmt.Errorf("timeout waiting for ClickHouse Service: %v", err)
	}
	// check ClickHouse Service http port for Service connectivity
	if err := wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		rc, _, _, err := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()],
			fmt.Sprintf("curl -Ss %s:%s", chSvc.Spec.ClusterIP, clickHouseHTTPPort))
		if rc != 0 || err != nil {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		return "", 0, fmt.Errorf("timeout checking http port connectivity of clickhouse service: %v", err)
	}
	for _, port := range chSvc.Spec.Ports {
		if port.Name == "https" {
			if port.NodePort != 0 {
				return chSvc.Spec.ClusterIP, port.NodePort, nil
			}
		}
	}
	return "", 0, fmt.Errorf("ClickHouse service doesn't contain http NodePort: %v", err)
}

// deployFlowAggregator deploys the Flow Aggregator with clickHouse enabled.
func (data *MCTestData) deployFlowAggregator(td *e2e.TestData, databaseURL string, security bool) error {

	flowAggYaml := flowAggregatorYML
	rc, _, _, err := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl apply -f %s", flowAggYaml))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deploying the Flow Aggregator; %s not available on the control-plane Node", flowAggYaml)
	}
	// clickhouse-ca Secret is created in the flow-visibility Namespace. In order to make it accessible to the Flow Aggregator,
	// we copy it from Namespace flow-visibility to Namespace flow-aggregator when secureConnection is true.
	if security {
		secret, err := data.clusterTestDataMap[eastCluster].GetClientSet().CoreV1().Secrets(flowVisibilityNamespace).Get(context.TODO(), flowAggregatorCHSecret, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("unable to get Secret with name %s in Namespace %s: %v", flowAggregatorCHSecret, flowVisibilityNamespace, err)
		}
		newSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: flowAggregatorNamespace,
				Name:      flowAggregatorCHSecret,
			},
			Data: secret.Data,
		}
		_, err = td.GetClientSet().CoreV1().Secrets(flowAggregatorNamespace).Create(context.TODO(), newSecret, metav1.CreateOptions{})
		if errors.IsAlreadyExists(err) {
			_, err = td.GetClientSet().CoreV1().Secrets(flowAggregatorNamespace).Update(context.TODO(), newSecret, metav1.UpdateOptions{})
		}
		if err != nil {
			return fmt.Errorf("unable to copy ClickHouse CA secret '%s' from Namespace '%s' to Namespace '%s': %v", flowAggregatorCHSecret, flowVisibilityNamespace, flowAggregatorNamespace, err)
		}
	}
	if err = td.MutateFlowAggregatorConfigMap(databaseURL, security); err != nil {
		return err
	}
	if rc, _, _, err = td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl -n %s rollout status deployment/%s --timeout=%v", flowAggregatorNamespace, flowAggregatorDeployment, 2*defaultTimeout)); err != nil || rc != 0 {
		_, stdout, _, _ := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl -n %s describe pod", flowAggregatorNamespace))
		_, logStdout, _, _ := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl -n %s logs -l app=flow-aggregator", flowAggregatorNamespace))
		return fmt.Errorf("error when waiting for the Flow Aggregator rollout to complete. kubectl describe output: %s, logs: %s", stdout, logStdout)
	}
	// Check for flow-aggregator Pod running again for db connection establishment
	flowAggPod, err := td.GetFlowAggregator()
	if err != nil {
		return fmt.Errorf("error when getting flow-aggregator Pod: %v", err)
	}
	if err = td.PodWaitForReady(2*defaultTimeout, flowAggPod.Name, flowAggregatorNamespace); err != nil {
		return err
	}
	return nil
}

func (data *MCTestData) deployFlowVisibilityCommon(td *e2e.TestData, chOperatorYML, flowVisibilityYML string) error {
	rc, _, _, err := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl apply -f %s", chOperatorYML))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deploying the ClickHouse Operator YML: %v\n is %s available on the control-plane Node?", err, chOperatorYML)
	}
	if err := wait.Poll(2*time.Second, 10*time.Second, func() (bool, error) {
		rc, stdout, stderr, err := td.RunCommandOnNode(data.controlPlaneNames[td.GetClusterName()], fmt.Sprintf("kubectl apply -f %s", flowVisibilityYML))
		if err != nil || rc != 0 {
			log.Infof("error when deploying the flow visibility YML %s: %s, %s, %v", flowVisibilityYML, stdout, stderr, err)
			// ClickHouseInstallation CRD from ClickHouse Operator install bundle applied soon before
			// applying CR. Sometimes apiserver validation fails to recognize resource of
			// kind: ClickHouseInstallation. Retry in such scenario.
			if strings.Contains(stderr, "ClickHouseInstallation") || strings.Contains(stdout, "ClickHouseInstallation") {
				return false, nil
			}
			return false, fmt.Errorf("error when deploying the flow visibility YML %s: %s, %s, %v", flowVisibilityYML, stdout, stderr, err)
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// createPodOnNode creates a pod in the test namespace with a container whose type is decided by imageName.
// Pod will be scheduled on the specified Node (if nodeName is not empty).
// mutateFunc can be used to customize the Pod if the other parameters don't meet the requirements.
func (data *MCTestData) createPodOnNode(td *e2e.TestData, name string, ns string, nodeName string, image string, command []string, args []string, env []corev1.EnvVar, ports []corev1.ContainerPort, hostNetwork bool, mutateFunc func(*corev1.Pod)) error {
	// image could be a fully qualified URI which can't be used as container name and label value,
	// extract the image name from it.
	imageName := e2e.GetImageName(image)
	return data.CreatePodOnNodeInNamespace(td, name, ns, nodeName, imageName, image, command, args, env, ports, hostNetwork, mutateFunc)
}

// CreatePodOnNodeInNamespace creates a pod in the provided namespace with a container whose type is decided by imageName.
// Pod will be scheduled on the specified Node (if nodeName is not empty).
// mutateFunc can be used to customize the Pod if the other parameters don't meet the requirements.
func (data *MCTestData) CreatePodOnNodeInNamespace(td *e2e.TestData, name, ns string, nodeName, ctrName string, image string, command []string, args []string, env []corev1.EnvVar, ports []corev1.ContainerPort, hostNetwork bool, mutateFunc func(*corev1.Pod)) error {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:            ctrName,
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         command,
				Args:            args,
				Env:             env,
				Ports:           ports,
			},
		},
		RestartPolicy: corev1.RestartPolicyNever,
		HostNetwork:   hostNetwork,
	}
	if nodeName != "" {
		podSpec.NodeSelector = map[string]string{
			"kubernetes.io/hostname": nodeName,
		}
	}
	if nodeName == data.controlPlaneNames[td.GetClusterName()] {
		// tolerate NoSchedule taint if we want Pod to run on control-plane Node
		podSpec.Tolerations = e2e.ControlPlaneNoScheduleTolerations()
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"antrea-e2e": name,
				"app":        ctrName,
			},
		},
		Spec: podSpec,
	}
	if mutateFunc != nil {
		mutateFunc(pod)
	}
	if _, err := td.GetClientSet().CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func flowVisibilityCleanup(tb testing.TB, data *MCTestData) {
	teardownTest(tb, data)
	tb.Logf("Cleaning Flow Visibility in Cluster east")
	e2e.TeardownFlowVisibility(tb, data.clusterTestDataMap[eastCluster], e2e.CreateFlowVisibilitySetUpConfig(false, false, false, true, false), data.controlPlaneNames[eastCluster])
	tb.Logf("Cleaning Flow Visibility in Cluster west")
	e2e.TeardownFlowVisibility(tb, data.clusterTestDataMap[westCluster], e2e.CreateFlowVisibilitySetUpConfig(false, false, false, true, true), data.controlPlaneNames[westCluster])
}

func (data *MCTestData) collectPodCIDRInfo() error {
	for cluster, d := range data.clusterTestDataMap {
		if err := d.CollectPodCIDRInfo(data.controlPlaneNames[cluster]); err != nil {
			log.Errorf("Failed to collect PodCIRD Info in cluster %s", cluster)
			return err
		}
	}
	return nil
}
