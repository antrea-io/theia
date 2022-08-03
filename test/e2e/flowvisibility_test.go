// Copyright 2022 Antrea Authors
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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"syscall"

	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	ipfixregistry "github.com/vmware/go-ipfix/pkg/registry"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"antrea.io/antrea/pkg/agent/openflow"
	secv1alpha1 "antrea.io/antrea/pkg/apis/crd/v1alpha1"
	"antrea.io/antrea/test/e2e/utils"
)

/* Sample record in ClickHouse table:
timeInserted: 1637706985
flowStartSeconds: 1637706961
flowEndSeconds: 1637706973
flowEndSecondsFromSourceNode: 1637706973
flowEndSecondsFromDestinationNode: 1637706973
flowEndReason: 3
sourceIP: 10.10.0.79
destinationIP: 10.10.0.80
sourceTransportPort: 44752
destinationTransportPort: 5201
protocolIdentifier: 6
packetTotalCount: 823188
octetTotalCount: 30472817041
packetDeltaCount: 241333
octetDeltaCount: 8982624938
reversePacketTotalCount: 471111
reverseOctetTotalCount: 24500996
reversePacketDeltaCount: 136211
reverseOctetDeltaCount: 7083284
sourcePodName: perftest-a
sourcePodNamespace: antrea-test
sourceNodeName: k8s-node-control-plane
destinationPodName: perftest-b
destinationPodNamespace: antrea-test
destinationNodeName: k8s-node-control-plane
destinationClusterIP: 0.0.0.0
destinationServicePort: 0
destinationServicePortName:
ingressNetworkPolicyName: test-flow-aggregator-networkpolicy-ingress-allow
ingressNetworkPolicyNamespace: antrea-test
ingressNetworkPolicyRuleName:
ingressNetworkPolicyRuleAction: 1
ingressNetworkPolicyType: 1
egressNetworkPolicyName: test-flow-aggregator-networkpolicy-egress-allow
egressNetworkPolicyNamespace: antrea-test
egressNetworkPolicyRuleName:
egressNetworkPolicyRuleAction: 1
egressNetworkPolicyType: 1
tcpState: TIME_WAIT
flowType: 1
sourcePodLabels: {"antrea-e2e":"perftest-a","app":"perftool"}
destinationPodLabels: {"antrea-e2e":"perftest-b","app":"perftool"}
throughput: 15902813472
reverseThroughput: 12381344
throughputFromSourceNode: 15902813472
throughputFromDestinationNode: 15902813472
reverseThroughputFromSourceNode: 12381344
reverseThroughputFromDestinationNode: 12381344
trusted: 0
*/

const (
	clickHousePodName = "chi-clickhouse-clickhouse-0-0-0"
	iperfTimeSec      = 12
	// Set target bandwidth(bits/sec) of iPerf traffic to a relatively small value
	// (default unlimited for TCP), to reduce the variances caused by network performance
	// during 12s, and make the throughput test more stable.
	iperfBandwidth                  = "10m"
	antreaEgressTableInitFlowCount  = 3
	antreaIngressTableInitFlowCount = 6
	ingressTableInitFlowCount       = 1
	egressTableInitFlowCount        = 1
	monitorThreshold                = 0.1
	monitorDeletePercentage         = 0.5
	monitorIperfRounds              = 80
	grafanaCred                     = "admin:admin"
	grafanaLocalPort                = "5000"
	grafanaSvcPort                  = "3000"
	grafanaAddr                     = "http://127.0.0.1:5000"
	grafanaQueryTimeout             = 10 * time.Second
)

var (
	// Single iperf run results in two connections with separate ports (control connection and actual data connection).
	// As 2s is the export active timeout of flow exporter and iperf traffic runs for 12s, we expect totally 12 records
	// exporting to the flow aggregator at time 2s, 4s, 6s, 8s, 10s, and 12s after iperf traffic begins.
	// Since flow aggregator will aggregate records based on 5-tuple connection key and active timeout is 3.5 seconds,
	// we expect 3 records at time 5.5s, 9s, and 12.5s after iperf traffic begins.
	expectedNumDataRecords = 3
	grafanaPorts           = fmt.Sprintf("%s:%s", grafanaLocalPort, grafanaSvcPort)
	portForwardCmd         = exec.Command("kubectl", "port-forward", "service/grafana", "-n", "flow-visibility", grafanaPorts)
)

type testFlow struct {
	srcIP      string
	dstIP      string
	srcPodName string
	dstPodName string
}

func TestFlowVisibility(t *testing.T) {
	data, v4Enabled, v6Enabled, err := setupTestForFlowVisibility(t, false, true)
	if err != nil {
		t.Errorf("Error when setting up test: %v", err)
		failOnError(err, t, data)
	}
	// port-forward Grafana Service port to local port
	if err := portForwardCmd.Start(); err != nil {
		t.Errorf("Error when port forwarding Grafana Service: %v", err)
		failOnError(err, t, data)
	}
	defer portForwardCmd.Process.Kill()
	defer flowVisibilityCleanup(t, data, false)

	podAIPs, podBIPs, podCIPs, podDIPs, podEIPs, err := createPerftestPods(data)
	if err != nil {
		failOnError(fmt.Errorf("error when creating perftest Pods: %v", err), t, data)
	}

	if v4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			testHelper(t, data, podAIPs, podBIPs, podCIPs, podDIPs, podEIPs, false)
		})
	}

	if v6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			testHelper(t, data, podAIPs, podBIPs, podCIPs, podDIPs, podEIPs, true)
		})
	}

}

