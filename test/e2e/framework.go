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
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/semver"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/retry"
	aggregatorclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	utilnet "k8s.io/utils/net"

	"antrea.io/antrea/pkg/agent/openflow"
	crdv1alpha1 "antrea.io/antrea/pkg/apis/crd/v1alpha1"
	crdclientset "antrea.io/antrea/pkg/client/clientset/versioned"

	"antrea.io/theia/pkg/theia/commands"
	"antrea.io/theia/pkg/theia/commands/config"
	"antrea.io/theia/pkg/theia/portforwarder"
	"antrea.io/theia/pkg/util/clickhouse"
	"antrea.io/theia/pkg/util/k8s"
	"antrea.io/theia/test/e2e/providers"
)

var (
	errConnectionLost = fmt.Errorf("http2: client connection lost")
)

const (
	defaultTimeout  = 90 * time.Second
	defaultInterval = 1 * time.Second
	realizeTimeout  = 5 * time.Minute

	antreaNamespace            string = "kube-system"
	kubeNamespace              string = "kube-system"
	flowAggregatorNamespace    string = "flow-aggregator"
	flowVisibilityNamespace    string = "flow-visibility"
	testNamespace              string = "antrea-test"
	iperfPort                  int32  = 5201
	clickHouseHTTPPort         string = "8123"
	busyboxContainerName       string = "busybox"
	defaultBridgeName          string = "br-int"
	antreaYML                  string = "antrea.yml"
	antreaDaemonSet            string = "antrea-agent"
	antreaDeployment           string = "antrea-controller"
	flowAggregatorDeployment   string = "flow-aggregator"
	flowAggregatorYML          string = "flow-aggregator.yml"
	flowVisibilityYML          string = "flow-visibility.yml"
	flowVisibilityWithSparkYML string = "flow-visibility-with-spark.yml"
	flowVisibilityChOnlyYML    string = "flow-visibility-ch-only.yml"
	clickHouseOperatorYML      string = "clickhouse-operator-install-bundle.yaml"
	clickHousePodNamePrefix    string = "chi-clickhouse-clickhouse"
	policyOutputYML            string = "output.yaml"
	sparkOperatorPodLabel      string = "app.kubernetes.io/name=spark-operator"
	grafanaPodLabel            string = "app=grafana"
	theiaManagerPodLabel       string = "app=theia-manager"
	antreaPodLabel             string = "app=antrea,component=antrea-agent"
	clickHouseLocalPvLabel     string = "antrea.io/clickhouse-data-node"
	clickHouseLocalPvPath      string = "/data/clickhouse"

	agnhostImage  = "registry.k8s.io/e2e-test-images/agnhost:2.29"
	busyboxImage  = "projects.registry.vmware.com/antrea/busybox"
	perftoolImage = "projects.registry.vmware.com/antrea/perftool"

	exporterActiveFlowExportTimeout    = 2 * time.Second
	aggregatorActiveFlowRecordTimeout  = 3500 * time.Millisecond
	aggregatorClickHouseCommitInterval = 1 * time.Second

	insertQueryflowtable = `INSERT INTO flows (
		flowStartSeconds,
		flowEndSeconds,
		flowEndSecondsFromSourceNode,
		flowEndSecondsFromDestinationNode,
		flowEndReason,
		sourceIP,
		destinationIP,
		sourceTransportPort,
		destinationTransportPort,
		protocolIdentifier,
		packetTotalCount,
		octetTotalCount,
		packetDeltaCount,
		octetDeltaCount,
		reversePacketTotalCount,
		reverseOctetTotalCount,
		reversePacketDeltaCount,
		reverseOctetDeltaCount,
		sourcePodName,
		sourcePodNamespace,
		sourceNodeName,
		destinationPodName,
		destinationPodNamespace,
		destinationNodeName,
		destinationClusterIP,
		destinationServicePort,
		destinationServicePortName,
		ingressNetworkPolicyName,
		ingressNetworkPolicyNamespace,
		ingressNetworkPolicyRuleName,
		ingressNetworkPolicyRuleAction,
		ingressNetworkPolicyType,
		egressNetworkPolicyName,
		egressNetworkPolicyNamespace,
		egressNetworkPolicyRuleName,
		egressNetworkPolicyRuleAction,
		egressNetworkPolicyType,
		tcpState,
		flowType,
		sourcePodLabels,
		destinationPodLabels,
		throughput,
		reverseThroughput,
		throughputFromSourceNode,
		throughputFromDestinationNode,
		reverseThroughputFromSourceNode,
		reverseThroughputFromDestinationNode,
		clusterUUID)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
				?, ?, ?, ?, ?, ?, ?, ?)`
)

type FlowVisibiltiySetUpConfig struct {
	withSparkOperator     bool
	withGrafana           bool
	withClickHouseLocalPv bool
	withFlowAggregator    bool
}

type ClusterNode struct {
	idx              int // 0 for control-plane Node
	name             string
	ipv4Addr         string
	ipv6Addr         string
	podV4NetworkCIDR string
	podV6NetworkCIDR string
	gwV4Addr         string
	gwV6Addr         string
	os               string
}

type ClusterInfo struct {
	numNodes             int
	podV4NetworkCIDR     string
	podV6NetworkCIDR     string
	svcV4NetworkCIDR     string
	svcV6NetworkCIDR     string
	controlPlaneNodeName string
	controlPlaneNodeIPv4 string
	controlPlaneNodeIPv6 string
	nodes                map[int]ClusterNode
	nodesOS              map[string]string
	windowsNodes         []int
	k8sServerVersion     string
	k8sServiceHost       string
	k8sServicePort       int32
}

var clusterInfo ClusterInfo

// TestData stores the state required for each test case.
type TestData struct {
	provider           providers.ProviderInterface
	kubeConfig         *restclient.Config
	clientset          kubernetes.Interface
	aggregatorClient   aggregatorclientset.Interface
	crdClient          crdclientset.Interface
	logsDirForTestCase string
}

var testData *TestData

type TestOptions struct {
	providerName        string
	providerConfigPath  string
	logsExportDir       string
	logsExportOnSuccess bool
	skipCases           string
}

var testOptions TestOptions

type PodIPs struct {
	ipv4      *net.IP
	ipv6      *net.IP
	ipStrings []string
}

func (p PodIPs) String() string {
	res := ""
	if p.ipv4 != nil {
		res += fmt.Sprintf("IPv4(%s),", p.ipv4.String())
	}
	if p.ipv6 != nil {
		res += fmt.Sprintf("IPv6(%s),", p.ipv6.String())
	}
	return fmt.Sprintf("%sIPstrings(%s)", res, strings.Join(p.ipStrings, ","))
}

// deployAntreaCommon deploys Antrea using kubectl on the control-plane Node.
func (data *TestData) deployAntreaCommon(yamlFile string, extraOptions string, waitForAgentRollout bool) error {
	// TODO: use the K8s apiserver when server side apply is available?
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#server-side-apply
	rc, _, _, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl apply %s -f %s", extraOptions, yamlFile))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deploying Antrea; is %s available on the control-plane Node?", yamlFile)
	}
	rc, stdout, stderr, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s rollout status deploy/%s --timeout=%v", antreaNamespace, antreaDeployment, defaultTimeout))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when waiting for antrea-controller rollout to complete - rc: %v - stdout: %v - stderr: %v - err: %v", rc, stdout, stderr, err)
	}
	if waitForAgentRollout {
		rc, stdout, stderr, err = data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s rollout status ds/%s --timeout=%v", antreaNamespace, antreaDaemonSet, defaultTimeout))
		if err != nil || rc != 0 {
			return fmt.Errorf("error when waiting for antrea-agent rollout to complete - rc: %v - stdout: %v - stderr: %v - err: %v", rc, stdout, stderr, err)
		}
	}

	return nil
}

// deployAntrea deploys Antrea.
func (data *TestData) deployAntrea() error {
	return data.deployAntreaCommon(antreaYML, "", true)
}

// waitForAntreaDaemonSetPods waits for the K8s apiserver to report that all the Antrea Pods are
// available, i.e. all the Nodes have one or more of the Antrea daemon Pod running and available.
func (data *TestData) waitForAntreaDaemonSetPods(timeout time.Duration) error {
	err := wait.Poll(defaultInterval, timeout, func() (bool, error) {
		getDS := func(dsName string, os string) (*appsv1.DaemonSet, error) {
			ds, err := data.clientset.AppsV1().DaemonSets(antreaNamespace).Get(context.TODO(), dsName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("error when getting Antrea %s daemonset: %v", os, err)
			}
			return ds, nil
		}
		var dsLinux *appsv1.DaemonSet
		var err error
		if dsLinux, err = getDS(antreaDaemonSet, "Linux"); err != nil {
			return false, err
		}
		currentNumAvailable := dsLinux.Status.NumberAvailable
		UpdatedNumberScheduled := dsLinux.Status.UpdatedNumberScheduled

		// Make sure that all Daemon Pods are available.
		// We use clusterInfo.numNodes instead of DesiredNumberScheduled because
		// DesiredNumberScheduled may not be updated right away. If it is still set to 0 the
		// first time we get the DaemonSet's Status, we would return immediately instead of
		// waiting.
		desiredNumber := int32(clusterInfo.numNodes)
		if currentNumAvailable != desiredNumber || UpdatedNumberScheduled != desiredNumber {
			return false, nil
		}

		// Make sure that all antrea-agent Pods are not terminating. This is required because NumberAvailable of
		// DaemonSet counts Pods even if they are terminating. Deleting antrea-agent Pods directly does not cause the
		// number to decrease if the process doesn't quit immediately, e.g. when the signal is caught by bincover
		// program and triggers coverage calculation.
		pods, err := data.clientset.CoreV1().Pods(antreaNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: antreaPodLabel,
		})
		if err != nil {
			return false, fmt.Errorf("failed to list antrea-agent Pods: %v", err)
		}
		if len(pods.Items) != clusterInfo.numNodes {
			return false, nil
		}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				return false, nil
			}
		}
		return true, nil
	})
	if err == wait.ErrWaitTimeout {
		_, stdout, _, _ := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s describe pod", antreaNamespace))
		return fmt.Errorf("antrea-agent DaemonSet not ready within %v; kubectl describe pod output: %v", defaultTimeout, stdout)
	} else if err != nil {
		return err
	}

	return nil
}

func isConnectionLostError(err error) bool {
	return strings.Contains(err.Error(), errConnectionLost.Error())
}

// retryOnConnectionLostError allows the caller to retry fn in case the error is ConnectionLost.
// e2e script might get ConnectionLost error when accessing k8s apiserver if AntreaIPAM is enabled and antrea-agent is restarted.
func retryOnConnectionLostError(backoff wait.Backoff, fn func() error) error {
	return retry.OnError(backoff, isConnectionLostError, fn)
}

// waitForCoreDNSPods waits for the K8s apiserver to report that all the CoreDNS Pods are available.
func (data *TestData) waitForCoreDNSPods(timeout time.Duration) error {
	err := wait.PollImmediate(defaultInterval, timeout, func() (bool, error) {
		deployment, err := data.clientset.AppsV1().Deployments("kube-system").Get(context.TODO(), "coredns", metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error when retrieving CoreDNS deployment: %v", err)
		}
		if deployment.Status.UnavailableReplicas == 0 {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("some CoreDNS replicas are still unavailable after %v", defaultTimeout)
	} else if err != nil {
		return err
	}
	return nil
}

// restartCoreDNSPods deletes all the CoreDNS Pods to force them to be re-scheduled. It then waits
// for all the Pods to become available, by calling waitForCoreDNSPods.
func (data *TestData) restartCoreDNSPods(timeout time.Duration) error {
	var gracePeriodSeconds int64 = 1
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	}
	listOptions := metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	}
	if err := data.clientset.CoreV1().Pods(antreaNamespace).DeleteCollection(context.TODO(), deleteOptions, listOptions); err != nil {
		return fmt.Errorf("error when deleting all CoreDNS Pods: %v", err)
	}
	return retryOnConnectionLostError(retry.DefaultRetry, func() error { return data.waitForCoreDNSPods(timeout) })
}

// restartAntreaAgentPods deletes all the antrea-agent Pods to force them to be re-scheduled. It
// then waits for the new Pods to become available.
func (data *TestData) restartAntreaAgentPods(timeout time.Duration) error {
	var gracePeriodSeconds int64 = 1
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	}
	listOptions := metav1.ListOptions{
		LabelSelector: antreaPodLabel,
	}
	if err := data.clientset.CoreV1().Pods(antreaNamespace).DeleteCollection(context.TODO(), deleteOptions, listOptions); err != nil {
		return fmt.Errorf("error when deleting antrea-agent Pods: %v", err)
	}
	return data.waitForAntreaDaemonSetPods(timeout)
}

// checkCoreDNSPods checks that all the Pods for the CoreDNS deployment are ready. If not, it
// deletes all the Pods to force them to restart and waits up to timeout for the Pods to become
// ready.
func (data *TestData) checkCoreDNSPods(timeout time.Duration) error {
	if deployment, err := data.clientset.AppsV1().Deployments(antreaNamespace).Get(context.TODO(), "coredns", metav1.GetOptions{}); err != nil {
		return fmt.Errorf("error when retrieving CoreDNS deployment: %v", err)
	} else if deployment.Status.UnavailableReplicas == 0 {
		// deployment ready, nothing to do
		return nil
	}
	return data.restartCoreDNSPods(timeout)
}

// CreateClient initializes the K8s clientset in the TestData structure.
func (data *TestData) CreateClient(kubeconfigPath string) error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath
	configOverrides := &clientcmd.ConfigOverrides{}

	kubeConfig, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("error when building kube config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when creating kubernetes client: %v", err)
	}
	aggregatorClient, err := aggregatorclientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when creating kubernetes aggregatorClient: %v", err)
	}
	crdClient, err := crdclientset.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("error when creating CRD client: %v", err)
	}
	data.kubeConfig = kubeConfig
	data.clientset = clientset
	data.aggregatorClient = aggregatorClient
	data.crdClient = crdClient
	return nil
}