func testHelper(t *testing.T, data *TestData, podAIPs, podBIPs, podCIPs, podDIPs, podEIPs *PodIPs, isIPv6 bool) {
	svcB, svcC, err := createPerftestServices(data, isIPv6)
	if err != nil {
		failOnError(fmt.Errorf("error when creating perftest Services: %v", err), t, data)
	}

	// IntraNodeFlows tests the case, where Pods are deployed on same Node
	// and their flow information is exported as IPFIX flow records.
	// K8s network policies are being tested here.
	t.Run("IntraNodeFlows", func(t *testing.T) {
		np1, np2 := deployK8sNetworkPolicies(t, data, "perftest-a", "perftest-b")
		defer func() {
			if np1 != nil {
				if err = data.deleteNetworkpolicy(np1); err != nil {
					t.Errorf("Error when deleting network policy: %v", err)
				}
			}
			if np2 != nil {
				if err = data.deleteNetworkpolicy(np2); err != nil {
					t.Errorf("Error when deleting network policy: %v", err)
				}
			}
		}()
		if !isIPv6 {
			checkRecordsForFlows(t, data, podAIPs.ipv4.String(), podBIPs.ipv4.String(), isIPv6, true, false, true, false)
		} else {
			checkRecordsForFlows(t, data, podAIPs.ipv6.String(), podBIPs.ipv6.String(), isIPv6, true, false, true, false)
		}
	})

	// IntraNodeDenyConnIngressANP tests the case, where Pods are deployed on same Node with an Antrea ingress deny policy rule
	// applied to destination Pod (one reject rule, one drop rule) and their flow information is exported as IPFIX flow records.
	// perftest-a -> perftest-b (Ingress reject), perftest-a -> perftest-d (Ingress drop)
	t.Run("IntraNodeDenyConnIngressANP", func(t *testing.T) {
		anp1, anp2 := deployDenyAntreaNetworkPolicies(t, data, "perftest-a", "perftest-b", "perftest-d", controlPlaneNodeName(), controlPlaneNodeName(), true)
		defer func() {
			if anp1 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp1); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
			if anp2 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp2); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-d",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podBIPs.ipv4.String(), podAIPs.ipv4.String(), podDIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, true)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podBIPs.ipv6.String(), podAIPs.ipv6.String(), podDIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, true)
		}
	})

	// IntraNodeDenyConnEgressANP tests the case, where Pods are deployed on same Node with an Antrea egress deny policy rule
	// applied to source Pods (one reject rule, one drop rule) and their flow information is exported as IPFIX flow records.
	// perftest-a (Egress reject) -> perftest-b , perftest-a (Egress drop) -> perftest-d
	t.Run("IntraNodeDenyConnEgressANP", func(t *testing.T) {
		anp1, anp2 := deployDenyAntreaNetworkPolicies(t, data, "perftest-a", "perftest-b", "perftest-d", controlPlaneNodeName(), controlPlaneNodeName(), false)
		defer func() {
			if anp1 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp1); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
			if anp2 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp2); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-d",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podBIPs.ipv4.String(), podAIPs.ipv4.String(), podDIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, true)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podBIPs.ipv6.String(), podAIPs.ipv6.String(), podDIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, true)
		}
	})

	// IntraNodeDenyConnNP tests the case, where Pods are deployed on same Node with an ingress and an egress deny policy rule
	// applied to one destination Pod, one source Pod, respectively and their flow information is exported as IPFIX flow records.
	// perftest-a -> perftest-b (Ingress deny), perftest-d (Egress deny) -> perftest-a
	t.Run("IntraNodeDenyConnNP", func(t *testing.T) {
		np1, np2 := deployDenyNetworkPolicies(t, data, "perftest-b", "perftest-d", controlPlaneNodeName(), controlPlaneNodeName())
		defer func() {
			if np1 != nil {
				if err = data.deleteNetworkpolicy(np1); err != nil {
					t.Errorf("Error when deleting Network Policy: %v", err)
				}
			}
			if np2 != nil {
				if err = data.deleteNetworkpolicy(np2); err != nil {
					t.Errorf("Error when deleting Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-d",
			dstPodName: "perftest-a",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podBIPs.ipv4.String(), podDIPs.ipv4.String(), podAIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, false)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podBIPs.ipv6.String(), podDIPs.ipv6.String(), podAIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, true, false)
		}
	})

	// InterNodeFlows tests the case, where Pods are deployed on different Nodes
	// and their flow information is exported as IPFIX flow records.
	// Antrea network policies are being tested here.
	t.Run("InterNodeFlows", func(t *testing.T) {
		anp1, anp2 := deployAntreaNetworkPolicies(t, data, "perftest-a", "perftest-c", controlPlaneNodeName(), workerNodeName(1))
		defer func() {
			if anp1 != nil {
				data.DeleteANP(testNamespace, anp1.Name)
			}
			if anp2 != nil {
				data.DeleteANP(testNamespace, anp2.Name)
			}
		}()
		if !isIPv6 {
			checkRecordsForFlows(t, data, podAIPs.ipv4.String(), podCIPs.ipv4.String(), isIPv6, false, false, false, true)
		} else {
			checkRecordsForFlows(t, data, podAIPs.ipv6.String(), podCIPs.ipv6.String(), isIPv6, false, false, false, true)
		}
	})

	// InterNodeDenyConnIngressANP tests the case, where Pods are deployed on different Nodes with an Antrea ingress deny policy rule
	// applied to destination Pod (one reject rule, one drop rule) and their flow information is exported as IPFIX flow records.
	// perftest-a -> perftest-c (Ingress reject), perftest-a -> perftest-e (Ingress drop)
	t.Run("InterNodeDenyConnIngressANP", func(t *testing.T) {
		anp1, anp2 := deployDenyAntreaNetworkPolicies(t, data, "perftest-a", "perftest-c", "perftest-e", controlPlaneNodeName(), workerNodeName(1), true)
		defer func() {
			if anp1 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp1); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
			if anp2 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp2); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-c",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-e",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podCIPs.ipv4.String(), podAIPs.ipv4.String(), podEIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, true)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podCIPs.ipv6.String(), podAIPs.ipv6.String(), podEIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, true)
		}
	})

	// InterNodeDenyConnEgressANP tests the case, where Pods are deployed on different Nodes with an Antrea egress deny policy rule
	// applied to source Pod (one reject rule, one drop rule) and their flow information is exported as IPFIX flow records.
	// perftest-a (Egress reject) -> perftest-c, perftest-a (Egress drop)-> perftest-e
	t.Run("InterNodeDenyConnEgressANP", func(t *testing.T) {
		anp1, anp2 := deployDenyAntreaNetworkPolicies(t, data, "perftest-a", "perftest-c", "perftest-e", controlPlaneNodeName(), workerNodeName(1), false)
		defer func() {
			if anp1 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp1); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
			if anp2 != nil {
				if err = data.deleteAntreaNetworkpolicy(anp2); err != nil {
					t.Errorf("Error when deleting Antrea Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-c",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-e",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podCIPs.ipv4.String(), podAIPs.ipv4.String(), podEIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, true)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podCIPs.ipv6.String(), podAIPs.ipv6.String(), podEIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, true)
		}
	})

	// InterNodeDenyConnNP tests the case, where Pods are deployed on different Nodes with an ingress and an egress deny policy rule
	// applied to one destination Pod, one source Pod, respectively and their flow information is exported as IPFIX flow records.
	// perftest-a -> perftest-c (Ingress deny), perftest-b (Egress deny) -> perftest-e
	t.Run("InterNodeDenyConnNP", func(t *testing.T) {
		np1, np2 := deployDenyNetworkPolicies(t, data, "perftest-c", "perftest-b", workerNodeName(1), controlPlaneNodeName())
		defer func() {
			if np1 != nil {
				if err = data.deleteNetworkpolicy(np1); err != nil {
					t.Errorf("Error when deleting Network Policy: %v", err)
				}
			}
			if np2 != nil {
				if err = data.deleteNetworkpolicy(np2); err != nil {
					t.Errorf("Error when deleting Network Policy: %v", err)
				}
			}
		}()
		testFlow1 := testFlow{
			srcPodName: "perftest-a",
			dstPodName: "perftest-c",
		}
		testFlow2 := testFlow{
			srcPodName: "perftest-b",
			dstPodName: "perftest-e",
		}
		if !isIPv6 {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv4.String(), podCIPs.ipv4.String(), podBIPs.ipv4.String(), podEIPs.ipv4.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, false)
		} else {
			testFlow1.srcIP, testFlow1.dstIP, testFlow2.srcIP, testFlow2.dstIP = podAIPs.ipv6.String(), podCIPs.ipv6.String(), podBIPs.ipv6.String(), podEIPs.ipv6.String()
			checkRecordsForDenyFlows(t, data, testFlow1, testFlow2, isIPv6, false, false)
		}
	})

	// ToExternalFlows tests the export of IPFIX flow records when a source Pod
	// sends traffic to an external IP
	t.Run("ToExternalFlows", func(t *testing.T) {
		// Creating an agnhost server as a host network Pod
		serverPodPort := int32(80)
		_, serverIPs, cleanupFunc := createAndWaitForPod(t, data, func(name string, ns string, nodeName string, hostNetwork bool) error {
			return data.createServerPod(name, testNamespace, "", serverPodPort, false, true)
		}, "test-server-", "", testNamespace, false)
		defer cleanupFunc()

		clientName, clientIPs, cleanupFunc := createAndWaitForPod(t, data, data.createBusyboxPodOnNode, "test-client-", nodeName(0), testNamespace, false)
		defer cleanupFunc()

		if !isIPv6 {
			toExternalServerIP = serverIPs.ipv4.String()
		} else {
			toExternalServerIP = serverIPs.ipv6.String()
		}
		toExternalClientName = clientName

		if !isIPv6 {
			if clientIPs.ipv4 != nil && serverIPs.ipv4 != nil {
				checkRecordsForToExternalFlows(t, data, nodeName(0), clientName, clientIPs.ipv4.String(), serverIPs.ipv4.String(), serverPodPort, isIPv6)
			}
		} else {
			if clientIPs.ipv6 != nil && serverIPs.ipv6 != nil {
				checkRecordsForToExternalFlows(t, data, nodeName(0), clientName, clientIPs.ipv6.String(), serverIPs.ipv6.String(), serverPodPort, isIPv6)
			}
		}
	})

	// LocalServiceAccess tests the case, where Pod and Service are deployed on the same Node and their flow information is exported as IPFIX flow records.
	t.Run("LocalServiceAccess", func(t *testing.T) {
		// In dual stack cluster, Service IP can be assigned as different IP family from specified.
		// In that case, source IP and destination IP will align with IP family of Service IP.
		// For IPv4-only and IPv6-only cluster, IP family of Service IP will be same as Pod IPs.
		isServiceIPv6 := net.ParseIP(svcB.Spec.ClusterIP).To4() == nil
		if isServiceIPv6 {
			checkRecordsForFlows(t, data, podAIPs.ipv6.String(), svcB.Spec.ClusterIP, isServiceIPv6, true, true, false, false)
		} else {
			checkRecordsForFlows(t, data, podAIPs.ipv4.String(), svcB.Spec.ClusterIP, isServiceIPv6, true, true, false, false)
		}
	})

	// RemoteServiceAccess tests the case, where Pod and Service are deployed on different Nodes and their flow information is exported as IPFIX flow records.
	t.Run("RemoteServiceAccess", func(t *testing.T) {
		// In dual stack cluster, Service IP can be assigned as different IP family from specified.
		// In that case, source IP and destination IP will align with IP family of Service IP.
		// For IPv4-only and IPv6-only cluster, IP family of Service IP will be same as Pod IPs.
		isServiceIPv6 := net.ParseIP(svcC.Spec.ClusterIP).To4() == nil
		if isServiceIPv6 {
			checkRecordsForFlows(t, data, podAIPs.ipv6.String(), svcC.Spec.ClusterIP, isServiceIPv6, false, true, false, false)
		} else {
			checkRecordsForFlows(t, data, podAIPs.ipv4.String(), svcC.Spec.ClusterIP, isServiceIPv6, false, true, false, false)
		}
	})

	// Grafana tests the queries in self-defined dashboards can be successfully
	// executed, and the returned data contains expected results.
	t.Run("Grafana", func(t *testing.T) {
		for _, tc := range grafanaTestCases {
			checkGrafanaQueryResults(t, data, tc.dashboardName, tc.dashboardUid, &tc.queryList)
		}
	})

	// ClickHouseMonitor tests ensure ClickHouse monitor inspects the ClickHouse
	// Pod storage usage and deletes the data when the stoage usage grows above
	// the threshold.
	t.Run("ClickHouseMonitor", func(t *testing.T) {
		var flow testFlow
		if !isIPv6 {
			flow = testFlow{
				srcIP:      podAIPs.ipv4.String(),
				dstIP:      podBIPs.ipv4.String(),
				srcPodName: "perftest-a",
				dstPodName: "perftest-b",
			}
		} else {
			flow = testFlow{
				srcIP:      podAIPs.ipv6.String(),
				dstIP:      podBIPs.ipv6.String(),
				srcPodName: "perftest-a",
				dstPodName: "perftest-b",
			}
		}
		checkClickHouseMonitor(t, data, isIPv6, flow)
	})
}

func checkGrafanaQueryResults(t *testing.T, data *TestData, dashboardName, dashboardUid string, queryList *[]query) {
	queries, err := getQueriesByDashboard("/api/dashboards/uid/", dashboardUid, http.MethodGet, queryList)
	if err != nil {
		t.Errorf("Error when getting queries from dashboard: %s\nuid:%s, err: %v", dashboardName, dashboardUid, err)
		failOnError(err, t, data)
	}
	err = checkQueryResult(t, "/api/ds/query", http.MethodPost, queries, queryList)
	if err != nil {
		t.Errorf("Error when checking query results from dashboard: %s\nuid:%s, err: %v", dashboardName, dashboardUid, err)
		failOnError(err, t, data)
	}
}

func createNewHttpRequest(httpMethod, url string, body io.Reader) (*http.Request, error) {
	encodedCreds := base64.StdEncoding.EncodeToString([]byte(grafanaCred))
	req, err := http.NewRequest(httpMethod, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	return req, nil
}

func getQueriesByDashboard(apiEndpoint, dashboardUid, httpMethod string, queryList *[]query) ([]string, error) {
	queries := []string{}
	url := grafanaAddr + apiEndpoint + dashboardUid
	req, err := createNewHttpRequest(httpMethod, url, nil)
	if err != nil {
		return queries, fmt.Errorf("error when creating http request: %v", err)
	}
	client := &http.Client{}
	var resp *http.Response
	var respErr error
	err = wait.PollImmediate(500*time.Millisecond, grafanaQueryTimeout, func() (bool, error) {
		resp, respErr = client.Do(req)
		// If connection refused, keep trying
		if errors.Is(respErr, syscall.ECONNREFUSED) {
			return false, respErr
		}
		return true, nil
	})
	if err != nil {
		return queries, fmt.Errorf("unable to access Grafana Service, err: %v", err)
	}
	if respErr != nil {
		return queries, fmt.Errorf("error on response, err: %v", respErr)
	}
	defer resp.Body.Close()

	dashboard, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return queries, fmt.Errorf("error while reading the response bytes: %v", err)
	}
	for _, q := range *queryList {
		query := gjson.GetBytes(dashboard, fmt.Sprintf("dashboard.panels*.%d.targets", q.queryId))
		queryWithTimeRange := fmt.Sprintf(`{"queries":%s,"from":"now-15m","to":"now"}`, query.Raw)
		queries = append(queries, queryWithTimeRange)
	}
	return queries, nil
}

func checkQueryResult(t *testing.T, apiEndpoint, httpMethod string, queries []string, queryList *[]query) error {
	url := grafanaAddr + apiEndpoint
	for i, q := range *queryList {
		req, err := createNewHttpRequest(httpMethod, url, bytes.NewBuffer([]byte(queries[i])))
		if err != nil {
			return fmt.Errorf("error when creating http request: %v", err)
		}
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error on response, err: %v", err)
		}
		defer resp.Body.Close()

		result, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error while reading the response bytes: %v", err)
		}
		data := gjson.GetBytes(result, "results.A.frames*.0").Raw
		assert := assert.New(t)
		for _, str := range q.expectResult {
			assert.Containsf(data, str, "Panel name: %s", q.panelName)
		}
	}
	return nil
}