func labelNodeRoleControlPlane() string {
	// TODO: return labelNodeRoleControlPlane unconditionally when the min K8s version
	// requirement to run Antrea becomes K8s v1.20
	const labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	const labelNodeRoleOldControlPlane = "node-role.kubernetes.io/master"
	// If clusterInfo.k8sServerVersion < "v1.20.0"
	if semver.Compare(clusterInfo.k8sServerVersion, "v1.20.0") < 0 {
		return labelNodeRoleOldControlPlane
	}
	return labelNodeRoleControlPlane
}

func (data *TestData) collectClusterInfo() error {
	// retrieve Node information
	nodes, err := testData.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error when listing cluster Nodes: %v", err)
	}
	workerIdx := 1
	clusterInfo.nodes = make(map[int]ClusterNode)
	clusterInfo.nodesOS = make(map[string]string)

	// retrieve K8s server version

	serverVersion, err := testData.clientset.Discovery().ServerVersion()
	if err != nil {
		return err
	}
	clusterInfo.k8sServerVersion = serverVersion.String()

	for _, node := range nodes.Items {
		isControlPlaneNode := func() bool {
			_, ok := node.Labels[labelNodeRoleControlPlane()]
			return ok
		}()

		var nodeIPv4 string
		var nodeIPv6 string
		for _, address := range node.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				if utilnet.IsIPv6String(address.Address) {
					nodeIPv6 = address.Address
				} else if utilnet.IsIPv4String(address.Address) {
					nodeIPv4 = address.Address
				}
			}
		}

		var nodeIdx int
		// If multiple control-plane Nodes (HA), we will select the last one in the list
		if isControlPlaneNode {
			nodeIdx = 0
			clusterInfo.controlPlaneNodeName = node.Name
			clusterInfo.controlPlaneNodeIPv4 = nodeIPv4
			clusterInfo.controlPlaneNodeIPv6 = nodeIPv6
		} else {
			nodeIdx = workerIdx
			workerIdx++
		}

		var podV4NetworkCIDR, podV6NetworkCIDR string
		var gwV4Addr, gwV6Addr string
		processPodCIDR := func(podCIDR string) error {
			_, cidr, err := net.ParseCIDR(podCIDR)
			if err != nil {
				return err
			}
			if cidr.IP.To4() != nil {
				podV4NetworkCIDR = podCIDR
				gwV4Addr = ip.NextIP(cidr.IP).String()
			} else {
				podV6NetworkCIDR = podCIDR
				gwV6Addr = ip.NextIP(cidr.IP).String()
			}
			return nil
		}
		if len(node.Spec.PodCIDRs) == 0 {
			if err := processPodCIDR(node.Spec.PodCIDR); err != nil {
				return fmt.Errorf("error when processing PodCIDR field for Node %s: %v", node.Name, err)
			}
		} else {
			for _, podCIDR := range node.Spec.PodCIDRs {
				if err := processPodCIDR(podCIDR); err != nil {
					return fmt.Errorf("error when processing PodCIDRs field for Node %s: %v", node.Name, err)
				}
			}
		}

		clusterInfo.nodes[nodeIdx] = ClusterNode{
			idx:              nodeIdx,
			name:             node.Name,
			ipv4Addr:         nodeIPv4,
			ipv6Addr:         nodeIPv6,
			podV4NetworkCIDR: podV4NetworkCIDR,
			podV6NetworkCIDR: podV6NetworkCIDR,
			gwV4Addr:         gwV4Addr,
			gwV6Addr:         gwV6Addr,
			os:               node.Status.NodeInfo.OperatingSystem,
		}
		if node.Status.NodeInfo.OperatingSystem == "windows" {
			clusterInfo.windowsNodes = append(clusterInfo.windowsNodes, nodeIdx)
		}
		clusterInfo.nodesOS[node.Name] = node.Status.NodeInfo.OperatingSystem
	}
	if clusterInfo.controlPlaneNodeName == "" {
		return fmt.Errorf("error when listing cluster Nodes: control-plane Node not found")
	}
	clusterInfo.numNodes = workerIdx

	retrieveCIDRs := func(cmd string, reg string) ([]string, error) {
		res := make([]string, 2)
		rc, stdout, _, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if err != nil || rc != 0 {
			return res, fmt.Errorf("error when running the following command `%s` on control-plane Node: %v, %s", cmd, err, stdout)
		}
		re := regexp.MustCompile(reg)
		matches := re.FindStringSubmatch(stdout)
		if len(matches) == 0 {
			return res, fmt.Errorf("cannot retrieve CIDR, unexpected kubectl output: %s", stdout)
		}
		cidrs := strings.Split(matches[1], ",")
		if len(cidrs) == 1 {
			_, cidr, err := net.ParseCIDR(cidrs[0])
			if err != nil {
				return res, fmt.Errorf("CIDR cannot be parsed: %s", cidrs[0])
			}
			if cidr.IP.To4() != nil {
				res[0] = cidrs[0]
			} else {
				res[1] = cidrs[0]
			}
		} else if len(cidrs) == 2 {
			_, cidr, err := net.ParseCIDR(cidrs[0])
			if err != nil {
				return res, fmt.Errorf("CIDR cannot be parsed: %s", cidrs[0])
			}
			if cidr.IP.To4() != nil {
				res[0] = cidrs[0]
				res[1] = cidrs[1]
			} else {
				res[0] = cidrs[1]
				res[1] = cidrs[0]
			}
		} else {
			return res, fmt.Errorf("unexpected cluster CIDR: %s", matches[1])
		}
		return res, nil
	}

	// retrieve cluster CIDRs
	podCIDRs, err := retrieveCIDRs("kubectl cluster-info dump | grep cluster-cidr", `cluster-cidr=([^"]+)`)
	if err != nil {
		return err
	}
	clusterInfo.podV4NetworkCIDR = podCIDRs[0]
	clusterInfo.podV6NetworkCIDR = podCIDRs[1]

	// retrieve service CIDRs
	svcCIDRs, err := retrieveCIDRs("kubectl cluster-info dump | grep service-cluster-ip-range", `service-cluster-ip-range=([^"]+)`)
	if err != nil {
		return err
	}
	clusterInfo.svcV4NetworkCIDR = svcCIDRs[0]
	clusterInfo.svcV6NetworkCIDR = svcCIDRs[1]

	// Retrieve kubernetes Service host and Port
	svc, err := testData.clientset.CoreV1().Services("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get Service kubernetes: %v", err)
	}
	clusterInfo.k8sServiceHost = svc.Spec.ClusterIP
	clusterInfo.k8sServicePort = svc.Spec.Ports[0].Port

	return nil
}

// deleteTestNamespace deletes test namespace and waits for deletion to actually complete.
func (data *TestData) deleteTestNamespace(timeout time.Duration) error {
	return data.DeleteNamespace(testNamespace, timeout)
}