func checkClickHouseMonitor(t *testing.T, data *TestData, isIPv6 bool, flow testFlow) {
	checkClickHouseMonitorLogs(t, data, false, 0)
	var cmdStr string
	// iperf3 has a limit on maximum parallel streams at 128
	if !isIPv6 {
		cmdStr = fmt.Sprintf("iperf3 -u -c %s -P 128 -n 1", flow.dstIP)
	} else {
		cmdStr = fmt.Sprintf("iperf3 -u -6 -c %s -P 128 -n 1", flow.dstIP)
	}
	log.Infof("Generating flow records to exceed monitor threshold...")
	for i := 0; i < monitorIperfRounds; i++ {
		stdout, stderr, err := data.RunCommandFromPod(testNamespace, flow.srcPodName, "perftool", []string{"bash", "-c", cmdStr})
		require.NoErrorf(t, err, "Error when running iPerf3 client: %v,\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}
	log.Infof("Waiting for the flows to be exported...")
	time.Sleep(30 * time.Second)
	// Get the number of records in database before the monitor deletes the records
	stdout, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", "clickhouse client -q \"SELECT COUNT() FROM default.flows\""})
	require.NoErrorf(t, err, "Error when querying ClickHouse server: %v,\nstdout:%s\nstderr:%s", err, stdout, stderr)
	numRecord, err := strconv.ParseInt(strings.TrimSuffix(stdout, "\n"), 10, 64)
	require.NoErrorf(t, err, "Failed when parsing the number of records %v", err)
	log.Infof("Waiting for the monitor to detect and clean up the ClickHouse storage")
	time.Sleep(2 * time.Minute)
	checkClickHouseMonitorLogs(t, data, true, numRecord)
}

func checkClickHouseMonitorLogs(t *testing.T, data *TestData, deleted bool, numRecord int64) {
	logString, err := data.GetPodLogs(flowVisibilityNamespace, clickHousePodName,
		&corev1.PodLogOptions{
			Container: "clickhouse-monitor",
		},
	)
	require.NoErrorf(t, err, "Error when getting ClickHouse monitor logs: %v", err)
	// ClickHouse monitor log sample
	// "Memory usage" total=2062296555 used=42475 percentage=2.0595970980516912e-05
	logs := strings.Split(logString, "Memory usage")
	memoryUsageLog := strings.Split(logs[len(logs)-1], "\n")[0]
	assert.Contains(t, memoryUsageLog, "total=")
	assert.Contains(t, memoryUsageLog, "used=")
	assert.Contains(t, memoryUsageLog, "percentage=")
	percentage, err := strconv.ParseFloat(strings.Split(memoryUsageLog, "percentage=")[1], 64)
	require.NoErrorf(t, err, "Failed when parsing the memory usage percentage %v", err)
	if !deleted {
		assert.LessOrEqual(t, percentage, monitorThreshold)
	} else {
		assert.Greater(t, percentage, monitorThreshold)
		// Monitor deletes records from table flows and related MVs
		assert.Contains(t, logString, "ALTER TABLE default.flows DELETE WHERE timeInserted < toDateTime", "Monitor should delete records from Table flows")
		assert.Contains(t, logString, "ALTER TABLE default.flows_pod_view DELETE WHERE timeInserted < toDateTime", "Monitor should delete records from View flows_pod_view")
		assert.Contains(t, logString, "ALTER TABLE default.flows_node_view DELETE WHERE timeInserted < toDateTime", "Monitor should delete records from View flows_node_view")
		assert.Contains(t, logString, "ALTER TABLE default.flows_policy_view DELETE WHERE timeInserted < toDateTime", "Monitor should delete records from View flows_policy_view")
		assert.Contains(t, logString, "Skip rounds after a successful deletion", "Monitor should skip rounds after a successful deletion")
		require.Contains(t, logString, "SELECT timeInserted FROM default.flows LIMIT 1 OFFSET ", "Monitor should log the deletion SQL command")
		deletedRecordLog := strings.Split(logString, "SELECT timeInserted FROM default.flows LIMIT 1 OFFSET ")[1]
		deletedRecordLog = strings.Split(deletedRecordLog, "\n")[0]
		numDeletedRecord, err := strconv.ParseInt(deletedRecordLog, 10, 64)
		require.NoErrorf(t, err, "Failed when parsing the number of deleted records %v", err)

		assert.InDeltaf(t, numDeletedRecord, float64(numRecord)*monitorDeletePercentage, float64(numDeletedRecord)*0.15, "Difference between expected and actual number of deleted Records should be lower than 15%")
	}
}

func checkRecordsForFlows(t *testing.T, data *TestData, srcIP string, dstIP string, isIPv6 bool, isIntraNode bool, checkService bool, checkK8sNetworkPolicy bool, checkAntreaNetworkPolicy bool) {
	var cmdStr string
	if !isIPv6 {
		cmdStr = fmt.Sprintf("iperf3 -c %s -t %d -b %s", dstIP, iperfTimeSec, iperfBandwidth)
	} else {
		cmdStr = fmt.Sprintf("iperf3 -6 -c %s -t %d -b %s", dstIP, iperfTimeSec, iperfBandwidth)
	}
	stdout, _, err := data.RunCommandFromPod(testNamespace, "perftest-a", "perftool", []string{"bash", "-c", cmdStr})
	require.NoErrorf(t, err, "Error when running iperf3 client: %v", err)
	bwSlice, srcPort, _ := getBandwidthAndPorts(stdout)
	require.Equal(t, 2, len(bwSlice), "bandwidth value and / or bandwidth unit are not available")
	// bandwidth from iperf output
	bandwidthInFloat, err := strconv.ParseFloat(bwSlice[0], 64)
	require.NoErrorf(t, err, "Error when converting iperf bandwidth %s to float64 type", bwSlice[0])
	var bandwidthInMbps float64
	if strings.Contains(bwSlice[1], "Mbits") {
		bandwidthInMbps = bandwidthInFloat
	} else {
		failOnError(fmt.Errorf("unit of the traffic bandwidth reported by iperf should be Mbits."), t, data)
	}

	checkRecordsForFlowsClickHouse(t, data, srcIP, dstIP, srcPort, isIntraNode, checkService, checkK8sNetworkPolicy, checkAntreaNetworkPolicy, bandwidthInMbps)
}

func checkRecordsForFlowsClickHouse(t *testing.T, data *TestData, srcIP, dstIP, srcPort string, isIntraNode, checkService, checkK8sNetworkPolicy, checkAntreaNetworkPolicy bool, bandwidthInMbps float64) {
	// Check the source port along with source and destination IPs as there
	// are flow records for control flows during the iperf with same IPs
	// and destination port.
	clickHouseRecords := getClickHouseOutput(t, data, srcIP, dstIP, srcPort, checkService, true)

	for _, record := range clickHouseRecords {
		// Check if record has both Pod name of source and destination Pod.
		if isIntraNode {
			checkPodAndNodeDataClickHouse(t, record, "perftest-a", controlPlaneNodeName(), "perftest-b", controlPlaneNodeName())
			checkFlowTypeClickHouse(t, record, ipfixregistry.FlowTypeIntraNode)
		} else {
			checkPodAndNodeDataClickHouse(t, record, "perftest-a", controlPlaneNodeName(), "perftest-c", workerNodeName(1))
			checkFlowTypeClickHouse(t, record, ipfixregistry.FlowTypeInterNode)
		}
		assert := assert.New(t)
		if checkService {
			if isIntraNode {
				assert.Contains(record.DestinationServicePortName, "antrea-test/perftest-b", "Record with ServiceIP does not have Service name")
			} else {
				assert.Contains(record.DestinationServicePortName, "antrea-test/perftest-c", "Record with ServiceIP does not have Service name")
			}
		}
		if checkK8sNetworkPolicy {
			// Check if records have both ingress and egress network policies.
			assert.Equal(record.IngressNetworkPolicyName, ingressAllowNetworkPolicyName, "Record does not have the correct NetworkPolicy name with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyNamespace, testNamespace, "Record does not have the correct NetworkPolicy Namespace with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyType, ipfixregistry.PolicyTypeK8sNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the ingress rule")
			assert.Equal(record.EgressNetworkPolicyName, egressAllowNetworkPolicyName, "Record does not have the correct NetworkPolicy name with the egress rule")
			assert.Equal(record.EgressNetworkPolicyNamespace, testNamespace, "Record does not have the correct NetworkPolicy Namespace with the egress rule")
			assert.Equal(record.EgressNetworkPolicyType, ipfixregistry.PolicyTypeK8sNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the egress rule")
		}
		if checkAntreaNetworkPolicy {
			// Check if records have both ingress and egress network policies.
			assert.Equal(record.IngressNetworkPolicyName, ingressAntreaNetworkPolicyName, "Record does not have the correct NetworkPolicy name with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyNamespace, testNamespace, "Record does not have the correct NetworkPolicy Namespace with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyRuleName, testIngressRuleName, "Record does not have the correct NetworkPolicy RuleName with the ingress rule")
			assert.Equal(record.IngressNetworkPolicyRuleAction, ipfixregistry.NetworkPolicyRuleActionAllow, "Record does not have the correct NetworkPolicy RuleAction with the ingress rule")
			assert.Equal(record.EgressNetworkPolicyName, egressAntreaNetworkPolicyName, "Record does not have the correct NetworkPolicy name with the egress rule")
			assert.Equal(record.EgressNetworkPolicyNamespace, testNamespace, "Record does not have the correct NetworkPolicy Namespace with the egress rule")
			assert.Equal(record.EgressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the egress rule")
			assert.Equal(record.EgressNetworkPolicyRuleName, testEgressRuleName, "Record does not have the correct NetworkPolicy RuleName with the egress rule")
			assert.Equal(record.EgressNetworkPolicyRuleAction, ipfixregistry.NetworkPolicyRuleActionAllow, "Record does not have the correct NetworkPolicy RuleAction with the egress rule")
		}

		// Skip the bandwidth check for the iperf control flow records which have 0 throughput.
		if record.Throughput > 0 {
			flowStartTime := record.FlowStartSeconds.Unix()
			exportTime := record.FlowEndSeconds.Unix()
			var recBandwidth float64
			// flowEndReason == 3 means the end of flow detected
			if exportTime >= flowStartTime+iperfTimeSec || record.FlowEndReason == 3 {
				octetTotalCount := record.OctetTotalCount
				recBandwidth = float64(octetTotalCount) * 8 / float64(exportTime-flowStartTime) / 1000000
			} else {
				// Check bandwidth with the field "throughput" except for the last record,
				// as their throughput may be significantly lower than the average Iperf throughput.
				throughput := record.Throughput
				recBandwidth = float64(throughput) / 1000000
			}
			t.Logf("Throughput check on record with flowEndSeconds-flowStartSeconds: %v, Iperf throughput: %.2f Mbits/s, ClickHouse record throughput: %.2f Mbits/s", exportTime-flowStartTime, bandwidthInMbps, recBandwidth)
			assert.InDeltaf(recBandwidth, bandwidthInMbps, bandwidthInMbps*0.15, "Difference between Iperf bandwidth and ClickHouse record bandwidth should be lower than 15%%, record: %v", record)
		}

	}
	// Checking only data records as data records cannot be decoded without template record.
	assert.GreaterOrEqualf(t, len(clickHouseRecords), expectedNumDataRecords, "ClickHouse should receive expected number of flow records. Considered records: %s", clickHouseRecords)
}

func checkRecordsForToExternalFlows(t *testing.T, data *TestData, srcNodeName string, srcPodName string, srcIP string, dstIP string, dstPort int32, isIPv6 bool) {
	var cmd string
	if !isIPv6 {
		cmd = fmt.Sprintf("wget -O- %s:%d", dstIP, dstPort)
	} else {
		cmd = fmt.Sprintf("wget -O- [%s]:%d", dstIP, dstPort)
	}
	stdout, stderr, err := data.RunCommandFromPod(testNamespace, srcPodName, busyboxContainerName, strings.Fields(cmd))
	require.NoErrorf(t, err, "Error when running wget command, stdout: %s, stderr: %s", stdout, stderr)

	clickHouseRecords := getClickHouseOutput(t, data, srcIP, dstIP, "", false, false)
	for _, record := range clickHouseRecords {
		checkPodAndNodeDataClickHouse(t, record, srcPodName, srcNodeName, "", "")
		checkFlowTypeClickHouse(t, record, ipfixregistry.FlowTypeToExternal)
		// Since the OVS userspace conntrack implementation doesn't maintain
		// packet or byte counter statistics, skip the check for Kind clusters
		if testOptions.providerName != "kind" {
			assert.Greater(t, record.OctetDeltaCount, uint64(0), "octetDeltaCount should be non-zero")
		}
	}
}

func checkRecordsForDenyFlows(t *testing.T, data *TestData, testFlow1, testFlow2 testFlow, isIPv6, isIntraNode, isANP bool) {
	var cmdStr1, cmdStr2 string
	if !isIPv6 {
		cmdStr1 = fmt.Sprintf("iperf3 -c %s -n 1", testFlow1.dstIP)
		cmdStr2 = fmt.Sprintf("iperf3 -c %s -n 1", testFlow2.dstIP)
	} else {
		cmdStr1 = fmt.Sprintf("iperf3 -6 -c %s -n 1", testFlow1.dstIP)
		cmdStr2 = fmt.Sprintf("iperf3 -6 -c %s -n 1", testFlow2.dstIP)
	}
	_, _, err := data.RunCommandFromPod(testNamespace, testFlow1.srcPodName, "", []string{"timeout", "2", "bash", "-c", cmdStr1})
	assert.Error(t, err)
	_, _, err = data.RunCommandFromPod(testNamespace, testFlow2.srcPodName, "", []string{"timeout", "2", "bash", "-c", cmdStr2})
	assert.Error(t, err)

	checkRecordsForDenyFlowsClickHouse(t, data, testFlow1, testFlow2, isIPv6, isIntraNode, isANP)
}

func checkRecordsForDenyFlowsClickHouse(t *testing.T, data *TestData, testFlow1, testFlow2 testFlow, isIPv6, isIntraNode, isANP bool) {
	clickHouseRecords1 := getClickHouseOutput(t, data, testFlow1.srcIP, testFlow1.dstIP, "", false, false)
	clickHouseRecords2 := getClickHouseOutput(t, data, testFlow2.srcIP, testFlow2.dstIP, "", false, false)
	recordSlices := append(clickHouseRecords1, clickHouseRecords2...)
	// Iterate over recordSlices and build some results to test with expected results
	for _, record := range recordSlices {
		var srcPodName, dstPodName string
		if record.SourceIP == testFlow1.srcIP && record.DestinationIP == testFlow1.dstIP {
			srcPodName = testFlow1.srcPodName
			dstPodName = testFlow1.dstPodName
		} else if record.SourceIP == testFlow2.srcIP && record.DestinationIP == testFlow2.dstIP {
			srcPodName = testFlow2.srcPodName
			dstPodName = testFlow2.dstPodName
		}

		if isIntraNode {
			checkPodAndNodeDataClickHouse(t, record, srcPodName, controlPlaneNodeName(), dstPodName, controlPlaneNodeName())
			checkFlowTypeClickHouse(t, record, ipfixregistry.FlowTypeIntraNode)
		} else {
			checkPodAndNodeDataClickHouse(t, record, srcPodName, controlPlaneNodeName(), dstPodName, workerNodeName(1))
			checkFlowTypeClickHouse(t, record, ipfixregistry.FlowTypeInterNode)
		}
		assert := assert.New(t)
		if !isANP { // K8s Network Policies
			if (record.IngressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionDrop) && (record.IngressNetworkPolicyName != ingressDropANPName) {
				assert.Equal(record.DestinationIP, testFlow1.dstIP)
			} else if (record.EgressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionDrop) && (record.EgressNetworkPolicyName != egressDropANPName) {
				assert.Equal(record.DestinationIP, testFlow2.dstIP)
			}
		} else { // Antrea Network Policies
			if record.IngressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionReject {
				assert.Equal(record.IngressNetworkPolicyName, ingressRejectANPName, "Record does not have Antrea NetworkPolicy name with ingress reject rule")
				assert.Equal(record.IngressNetworkPolicyNamespace, testNamespace, "Record does not have correct ingressNetworkPolicyNamespace")
				assert.Equal(record.IngressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the ingress reject rule")
				assert.Equal(record.IngressNetworkPolicyRuleName, testIngressRuleName, "Record does not have the correct NetworkPolicy RuleName with the ingress reject rule")
			} else if record.IngressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionDrop {
				assert.Equal(record.IngressNetworkPolicyName, ingressDropANPName, "Record does not have Antrea NetworkPolicy name with ingress drop rule")
				assert.Equal(record.IngressNetworkPolicyNamespace, testNamespace, "Record does not have correct ingressNetworkPolicyNamespace")
				assert.Equal(record.IngressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the ingress drop rule")
				assert.Equal(record.IngressNetworkPolicyRuleName, testIngressRuleName, "Record does not have the correct NetworkPolicy RuleName with the ingress drop rule")
			} else if record.EgressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionReject {
				assert.Equal(record.EgressNetworkPolicyName, egressRejectANPName, "Record does not have Antrea NetworkPolicy name with egress reject rule")
				assert.Equal(record.EgressNetworkPolicyNamespace, testNamespace, "Record does not have correct egressNetworkPolicyNamespace")
				assert.Equal(record.EgressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the egress reject rule")
				assert.Equal(record.EgressNetworkPolicyRuleName, testEgressRuleName, "Record does not have the correct NetworkPolicy RuleName with the egress reject rule")
			} else if record.EgressNetworkPolicyRuleAction == ipfixregistry.NetworkPolicyRuleActionDrop {
				assert.Equal(record.EgressNetworkPolicyName, egressDropANPName, "Record does not have Antrea NetworkPolicy name with egress drop rule")
				assert.Equal(record.EgressNetworkPolicyNamespace, testNamespace, "Record does not have correct egressNetworkPolicyNamespace")
				assert.Equal(record.EgressNetworkPolicyType, ipfixregistry.PolicyTypeAntreaNetworkPolicy, "Record does not have the correct NetworkPolicy Type with the egress drop rule")
				assert.Equal(record.EgressNetworkPolicyRuleName, testEgressRuleName, "Record does not have the correct NetworkPolicy RuleName with the egress drop rule")
			}
		}
	}
}

func checkPodAndNodeDataClickHouse(t *testing.T, record *ClickHouseFullRow, srcPod, srcNode, dstPod, dstNode string) {
	assert := assert.New(t)
	assert.Equal(record.SourcePodName, srcPod, "Record with srcIP does not have Pod name: %s", srcPod)
	assert.Equal(record.SourcePodNamespace, testNamespace, "Record does not have correct sourcePodNamespace: %s", testNamespace)
	assert.Equal(record.SourceNodeName, srcNode, "Record does not have correct sourceNodeName: %s", srcNode)
	// For Pod-To-External flow type, we send traffic to an external address,
	// so we skip the verification of destination Pod info.
	// Also, source Pod labels are different for Pod-To-External flow test.
	if dstPod != "" {
		assert.Equal(record.DestinationPodName, dstPod, "Record with dstIP does not have Pod name: %s", dstPod)
		assert.Equal(record.DestinationPodNamespace, testNamespace, "Record does not have correct destinationPodNamespace: %s", testNamespace)
		assert.Equal(record.DestinationNodeName, dstNode, "Record does not have correct destinationNodeName: %s", dstNode)
		assert.Equal(record.SourcePodLabels, fmt.Sprintf("{\"antrea-e2e\":\"%s\",\"app\":\"perftool\"}", srcPod), "Record does not have correct label for source Pod")
		assert.Equal(record.DestinationPodLabels, fmt.Sprintf("{\"antrea-e2e\":\"%s\",\"app\":\"perftool\"}", dstPod), "Record does not have correct label for destination Pod")
	} else {
		assert.Equal(record.SourcePodLabels, fmt.Sprintf("{\"antrea-e2e\":\"%s\",\"app\":\"busybox\"}", srcPod), "Record does not have correct label for source Pod")
	}
}

func checkFlowTypeClickHouse(t *testing.T, record *ClickHouseFullRow, flowType uint8) {
	assert.Equal(t, record.FlowType, flowType, "Record does not have correct flowType")
}

// getClickHouseOutput queries clickhouse with built-in client and checks if we have
// received all the expected records for a given flow with source IP, destination IP
// and source port. We send source port to ignore the control flows during the iperf test.
// Polling timeout is coded assuming IPFIX output has been checked first.
func getClickHouseOutput(t *testing.T, data *TestData, srcIP, dstIP, srcPort string, isDstService, checkAllRecords bool) []*ClickHouseFullRow {
	var flowRecords []*ClickHouseFullRow
	var queryOutput string

	query := fmt.Sprintf("SELECT * FROM flows WHERE (sourceIP = '%s') AND (destinationIP = '%s')", srcIP, dstIP)
	if isDstService {
		query = fmt.Sprintf("SELECT * FROM flows WHERE (sourceIP = '%s') AND (destinationClusterIP = '%s')", srcIP, dstIP)
	}
	if len(srcPort) > 0 {
		query = fmt.Sprintf("%s AND (sourceTransportPort = %s)", query, srcPort)
	}
	cmd := []string{
		"clickhouse-client",
		"--date_time_output_format=iso",
		"--format=JSONEachRow",
		fmt.Sprintf("--query=%s", query),
	}
	// Waiting additional 4x commit interval to be adequate for 3 commit attempts.
	timeout := (exporterActiveFlowExportTimeout + aggregatorActiveFlowRecordTimeout*2 + aggregatorClickHouseCommitInterval*4) * 2
	err := wait.PollImmediate(500*time.Millisecond, timeout, func() (bool, error) {
		queryOutput, _, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", cmd)
		if err != nil {
			return false, err
		}

		rows := strings.Split(queryOutput, "\n")
		flowRecords = make([]*ClickHouseFullRow, 0, len(rows))
		for _, row := range rows {
			row = strings.TrimSpace(row)
			if len(row) == 0 {
				continue
			}
			flowRecord := ClickHouseFullRow{}
			err = json.Unmarshal([]byte(row), &flowRecord)
			if err != nil {
				return false, err
			}
			flowRecords = append(flowRecords, &flowRecord)
		}

		if checkAllRecords {
			for _, record := range flowRecords {
				flowStartTime := record.FlowStartSeconds.Unix()
				exportTime := record.FlowEndSeconds.Unix()
				// flowEndReason == 3 means the end of flow detected
				if exportTime >= flowStartTime+iperfTimeSec || record.FlowEndReason == 3 {
					return true, nil
				}
			}
			return false, nil
		}
		return len(flowRecords) > 0, nil
	})
	require.NoErrorf(t, err, "ClickHouse did not receive the expected records in query output: %v; query: %s", queryOutput, query)
	return flowRecords
}

func deployK8sNetworkPolicies(t *testing.T, data *TestData, srcPod, dstPod string) (np1 *networkingv1.NetworkPolicy, np2 *networkingv1.NetworkPolicy) {
	// Add K8s NetworkPolicy between two iperf Pods.
	var err error
	np1, err = data.createNetworkPolicy(ingressAllowNetworkPolicyName, &networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		Ingress: []networkingv1.NetworkPolicyIngressRule{{
			From: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"antrea-e2e": srcPod,
					},
				}},
			},
		}},
	})
	if err != nil {
		t.Errorf("Error when creating Network Policy: %v", err)
	}
	np2, err = data.createNetworkPolicy(egressAllowNetworkPolicyName, &networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		Egress: []networkingv1.NetworkPolicyEgressRule{{
			To: []networkingv1.NetworkPolicyPeer{{
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"antrea-e2e": dstPod,
					},
				}},
			},
		}},
	})
	if err != nil {
		t.Errorf("Error when creating Network Policy: %v", err)
	}
	// Wait for network policies to be realized.
	if err := data.WaitNetworkPolicyRealize(controlPlaneNodeName(), openflow.IngressRuleTable, ingressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for ingress Network Policy to be realized: %v", err)
	}
	if err := data.WaitNetworkPolicyRealize(controlPlaneNodeName(), openflow.IngressRuleTable, egressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for egress Network Policy to be realized: %v", err)
	}
	t.Log("Network Policies are realized.")
	return np1, np2
}