// DeleteNamespace deletes the provided namespace and waits for deletion to actually complete.
func (data *TestData) DeleteNamespace(namespace string, timeout time.Duration) error {
	var gracePeriodSeconds int64
	var propagationPolicy = metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
		PropagationPolicy:  &propagationPolicy,
	}

	// To log time statistics
	startTime := time.Now()
	defer func() {
		log.Infof("Deleting Namespace %s took %v", namespace, time.Since(startTime))
	}()

	if err := data.clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, deleteOptions); err != nil {
		if errors.IsNotFound(err) {
			// namespace does not exist, we return right away
			return nil
		}
		return fmt.Errorf("error when deleting '%s' Namespace: %v", namespace, err)
	}
	err := wait.Poll(defaultInterval, timeout, func() (bool, error) {
		if ns, err := data.clientset.CoreV1().Namespaces().Get(context.TODO(), namespace, metav1.GetOptions{}); err != nil {
			if errors.IsNotFound(err) {
				// Success
				return true, nil
			}
			return false, fmt.Errorf("error when getting Namespace '%s' after delete: %v", namespace, err)
		} else if ns.Status.Phase != corev1.NamespaceTerminating {
			return false, fmt.Errorf("deleted Namespace '%s' should be in 'Terminating' phase", namespace)
		}

		// Keep trying
		return false, nil
	})
	return err
}

// deleteNetworkpolicy deletes the network policy.
func (data *TestData) deleteNetworkpolicy(policy *networkingv1.NetworkPolicy) error {
	if err := data.clientset.NetworkingV1().NetworkPolicies(policy.Namespace).Delete(context.TODO(), policy.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("unable to cleanup policy %v: %v", policy.Name, err)
	}
	return nil
}

// deleteAntreaNetworkpolicy deletes an Antrea NetworkPolicy.
func (data *TestData) deleteAntreaNetworkpolicy(policy *crdv1alpha1.NetworkPolicy) error {
	if err := data.crdClient.CrdV1alpha1().NetworkPolicies(testNamespace).Delete(context.TODO(), policy.Name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("unable to cleanup policy %v: %v", policy.Name, err)
	}
	return nil
}

// DeleteANP is a convenience function for deleting ANP by name and Namespace.
func (data *TestData) DeleteANP(ns, name string) error {
	log.Infof("Deleting Antrea NetworkPolicy '%s/%s'", ns, name)
	err := data.crdClient.CrdV1alpha1().NetworkPolicies(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete Antrea NetworkPolicy %s: %v", name, err)
	}
	return nil
}

// CreateOrUpdateANP is a convenience function for updating/creating Antrea NetworkPolicies.
func (data *TestData) CreateOrUpdateANP(anp *crdv1alpha1.NetworkPolicy) (*crdv1alpha1.NetworkPolicy, error) {
	log.Infof("Creating/updating Antrea NetworkPolicy %s/%s", anp.Namespace, anp.Name)
	cnpReturned, err := data.crdClient.CrdV1alpha1().NetworkPolicies(anp.Namespace).Get(context.TODO(), anp.Name, metav1.GetOptions{})
	if err != nil {
		log.Debugf("Creating Antrea NetworkPolicy %s", anp.Name)
		anp, err = data.crdClient.CrdV1alpha1().NetworkPolicies(anp.Namespace).Create(context.TODO(), anp, metav1.CreateOptions{})
		if err != nil {
			log.Debugf("Unable to create Antrea NetworkPolicy: %s", err)
		}
		return anp, err
	} else if cnpReturned.Name != "" {
		log.Debugf("Antrea NetworkPolicy with name %s already exists, updating", anp.Name)
		anp, err = data.crdClient.CrdV1alpha1().NetworkPolicies(anp.Namespace).Update(context.TODO(), anp, metav1.UpdateOptions{})
		return anp, err
	}
	return nil, fmt.Errorf("error occurred in creating/updating Antrea NetworkPolicy %s", anp.Name)
}

// DeletePod deletes a Pod in the test namespace.
func (data *TestData) DeletePod(namespace, name string) error {
	var gracePeriodSeconds int64 = 5
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	}
	if err := data.clientset.CoreV1().Pods(namespace).Delete(context.TODO(), name, deleteOptions); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// GetService is a convenience function for getting Service
func (data *TestData) GetService(namespace, name string) (*corev1.Service, error) {
	return data.clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

type PodCondition func(*corev1.Pod) (bool, error)

// PodWaitFor polls the K8s apiserver until the specified Pod is found (in the test Namespace) and
// the condition predicate is met (or until the provided timeout expires).
func (data *TestData) PodWaitFor(timeout time.Duration, name, namespace string, condition PodCondition) (*corev1.Pod, error) {
	var pod *corev1.Pod
	err := wait.Poll(defaultInterval, timeout, func() (bool, error) {
		var err error
		pod, err = data.clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("error when getting Pod '%s': %v", name, err)
		}
		return condition(pod)
	})
	if err != nil {
		if err == wait.ErrWaitTimeout && pod != nil {
			return nil, fmt.Errorf("timed out waiting for the condition, Pod.Status: %s", pod.Status.String())
		}
		return nil, err
	}
	return pod, nil
}

// Gets pod logs from Pod
func (data *TestData) GetPodLogs(namespace, name string, podLogOpts *corev1.PodLogOptions) (string, error) {
	var logString string
	req := data.clientset.CoreV1().Pods(namespace).GetLogs(name, podLogOpts)
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return logString, fmt.Errorf("error when opening stream: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return logString, fmt.Errorf("error when copying data from Pod logs to buffer: %v", err)
	}
	logString = buf.String()
	return logString, nil
}

func parsePodIPs(podIPStrings sets.String) (*PodIPs, error) {
	ips := new(PodIPs)
	for idx := range podIPStrings.List() {
		ipStr := podIPStrings.List()[idx]
		ip := net.ParseIP(ipStr)
		if ip.To4() != nil {
			if ips.ipv4 != nil && ipStr != ips.ipv4.String() {
				return nil, fmt.Errorf("pod is assigned multiple IPv4 addresses: %s and %s", ips.ipv4.String(), ipStr)
			}
			if ips.ipv4 == nil {
				ips.ipv4 = &ip
				ips.ipStrings = append(ips.ipStrings, ipStr)
			}
		} else {
			if ips.ipv6 != nil && ipStr != ips.ipv6.String() {
				return nil, fmt.Errorf("pod is assigned multiple IPv6 addresses: %s and %s", ips.ipv6.String(), ipStr)
			}
			if ips.ipv6 == nil {
				ips.ipv6 = &ip
				ips.ipStrings = append(ips.ipStrings, ipStr)
			}
		}
	}
	if len(ips.ipStrings) == 0 {
		return nil, fmt.Errorf("pod is running but has no assigned IP, which should never happen")
	}
	return ips, nil
}