func deployAntreaNetworkPolicies(t *testing.T, data *TestData, srcPod, dstPod, srcNode, dstNode string) (anp1 *secv1alpha1.NetworkPolicy, anp2 *secv1alpha1.NetworkPolicy) {
	builder1 := &utils.AntreaNetworkPolicySpecBuilder{}
	// apply anp to dstPod, allow ingress from srcPod
	builder1 = builder1.SetName(testNamespace, ingressAntreaNetworkPolicyName).
		SetPriority(2.0).
		SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": dstPod}}})
	builder1 = builder1.AddIngress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": srcPod}, map[string]string{},
		nil, nil, nil, secv1alpha1.RuleActionAllow, testIngressRuleName)
	anp1 = builder1.Get()
	anp1, err1 := data.CreateOrUpdateANP(anp1)
	if err1 != nil {
		failOnError(fmt.Errorf("error when creating Antrea Network Policy: %v", err1), t, data)
	}

	builder2 := &utils.AntreaNetworkPolicySpecBuilder{}
	// apply anp to srcPod, allow egress to dstPod
	builder2 = builder2.SetName(testNamespace, egressAntreaNetworkPolicyName).
		SetPriority(2.0).
		SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": srcPod}}})
	builder2 = builder2.AddEgress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": dstPod}, map[string]string{},
		nil, nil, nil, secv1alpha1.RuleActionAllow, testEgressRuleName)
	anp2 = builder2.Get()
	anp2, err2 := data.CreateOrUpdateANP(anp2)
	if err2 != nil {
		failOnError(fmt.Errorf("error when creating Network Policy: %v", err2), t, data)
	}

	// Wait for network policies to be realized.
	if err := data.WaitNetworkPolicyRealize(dstNode, openflow.AntreaPolicyIngressRuleTable, antreaIngressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for Antrea ingress Network Policy to be realized: %v", err)
	}
	if err := data.WaitNetworkPolicyRealize(srcNode, openflow.AntreaPolicyEgressRuleTable, antreaEgressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for Antrea egress Network Policy to be realized: %v", err)
	}
	t.Log("Antrea Network Policies are realized.")
	return anp1, anp2
}