// podWaitForIPs polls the K8s apiserver until the specified Pod is in the "running" state (or until
// the provided timeout expires). The function then returns the IP addresses assigned to the Pod. If the
// Pod is not using "hostNetwork", the function also checks that an IP address exists in each required
// Address Family in the cluster.
func (data *TestData) podWaitForIPs(timeout time.Duration, name, namespace string) (*PodIPs, error) {
	pod, err := data.PodWaitFor(timeout, name, namespace, func(pod *corev1.Pod) (bool, error) {
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
	podIPStrings := sets.NewString(pod.Status.PodIP)
	for _, podIP := range pod.Status.PodIPs {
		ipStr := strings.TrimSpace(podIP.IP)
		if ipStr != "" {
			podIPStrings.Insert(ipStr)
		}
	}
	ips, err := parsePodIPs(podIPStrings)
	if err != nil {
		return nil, err
	}

	if !pod.Spec.HostNetwork {
		if clusterInfo.podV4NetworkCIDR != "" && ips.ipv4 == nil {
			return nil, fmt.Errorf("no IPv4 address is assigned while cluster was configured with IPv4 Pod CIDR %s", clusterInfo.podV4NetworkCIDR)
		}
		if clusterInfo.podV6NetworkCIDR != "" && ips.ipv6 == nil {
			return nil, fmt.Errorf("no IPv6 address is assigned while cluster was configured with IPv6 Pod CIDR %s", clusterInfo.podV6NetworkCIDR)
		}
	}
	return ips, nil
}

// podWaitForReady polls the k8s apiserver until the specified Pod is in the "Ready" status (or
// until the provided timeout expires).
func (data *TestData) podWaitForReady(timeout time.Duration, name, namespace string) error {
	_, err := data.PodWaitFor(timeout, name, namespace, func(p *corev1.Pod) (bool, error) {
		for _, condition := range p.Status.Conditions {
			if condition.Type == corev1.PodReady {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}
		return false, nil
	})
	return err
}

// getImageName gets the image name from the fully qualified URI.
// For example: "gcr.io/kubernetes-e2e-test-images/agnhost:2.8" gets "agnhost".
func getImageName(uri string) string {
	registryAndImage := strings.Split(uri, ":")[0]
	paths := strings.Split(registryAndImage, "/")
	return paths[len(paths)-1]
}

func controlPlaneNodeName() string {
	return clusterInfo.controlPlaneNodeName
}

// nodeName returns an empty string if there is no Node with the provided idx. If idx is 0, the name
// of the control-plane Node will be returned.
func nodeName(idx int) string {
	node, ok := clusterInfo.nodes[idx]
	if !ok {
		return ""
	}
	return node.name
}

// workerNodeName returns an empty string if there is no worker Node with the provided idx
// (including if idx is 0, which is reserved for the control-plane Node)
func workerNodeName(idx int) string {
	if idx == 0 { // control-plane Node
		return ""
	}
	node, ok := clusterInfo.nodes[idx]
	if !ok {
		return ""
	}
	return node.name
}

func controlPlaneNoScheduleTolerations() []corev1.Toleration {
	// Use both "old" and "new" label for NoSchedule Toleration
	return []corev1.Toleration{
		{
			Key:      "node-role.kubernetes.io/master",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}

// CreatePodOnNodeInNamespace creates a pod in the provided namespace with a container whose type is decided by imageName.
// Pod will be scheduled on the specified Node (if nodeName is not empty).
// mutateFunc can be used to customize the Pod if the other parameters don't meet the requirements.
func (data *TestData) CreatePodOnNodeInNamespace(name, ns string, nodeName, ctrName string, image string, command []string, args []string, env []corev1.EnvVar, ports []corev1.ContainerPort, hostNetwork bool, mutateFunc func(*corev1.Pod)) error {
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
	if nodeName == controlPlaneNodeName() {
		// tolerate NoSchedule taint if we want Pod to run on control-plane Node
		podSpec.Tolerations = controlPlaneNoScheduleTolerations()
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
	if _, err := data.clientset.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

// createPodOnNode creates a pod in the test namespace with a container whose type is decided by imageName.
// Pod will be scheduled on the specified Node (if nodeName is not empty).
// mutateFunc can be used to customize the Pod if the other parameters don't meet the requirements.
func (data *TestData) createPodOnNode(name string, ns string, nodeName string, image string, command []string, args []string, env []corev1.EnvVar, ports []corev1.ContainerPort, hostNetwork bool, mutateFunc func(*corev1.Pod)) error {
	// image could be a fully qualified URI which can't be used as container name and label value,
	// extract the image name from it.
	imageName := getImageName(image)
	return data.CreatePodOnNodeInNamespace(name, ns, nodeName, imageName, image, command, args, env, ports, hostNetwork, mutateFunc)
}

// createServerPod creates a Pod that can listen to specified port and have named port set.
func (data *TestData) createServerPod(name string, ns string, portName string, portNum int32, setHostPort bool, hostNetwork bool) error {
	// See https://github.com/kubernetes/kubernetes/blob/master/test/images/agnhost/porter/porter.go#L17 for the image's detail.
	cmd := "porter"
	env := corev1.EnvVar{Name: fmt.Sprintf("SERVE_PORT_%d", portNum), Value: "foo"}
	port := corev1.ContainerPort{Name: portName, ContainerPort: portNum}
	if setHostPort {
		// If hostPort is to be set, it must match the container port number.
		port.HostPort = int32(portNum)
	}
	return data.createPodOnNode(name, ns, "", agnhostImage, nil, []string{cmd}, []corev1.EnvVar{env}, []corev1.ContainerPort{port}, hostNetwork, nil)
}

// createBusyboxPodOnNode creates a Pod in the test namespace with a single busybox container. The
// Pod will be scheduled on the specified Node (if nodeName is not empty).
func (data *TestData) createBusyboxPodOnNode(name string, ns string, nodeName string, hostNetwork bool) error {
	sleepDuration := 3600 // seconds
	return data.createPodOnNode(name, ns, nodeName, busyboxImage, []string{"sleep", strconv.Itoa(sleepDuration)}, nil, nil, nil, hostNetwork, nil)
}

// getFlowAggregator retrieves the name of the Flow-Aggregator Pod (flow-aggregator-*) running on a specific Node.
func (data *TestData) getFlowAggregator() (*corev1.Pod, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app=flow-aggregator",
	}
	pods, err := data.clientset.CoreV1().Pods(flowAggregatorNamespace).List(context.TODO(), listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list Flow Aggregator Pod: %v", err)
	}
	if len(pods.Items) != 1 {
		return nil, fmt.Errorf("expected *exactly* one Pod")
	}
	return &pods.Items[0], nil
}

// RunCommandFromPod Run the provided command in the specified Container for the give Pod and returns the contents of
// stdout and stderr as strings. An error either indicates that the command couldn't be run or that
// the command returned a non-zero error code.
func (data *TestData) RunCommandFromPod(podNamespace string, podName string, containerName string, cmd []string) (stdout string, stderr string, err error) {
	request := data.clientset.CoreV1().RESTClient().Post().
		Namespace(podNamespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		Param("container", containerName).
		VersionedParams(&corev1.PodExecOptions{
			Command: cmd,
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			TTY:     false,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(data.kubeConfig, "POST", request.URL())
	if err != nil {
		return "", "", err
	}
	var stdoutB, stderrB bytes.Buffer
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdoutB,
		Stderr: &stderrB,
	}); err != nil {
		return stdoutB.String(), stderrB.String(), err
	}
	return stdoutB.String(), stderrB.String(), nil
}

func (data *TestData) InitProvider(providerName, providerConfigPath string) error {
	providerFactory := map[string]func(string) (providers.ProviderInterface, error){
		"vagrant": providers.NewVagrantProvider,
		"kind":    providers.NewKindProvider,
		"remote":  providers.NewRemoteProvider,
	}
	if fn, ok := providerFactory[providerName]; ok {
		newProvider, err := fn(providerConfigPath)
		if err != nil {
			return err
		}
		data.provider = newProvider
	} else {
		return fmt.Errorf("unknown provider '%s'", providerName)
	}
	return nil
}

// RunCommandOnNode is a convenience wrapper around the Provider interface RunCommandOnNode method.
func (data *TestData) RunCommandOnNode(nodeName string, cmd string) (code int, stdout string, stderr string, err error) {
	return data.provider.RunCommandOnNode(nodeName, cmd)
}

// createNetworkPolicy creates a network policy with spec.
func (data *TestData) createNetworkPolicy(name string, spec *networkingv1.NetworkPolicySpec) (*networkingv1.NetworkPolicy, error) {
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"antrea-e2e": name,
			},
		},
		Spec: *spec,
	}
	return data.clientset.NetworkingV1().NetworkPolicies(testNamespace).Create(context.TODO(), policy, metav1.CreateOptions{})
}

// CreateService creates a service with port and targetPort.
func (data *TestData) CreateService(serviceName, namespace string, port, targetPort int32, selector map[string]string, affinity, nodeLocalExternal bool,
	serviceType corev1.ServiceType, ipFamily *corev1.IPFamily) (*corev1.Service, error) {
	annotation := make(map[string]string)
	return data.CreateServiceWithAnnotations(serviceName, namespace, port, targetPort, corev1.ProtocolTCP, selector, affinity, nodeLocalExternal, serviceType, ipFamily, annotation)
}

// CreateServiceWithAnnotations creates a service with Annotation
func (data *TestData) CreateServiceWithAnnotations(serviceName, namespace string, port, targetPort int32, protocol corev1.Protocol, selector map[string]string, affinity, nodeLocalExternal bool,
	serviceType corev1.ServiceType, ipFamily *corev1.IPFamily, annotations map[string]string) (*corev1.Service, error) {
	affinityType := corev1.ServiceAffinityNone
	var ipFamilies []corev1.IPFamily
	if ipFamily != nil {
		ipFamilies = append(ipFamilies, *ipFamily)
	}
	if affinity {
		affinityType = corev1.ServiceAffinityClientIP
	}
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				"antrea-e2e": serviceName,
				"app":        serviceName,
			},
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			SessionAffinity: affinityType,
			Ports: []corev1.ServicePort{{
				Port:       port,
				TargetPort: intstr.FromInt(int(targetPort)),
				Protocol:   protocol,
			}},
			Type:       serviceType,
			Selector:   selector,
			IPFamilies: ipFamilies,
		},
	}
	if (serviceType == corev1.ServiceTypeNodePort || serviceType == corev1.ServiceTypeLoadBalancer) && nodeLocalExternal {
		service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal
	}
	return data.clientset.CoreV1().Services(namespace).Create(context.TODO(), &service, metav1.CreateOptions{})
}

func (data *TestData) WaitNetworkPolicyRealize(nodeName string, table *openflow.Table, policyRules int) error {
	return wait.PollImmediate(50*time.Millisecond, realizeTimeout, func() (bool, error) {
		return data.checkRealize(nodeName, table, policyRules)
	})
}

// checkRealize checks if all CIDR rules in the Network Policy have been realized as OVS flows. It counts the number of
// flows installed in the ingressRuleTable of the OVS bridge of the control-plane Node. This relies on the implementation
// knowledge that given a single ingress policy, the Antrea agent will install exactly one flow per CIDR rule in table
// IngressRule. checkRealize returns true when the number of flows exceeds the number of CIDR, because each table has a
// default flow entry which is used for default matching.
// Since the check is done over SSH, the time measurement is not completely accurate.
func (data *TestData) checkRealize(nodeName string, table *openflow.Table, policyRules int) (bool, error) {
	antreaPodName, err := data.getAntreaPodOnNode(nodeName)
	if err != nil {
		return false, err
	}
	// table IngressRule is the ingressRuleTable where the rules in workload network policy is being applied to.
	cmd := []string{"ovs-ofctl", "dump-flows", defaultBridgeName, fmt.Sprintf("table=%s", table.GetName())}
	stdout, _, err := data.RunCommandFromPod(antreaNamespace, antreaPodName, "antrea-agent", cmd)
	if err != nil {
		return false, err
	}
	flowNums := strings.Count(stdout, "\n")
	return flowNums > policyRules, nil
}

// getAntreaPodOnNode retrieves the name of the Antrea Pod (antrea-agent-*) running on a specific Node.
func (data *TestData) getAntreaPodOnNode(nodeName string) (podName string, err error) {
	listOptions := metav1.ListOptions{
		LabelSelector: antreaPodLabel,
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
	}
	pods, err := data.clientset.CoreV1().Pods(antreaNamespace).List(context.TODO(), listOptions)
	if err != nil {
		return "", fmt.Errorf("failed to list Antrea Pods: %v", err)
	}
	if len(pods.Items) != 1 {
		return "", fmt.Errorf("expected *exactly* one Pod")
	}
	return pods.Items[0].Name, nil
}

// CreateNamespace creates the provided namespace.
func (data *TestData) CreateNamespace(namespace string, mutateFunc func(*corev1.Namespace)) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if mutateFunc != nil {
		mutateFunc(ns)
	}
	if ns, err := data.clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
		// Ignore error if the namespace already exists
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("error when creating '%s' Namespace: %v", namespace, err)
		}
		// When namespace already exists, check phase
		if ns.Status.Phase == corev1.NamespaceTerminating {
			return fmt.Errorf("error when creating '%s' Namespace: namespace exists but is in 'Terminating' phase", namespace)
		}
	}
	return nil
}

// createTestNamespace creates the namespace used for tests.
func (data *TestData) createTestNamespace() error {
	return data.CreateNamespace(testNamespace, nil)
}

// deployFlowVisibility deploys ClickHouse Operator and DB.
// Other applications will be deployed according to config.
func (data *TestData) deployFlowVisibility(config FlowVisibiltiySetUpConfig) (chSvcIP string, err error) {
	if config.withClickHouseLocalPv {
		// Label one of the worker Node to fulfill the Local PersistentVolume affinity requirement.
		node, err := data.clientset.CoreV1().Nodes().Get(context.TODO(), workerNodeName(1), metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("error when getting Node %s: %v", workerNodeName(1), err)
		}
		node.ObjectMeta.Labels[clickHouseLocalPvLabel] = ""
		_, err = data.clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		if err != nil {
			return "", fmt.Errorf("error when updating Node %s: %v", workerNodeName(1), err)
		}
		// Create the directory for ClickHouse Local Persistent Volume on the Node
		rc, _, stderr, err := data.provider.RunCommandOnNode(workerNodeName(1), fmt.Sprintf("mkdir -p %s", clickHouseLocalPvPath))
		if err != nil || rc != 0 {
			return "", fmt.Errorf("error when creating directory for ClickHouse PersistentVolume on Node %s: %v", workerNodeName(1), stderr)
		}
	}
	var flowVisibilityManifest string
	if !config.withGrafana && !config.withSparkOperator {
		flowVisibilityManifest = flowVisibilityChOnlyYML
	} else if config.withSparkOperator {
		flowVisibilityManifest = flowVisibilityWithSparkYML
	} else {
		flowVisibilityManifest = flowVisibilityYML
	}
	rc, _, _, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl apply -f %s", clickHouseOperatorYML))
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when deploying the ClickHouse Operator YML; %s not available on the control-plane Node", clickHouseOperatorYML)
	}
	err = data.deployFlowVisibilityCommon(flowVisibilityManifest)
	if err != nil {
		return "", err
	}
	if config.withSparkOperator {
		sparkOperatorPodName, err := data.getPodByLabel(sparkOperatorPodLabel, flowVisibilityNamespace)
		if err != nil {
			return "", fmt.Errorf("error when getting the Spark Operator Pod name: %v", err)
		}

		// check for Spark Operator Pod ready.
		if err = data.podWaitForReady(defaultTimeout, sparkOperatorPodName, flowVisibilityNamespace); err != nil {
			return "", err
		}
		theiaManagerPodName, err := data.getPodByLabel(theiaManagerPodLabel, flowVisibilityNamespace)
		if err != nil {
			return "", fmt.Errorf("error when getting the Theia Manager Pod name: %v", err)
		}

		// check for Theia Manager Pod ready.
		if err = data.podWaitForReady(defaultTimeout, theiaManagerPodName, flowVisibilityNamespace); err != nil {
			return "", err
		}
	}

	// check for ClickHouse Pod ready. Wait for 2x timeout as ch operator needs to be running first to handle chi
	clickHousePodName := fmt.Sprintf("%s-0-0-0", clickHousePodNamePrefix)
	if err = data.podWaitForReady(2*defaultTimeout, clickHousePodName, flowVisibilityNamespace); err != nil {
		return "", err
	}

	// check ClickHouse Service http port for Service connectivity
	chSvc, err := data.GetService(flowVisibilityNamespace, "clickhouse-clickhouse")
	if err != nil {
		return "", err
	}
	if err := wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		rc, _, _, err := testData.RunCommandOnNode(controlPlaneNodeName(),
			fmt.Sprintf("curl -Ss %s:%s", chSvc.Spec.ClusterIP, clickHouseHTTPPort))
		if rc != 0 || err != nil {
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		return "", fmt.Errorf("timeout checking http port connectivity of clickhouse service: %v", err)
	}

	if config.withGrafana {
		grafanaPodName, err := data.getPodByLabel(grafanaPodLabel, flowVisibilityNamespace)
		if err != nil {
			return "", fmt.Errorf("error when getting the Grafana Pod name: %v", err)
		}
		// check for Grafana Pod ready.
		if err = data.podWaitForReady(defaultTimeout, grafanaPodName, flowVisibilityNamespace); err != nil {
			return "", err
		}
	}

	return chSvc.Spec.ClusterIP, nil
}