func deployDenyAntreaNetworkPolicies(t *testing.T, data *TestData, srcPod, podReject, podDrop string, srcNode, dstNode string, isIngress bool) (anp1 *secv1alpha1.NetworkPolicy, anp2 *secv1alpha1.NetworkPolicy) {
	var err error
	builder1 := &utils.AntreaNetworkPolicySpecBuilder{}
	builder2 := &utils.AntreaNetworkPolicySpecBuilder{}
	var table *openflow.Table
	var flowCount int
	var nodeName string
	if isIngress {
		// apply reject and drop ingress rule to destination pods
		builder1 = builder1.SetName(testNamespace, ingressRejectANPName).
			SetPriority(2.0).
			SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": podReject}}})
		builder1 = builder1.AddIngress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": srcPod}, map[string]string{},
			nil, nil, nil, secv1alpha1.RuleActionReject, testIngressRuleName)
		builder2 = builder2.SetName(testNamespace, ingressDropANPName).
			SetPriority(2.0).
			SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": podDrop}}})
		builder2 = builder2.AddIngress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": srcPod}, map[string]string{},
			nil, nil, nil, secv1alpha1.RuleActionDrop, testIngressRuleName)
		table = openflow.AntreaPolicyIngressRuleTable
		flowCount = antreaIngressTableInitFlowCount + 2
		nodeName = dstNode
	} else {
		// apply reject and drop egress rule to source pod
		builder1 = builder1.SetName(testNamespace, egressRejectANPName).
			SetPriority(2.0).
			SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": srcPod}}})
		builder1 = builder1.AddEgress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": podReject}, map[string]string{},
			nil, nil, nil, secv1alpha1.RuleActionReject, testEgressRuleName)
		builder2 = builder2.SetName(testNamespace, egressDropANPName).
			SetPriority(2.0).
			SetAppliedToGroup([]utils.ANPAppliedToSpec{{PodSelector: map[string]string{"antrea-e2e": srcPod}}})
		builder2 = builder2.AddEgress(corev1.ProtocolTCP, nil, nil, nil, nil, map[string]string{"antrea-e2e": podDrop}, map[string]string{},
			nil, nil, nil, secv1alpha1.RuleActionDrop, testEgressRuleName)
		table = openflow.AntreaPolicyEgressRuleTable
		flowCount = antreaEgressTableInitFlowCount + 2
		nodeName = srcNode
	}
	anp1 = builder1.Get()
	anp1, err = data.CreateOrUpdateANP(anp1)
	if err != nil {
		failOnError(fmt.Errorf("error when creating Antrea Network Policy: %v", err), t, data)
	}
	anp2 = builder2.Get()
	anp2, err = data.CreateOrUpdateANP(anp2)
	if err != nil {
		failOnError(fmt.Errorf("error when creating Antrea Network Policy: %v", err), t, data)
	}
	// Wait for Antrea NetworkPolicy to be realized.
	if err := data.WaitNetworkPolicyRealize(nodeName, table, flowCount); err != nil {
		t.Errorf("Error when waiting for Antrea Network Policy to be realized: %v", err)
	}
	t.Log("Antrea Network Policies are realized.")
	return anp1, anp2
}

func deployDenyNetworkPolicies(t *testing.T, data *TestData, pod1, pod2 string, node1, node2 string) (np1 *networkingv1.NetworkPolicy, np2 *networkingv1.NetworkPolicy) {
	np1, err := data.createNetworkPolicy(ingressDenyNPName, &networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"antrea-e2e": pod1,
			},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
		Ingress:     []networkingv1.NetworkPolicyIngressRule{},
	})
	if err != nil {
		t.Errorf("Error when creating Network Policy: %v", err)
	}
	np2, err = data.createNetworkPolicy(egressDenyNPName, &networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"antrea-e2e": pod2,
			},
		},
		PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		Egress:      []networkingv1.NetworkPolicyEgressRule{},
	})
	if err != nil {
		t.Errorf("Error when creating Network Policy: %v", err)
	}
	// Wait for NetworkPolicy to be realized.
	if err := data.WaitNetworkPolicyRealize(node1, openflow.IngressRuleTable, ingressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for ingress Network Policies to be realized: %v", err)
	}
	if err := data.WaitNetworkPolicyRealize(node2, openflow.EgressRuleTable, egressTableInitFlowCount+1); err != nil {
		t.Errorf("Error when waiting for egress Network Policies to be realized: %v", err)
	}
	t.Log("Network Policies are realized.")
	return np1, np2
}