func (data *TestData) deployFlowVisibilityCommon(yamlFile string) error {
	if err := wait.Poll(2*time.Second, 10*time.Second, func() (bool, error) {
		rc, stdout, stderr, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl apply -f %s", yamlFile))
		if err != nil || rc != 0 {
			log.Infof("error when deploying the flow visibility YML %s: %s, %s, %v", yamlFile, stdout, stderr, err)
			// ClickHouseInstallation CRD from ClickHouse Operator install bundle applied soon before
			// applying CR. Sometimes apiserver validation fails to recognize resource of
			// kind: ClickHouseInstallation. Retry in such scenario.
			if strings.Contains(stderr, "ClickHouseInstallation") || strings.Contains(stdout, "ClickHouseInstallation") {
				return false, nil
			}
			return false, fmt.Errorf("error when deploying the flow visibility YML %s: %s, %s, %v", yamlFile, stdout, stderr, err)
		}
		return true, nil
	}); err != nil {
		return err
	}
	return nil
}

// waitForClickHousePod waits for the completion of the ClickHouse Pod updating
// after applying a new Flow Visibility manifest.
func (data *TestData) waitForClickHousePod() error {
	startUpdating := false
	// ClickHouse Operator takes around 1 minute to restart the ClickHouse Pod
	// which requires more time to make sure that ClickHouse Pod is ready.
	err := wait.Poll(defaultInterval, defaultTimeout*2, func() (bool, error) {
		clickHouseStatefulSetName := fmt.Sprintf("%s-0-0", clickHousePodNamePrefix)
		ss, err := data.clientset.AppsV1().StatefulSets(flowVisibilityNamespace).Get(context.TODO(), clickHouseStatefulSetName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error when getting ClickHouse statefulset: %v", err)
		}
		if !startUpdating {
			// The difference between UpdateRevision and CurrentRevision indicates the updating starts.
			if ss.Status.CurrentRevision != ss.Status.UpdateRevision {
				startUpdating = true
			}
			return false, nil
		}
		// The updating is completed when CurrentRevision is updated to UpdateRevision
		return ss.Status.CurrentRevision == ss.Status.UpdateRevision, nil
	})
	if err == wait.ErrWaitTimeout {
		clickHousePodName := fmt.Sprintf("%s-0-0-0", clickHousePodNamePrefix)
		_, stdout, _, _ := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s describe pod %s", flowVisibilityNamespace, clickHousePodName))
		return fmt.Errorf("ClickHouse StatefulSet not ready within %v; kubectl describe pod output: %v", defaultTimeout*2, stdout)
	} else if err != nil {
		return err
	}
	return nil
}

// deployFlowAggregator deploys the Flow Aggregator.
func (data *TestData) deployFlowAggregator() error {
	flowAggYaml := flowAggregatorYML
	rc, _, _, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl apply -f %s", flowAggYaml))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deploying the Flow Aggregator; %s not available on the control-plane Node", flowAggYaml)
	}

	if rc, _, _, err = data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s rollout status deployment/%s --timeout=%v", flowAggregatorNamespace, flowAggregatorDeployment, 2*defaultTimeout)); err != nil || rc != 0 {
		_, stdout, _, _ := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s describe pod", flowAggregatorNamespace))
		_, logStdout, _, _ := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl -n %s logs -l app=flow-aggregator", flowAggregatorNamespace))
		return fmt.Errorf("error when waiting for the Flow Aggregator rollout to complete. kubectl describe output: %s, logs: %s", stdout, logStdout)
	}
	// Check for flow-aggregator pod running again for db connection establishment
	flowAggPod, err := data.getFlowAggregator()
	if err != nil {
		return fmt.Errorf("error when getting flow-aggregator Pod: %v", err)
	}
	podName := flowAggPod.Name
	_, err = data.PodWaitFor(defaultTimeout*2, podName, flowAggregatorNamespace, func(p *corev1.Pod) (bool, error) {
		for _, condition := range p.Status.Conditions {
			if condition.Type == corev1.PodReady {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}
		return false, nil
	})
	if err != nil {
		_, stdout, stderr, podErr := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl get pod %s -n %s -o yaml", podName, flowAggregatorNamespace))
		return fmt.Errorf("error when waiting for flow-aggregator Ready: %v; stdout %s, stderr: %s, %v", err, stdout, stderr, podErr)
	}
	return nil
}

// getPodByLabel retrieves the name of the desired Pod.
func (data *TestData) getPodByLabel(podLabel, ns string) (string, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: podLabel,
	}
	var pod *corev1.Pod
	if err := wait.Poll(defaultInterval, defaultTimeout, func() (bool, error) {
		pods, err := data.clientset.CoreV1().Pods(ns).List(context.TODO(), listOptions)
		if err != nil {
			return false, fmt.Errorf("failed to list Pod: %v", err)
		}
		if len(pods.Items) == 0 {
			return false, nil
		}
		pod = &pods.Items[0]
		return true, nil
	}); err != nil {
		return "", err
	}

	return pod.Name, nil
}

func (data *TestData) deleteClickHouseOperator() error {
	rc, _, _, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl delete -f %s -n kube-system", clickHouseOperatorYML))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deleting ClickHouse operator: %v", err)
	}
	return nil
}

func teardownFlowVisibility(tb testing.TB, data *TestData, config FlowVisibiltiySetUpConfig) {
	if config.withFlowAggregator {
		if err := data.DeleteNamespace(flowAggregatorNamespace, defaultTimeout); err != nil {
			tb.Logf("Error when tearing down flow aggregator: %v", err)
		}
	}
	if err := data.deleteFlowVisibility(config); err != nil {
		tb.Logf("Error when deleting K8s resources created by flow visibility: %v", err)
	}
	if err := data.deleteClickHouseOperator(); err != nil {
		tb.Logf("Error when deleting ClickHouse Operator: %v", err)
	}
}

func (data *TestData) deleteFlowVisibility(config FlowVisibiltiySetUpConfig) error {
	var flowVisibilityManifest string
	if !config.withGrafana && !config.withSparkOperator {
		flowVisibilityManifest = flowVisibilityChOnlyYML
	} else if config.withSparkOperator {
		flowVisibilityManifest = flowVisibilityWithSparkYML
	} else {
		flowVisibilityManifest = flowVisibilityYML
	}
	startTime := time.Now()
	defer func() {
		log.Infof("Deleting K8s resources created by flow visibility YAML took %v", time.Since(startTime))
	}()
	rc, _, stderr, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl delete clickhouseinstallation.clickhouse.altinity.com clickhouse -n %s", flowVisibilityNamespace))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deleting ClickHouse StatefulSet: %v, stderr: %s", err, stderr)
	}

	rc, _, stderr, err = data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl delete -f %s --ignore-not-found=true", flowVisibilityManifest))
	if err != nil || rc != 0 {
		return fmt.Errorf("error when deleting K8s resources created by flow visibility YAML: %v, stderr: %s", err, stderr)
	}
	return nil
}

func deleteRecommendedPolicies(tb testing.TB, data *TestData) {
	startTime := time.Now()
	defer func() {
		log.Infof("Deleting recommended policies took %v", time.Since(startTime))
	}()
	// We do not check rc here as rc will not be 0. It will report the NetworkPolicies
	// under the testNamespace not found, as they have already been deleted along with
	// the testNamespace.
	_, _, stderr, err := data.provider.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("kubectl delete -f %s", policyOutputYML))
	if err != nil {
		tb.Logf("Error when deleting recommended policies: %v, stderr: %s", err, stderr)
	}
}

// DeleteACNP is a convenience function for deleting ACNP by name.
func (data *TestData) DeleteACNP(name string) error {
	log.Infof("Deleting AntreaClusterNetworkPolicies %s", name)
	err := data.crdClient.CrdV1alpha1().ClusterNetworkPolicies().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete ClusterNetworkPolicy %s: %v", name, err)
	}
	return nil
}

// CleanACNPs is a convenience function for deleting all Antrea ClusterNetworkPolicies in the cluster.
func (data *TestData) CleanACNPs() error {
	l, err := data.crdClient.CrdV1alpha1().ClusterNetworkPolicies().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list AntreaClusterNetworkPolicies: %v", err)
	}
	for _, cnp := range l.Items {
		if err = data.DeleteACNP(cnp.Name); err != nil {
			return err
		}
	}
	return nil
}

// DeleteV1Alpha2CG is a convenience function for deleting crd/v1alpha2 ClusterGroup by name.
func (data *TestData) DeleteV1Alpha2CG(name string) error {
	log.Infof("Deleting ClusterGroup %s", name)
	err := data.crdClient.CrdV1alpha2().ClusterGroups().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete ClusterGroup %s: %v", name, err)
	}
	return nil
}

// DeleteV1Alpha3CG is a convenience function for deleting core/v1alpha3 ClusterGroup by name.
func (data *TestData) DeleteV1Alpha3CG(name string) error {
	log.Infof("deleting ClusterGroup %s", name)
	err := data.crdClient.CrdV1alpha3().ClusterGroups().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("unable to delete ClusterGroup %s: %v", name, err)
	}
	return nil
}

// CleanCGs is a convenience function for deleting all ClusterGroups in the cluster.
func (data *TestData) CleanCGs() error {
	l, err := data.crdClient.CrdV1alpha2().ClusterGroups().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list ClusterGroups in v1alpha2: %v", err)
	}
	for _, cg := range l.Items {
		if err := data.DeleteV1Alpha2CG(cg.Name); err != nil {
			return err
		}
	}
	l2, err := data.crdClient.CrdV1alpha3().ClusterGroups().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("unable to list ClusterGroups in v1alpha3: %v", err)
	}
	for _, cg := range l2.Items {
		if err := data.DeleteV1Alpha3CG(cg.Name); err != nil {
			return err
		}
	}
	return nil
}

func (data *TestData) Cleanup(namespaces []string) {
	// Cleanup any cluster-scoped resources.
	if err := data.CleanACNPs(); err != nil {
		log.Errorf("Error when cleaning up ACNPs: %v", err)
	}
	if err := data.CleanCGs(); err != nil {
		log.Errorf("Error when cleaning up CGs: %v", err)
	}

	for _, ns := range namespaces {
		log.Infof("Deleting test Namespace %s", ns)
		if err := data.DeleteNamespace(ns, defaultTimeout); err != nil {
			log.Errorf("Error when deleting Namespace '%s': %v", ns, err)
		}
	}
}

func flowVisibilityCleanup(tb testing.TB, data *TestData, config FlowVisibiltiySetUpConfig) {
	teardownTest(tb, data)
	teardownFlowVisibility(tb, data, config)
}

func SetupClickHouseConnection(clientset kubernetes.Interface, kubeconfig string) (connect *sql.DB, portForward *portforwarder.PortForwarder, err error) {
	service := "clickhouse-clickhouse"
	listenAddress := "localhost"
	listenPort := 9000
	_, servicePort, err := k8s.GetServiceAddr(clientset, service, config.FlowVisibilityNS, corev1.ProtocolTCP)
	if err != nil {
		return nil, nil, fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
	}
	// Forward the ClickHouse service port
	portForward, err = commands.StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
	if err != nil {
		return nil, nil, fmt.Errorf("error when forwarding port: %v", err)
	}
	endpoint := fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)

	// Connect to ClickHouse and execute query
	username, password, err := clickhouse.GetSecret(clientset, "flow-visibility")
	if err != nil {
		return nil, portForward, err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err = clickhouse.Connect(url)
	if err != nil {
		return nil, portForward, fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	return connect, portForward, nil
}

func randInt(t *testing.T, limit int64) int64 {
	assert := assert.New(t)
	randNum, error := rand.Int(rand.Reader, big.NewInt(limit))
	assert.NoError(error)
	return randNum.Int64()
}