func createPerftestPods(data *TestData) (podAIPs *PodIPs, podBIPs *PodIPs, podCIPs *PodIPs, podDIPs *PodIPs, podEIPs *PodIPs, err error) {
	if err := data.createPodOnNode("perftest-a", testNamespace, controlPlaneNodeName(), perftoolImage, nil, nil, nil, nil, false, nil); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when creating the perftest client Pod: %v", err)
	}
	podAIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-a", testNamespace)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when waiting for the perftest client Pod: %v", err)
	}

	if err := data.createPodOnNode("perftest-b", testNamespace, controlPlaneNodeName(), perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podBIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-b", testNamespace)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}

	if err := data.createPodOnNode("perftest-c", testNamespace, workerNodeName(1), perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podCIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-c", testNamespace)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}

	if err := data.createPodOnNode("perftest-d", testNamespace, controlPlaneNodeName(), perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podDIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-d", testNamespace)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}

	if err := data.createPodOnNode("perftest-e", testNamespace, workerNodeName(1), perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podEIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-e", testNamespace)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}

	return podAIPs, podBIPs, podCIPs, podDIPs, podEIPs, nil
}

func createPerftestServices(data *TestData, isIPv6 bool) (svcB *corev1.Service, svcC *corev1.Service, err error) {
	svcIPFamily := corev1.IPv4Protocol
	if isIPv6 {
		svcIPFamily = corev1.IPv6Protocol
	}

	svcB, err = data.CreateService("perftest-b", testNamespace, iperfPort, iperfPort, map[string]string{"antrea-e2e": "perftest-b"}, false, false, corev1.ServiceTypeClusterIP, &svcIPFamily)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating perftest-b Service: %v", err)
	}

	svcC, err = data.CreateService("perftest-c", testNamespace, iperfPort, iperfPort, map[string]string{"antrea-e2e": "perftest-c"}, false, false, corev1.ServiceTypeClusterIP, &svcIPFamily)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating perftest-c Service: %v", err)
	}

	return svcB, svcC, nil
}

// getBandwidthAndPorts parses iperf commands output and returns bandwidth,
// source port and destination port. Bandwidth is returned as a slice containing
// two strings (bandwidth value and bandwidth unit).
func getBandwidthAndPorts(iperfStdout string) ([]string, string, string) {
	var bandwidth []string
	var srcPort, dstPort string
	outputLines := strings.Split(iperfStdout, "\n")
	for _, line := range outputLines {
		if strings.Contains(line, "sender") {
			fields := strings.Fields(line)
			bandwidth = fields[6:8]
		}
		if strings.Contains(line, "connected") {
			fields := strings.Fields(line)
			srcPort = fields[5]
			dstPort = fields[10]
		}
	}
	return bandwidth, srcPort, dstPort
}

type ClickHouseFullRow struct {
	TimeInserted                         time.Time `json:"timeInserted"`
	FlowStartSeconds                     time.Time `json:"flowStartSeconds"`
	FlowEndSeconds                       time.Time `json:"flowEndSeconds"`
	FlowEndSecondsFromSourceNode         time.Time `json:"flowEndSecondsFromSourceNode"`
	FlowEndSecondsFromDestinationNode    time.Time `json:"flowEndSecondsFromDestinationNode"`
	FlowEndReason                        uint8     `json:"flowEndReason"`
	SourceIP                             string    `json:"sourceIP"`
	DestinationIP                        string    `json:"destinationIP"`
	SourceTransportPort                  uint16    `json:"sourceTransportPort"`
	DestinationTransportPort             uint16    `json:"destinationTransportPort"`
	ProtocolIdentifier                   uint8     `json:"protocolIdentifier"`
	PacketTotalCount                     uint64    `json:"packetTotalCount,string"`
	OctetTotalCount                      uint64    `json:"octetTotalCount,string"`
	PacketDeltaCount                     uint64    `json:"packetDeltaCount,string"`
	OctetDeltaCount                      uint64    `json:"octetDeltaCount,string"`
	ReversePacketTotalCount              uint64    `json:"reversePacketTotalCount,string"`
	ReverseOctetTotalCount               uint64    `json:"reverseOctetTotalCount,string"`
	ReversePacketDeltaCount              uint64    `json:"reversePacketDeltaCount,string"`
	ReverseOctetDeltaCount               uint64    `json:"reverseOctetDeltaCount,string"`
	SourcePodName                        string    `json:"sourcePodName"`
	SourcePodNamespace                   string    `json:"sourcePodNamespace"`
	SourceNodeName                       string    `json:"sourceNodeName"`
	DestinationPodName                   string    `json:"destinationPodName"`
	DestinationPodNamespace              string    `json:"destinationPodNamespace"`
	DestinationNodeName                  string    `json:"destinationNodeName"`
	DestinationClusterIP                 string    `json:"destinationClusterIP"`
	DestinationServicePort               uint16    `json:"destinationServicePort"`
	DestinationServicePortName           string    `json:"destinationServicePortName"`
	IngressNetworkPolicyName             string    `json:"ingressNetworkPolicyName"`
	IngressNetworkPolicyNamespace        string    `json:"ingressNetworkPolicyNamespace"`
	IngressNetworkPolicyRuleName         string    `json:"ingressNetworkPolicyRuleName"`
	IngressNetworkPolicyRuleAction       uint8     `json:"ingressNetworkPolicyRuleAction"`
	IngressNetworkPolicyType             uint8     `json:"ingressNetworkPolicyType"`
	EgressNetworkPolicyName              string    `json:"egressNetworkPolicyName"`
	EgressNetworkPolicyNamespace         string    `json:"egressNetworkPolicyNamespace"`
	EgressNetworkPolicyRuleName          string    `json:"egressNetworkPolicyRuleName"`
	EgressNetworkPolicyRuleAction        uint8     `json:"egressNetworkPolicyRuleAction"`
	EgressNetworkPolicyType              uint8     `json:"egressNetworkPolicyType"`
	TcpState                             string    `json:"tcpState"`
	FlowType                             uint8     `json:"flowType"`
	SourcePodLabels                      string    `json:"sourcePodLabels"`
	DestinationPodLabels                 string    `json:"destinationPodLabels"`
	Throughput                           uint64    `json:"throughput,string"`
	ReverseThroughput                    uint64    `json:"reverseThroughput,string"`
	ThroughputFromSourceNode             uint64    `json:"throughputFromSourceNode,string"`
	ThroughputFromDestinationNode        uint64    `json:"throughputFromDestinationNode,string"`
	ReverseThroughputFromSourceNode      uint64    `json:"reverseThroughputFromSourceNode,string"`
	ReverseThroughputFromDestinationNode uint64    `json:"reverseThroughputFromDestinationNode,string"`
	Trusted                              uint8     `json:"trusted"`
}

func failOnError(err error, t *testing.T, data *TestData) {
	if portForwardCmd.Process != nil {
		portForwardCmd.Process.Kill()
	}
	flowVisibilityCleanup(t, data, false)
	t.Fatalf("test failed: %v", err)
}
