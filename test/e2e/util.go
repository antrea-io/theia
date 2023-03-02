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
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"
)

const (
	nameSuffixLength               int = 8
	ingressAllowNetworkPolicyName      = "test-flow-aggregator-networkpolicy-ingress-allow"
	ingressRejectANPName               = "test-flow-aggregator-anp-ingress-reject"
	ingressDropANPName                 = "test-flow-aggregator-anp-ingress-drop"
	ingressDenyNPName                  = "test-flow-aggregator-np-ingress-deny"
	egressAllowNetworkPolicyName       = "test-flow-aggregator-networkpolicy-egress-allow"
	egressRejectANPName                = "test-flow-aggregator-anp-egress-reject"
	egressDropANPName                  = "test-flow-aggregator-anp-egress-drop"
	egressDenyNPName                   = "test-flow-aggregator-np-egress-deny"
	ingressAntreaNetworkPolicyName     = "test-flow-aggregator-antrea-networkpolicy-ingress"
	egressAntreaNetworkPolicyName      = "test-flow-aggregator-antrea-networkpolicy-egress"
	testIngressRuleName                = "test-ingress-rule-name"
	testEgressRuleName                 = "test-egress-rule-name"
	homeDashboardUid                   = "Yw6zwRkVk"
	flowRecordsDashboardUid            = "t1UGX7t7k"
	podToPodDashboardUid               = "Yxn0Ghh7k"
	podToServiceDashboardUid           = "LGdxbW17z"
	podToExternalDashboardUid          = "K9SPrnJ7k"
	nodeToNodeDashboardUid             = "1F56RJh7z"
	networkPolicyDashboardUid          = "KJNMOwQnk"
	networkTopologyDashboardUid        = "yRVDEad4k"
)

var (
	toExternalServerIP   = ""
	toExternalClientName = ""
	// A DNS-1123 subdomain must consist of lower case alphanumeric characters
	lettersAndDigits = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
)

func createDirectory(path string) error {
	return os.Mkdir(path, 0700)
}

// IsDirEmpty checks whether a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		// #nosec G404: random number generator not used for security purposes
		randIdx := rand.Intn(len(lettersAndDigits))
		b[i] = lettersAndDigits[randIdx]
	}
	return string(b)
}

func randName(prefix string) string {
	return prefix + randSeq(nameSuffixLength)
}

type query struct {
	queryId      int
	panelName    string
	expectResult []string
}

var grafanaTestCases = []struct {
	dashboardName string
	dashboardUid  string
	queryList     []query
}{
	{
		dashboardName: "homepage",
		dashboardUid:  homeDashboardUid,
		queryList: []query{
			{
				queryId:      11,
				panelName:    "Number of NetworkPolicies",
				expectResult: []string{"16"},
			},
			{
				queryId:      15,
				panelName:    "Top 10 Active Source Pods",
				expectResult: []string{"antrea-test/perftest-a"},
			},
		},
	},
	{
		dashboardName: "flow_records_dashboard",
		dashboardUid:  flowRecordsDashboardUid,
		queryList: []query{
			{
				queryId:      2,
				panelName:    "Flow Records Table",
				expectResult: []string{"perftest-a", "perftest-b", "perftest-c", "perftest-d", "perftest-e"},
			},
		},
	},
	{
		dashboardName: "pod_to_pod_dashboard",
		dashboardUid:  podToPodDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Cumulative Bytes of Pod-to-Pod",
				expectResult: []string{"perftest-a", "perftest-b", "perftest-c", "perftest-d", "perftest-e"},
			},
			{
				queryId:      1,
				panelName:    "Cumulative Reverse Bytes of Pod-to-Pod",
				expectResult: []string{"perftest-a", "perftest-b", "perftest-c"},
			},
			{
				queryId:      2,
				panelName:    "Throughput of Pod-to-Pod",
				expectResult: []string{"perftest-a", "perftest-b", "perftest-c", "perftest-d", "perftest-e"},
			},
			{
				queryId:      3,
				panelName:    "Reverse Throughput of Pod-to-Pod",
				expectResult: []string{"perftest-a", "perftest-b", "perftest-c"},
			},
			{
				queryId:      4,
				panelName:    "Throughput of Pod as Source",
				expectResult: []string{"perftest-a", "perftest-b"},
			},
			{
				queryId:      5,
				panelName:    "Cumulative Bytes of Source Pod Namespace",
				expectResult: []string{testNamespace},
			},
			{
				queryId:      6,
				panelName:    "Throughput of Pod as Destination",
				expectResult: []string{"perftest-b", "perftest-c"},
			},
			{
				queryId:      7,
				panelName:    "Cumulative Bytes of Destination Pod Namespace",
				expectResult: []string{testNamespace},
			},
		},
	},
	{
		dashboardName: "pod_to_service_dashboard",
		dashboardUid:  podToServiceDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Cumulative Bytes Pod-to-Service",
				expectResult: []string{"perftest-a", "antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
			{
				queryId:      1,
				panelName:    "Cumulative Reverse Bytes Pod-to-Service",
				expectResult: []string{"perftest-a", "antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
			{
				queryId:      2,
				panelName:    "Throughput of Pod-to-Service",
				expectResult: []string{"perftest-a", "antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
			{
				queryId:      3,
				panelName:    "Reverse Throughput of Pod-to-Service",
				expectResult: []string{"perftest-a", "antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
			{
				queryId:      4,
				panelName:    "Throughput of Pod as Source",
				expectResult: []string{"perftest-a"},
			},
			{
				queryId:      5,
				panelName:    "Throughput of Service as Destination",
				expectResult: []string{"antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
		},
	},
	{
		dashboardName: "pod_to_external_dashboard",
		dashboardUid:  podToExternalDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Cumulative Bytes of Pod-to-External",
				expectResult: []string{toExternalClientName, toExternalServerIP},
			},
			{
				queryId:      1,
				panelName:    "Cumulative Reverse Bytes of Pod-to-External",
				expectResult: []string{toExternalClientName, toExternalServerIP},
			},
			{
				queryId:      2,
				panelName:    "Throughput of Pod-to-External",
				expectResult: []string{toExternalClientName, toExternalServerIP},
			},
			{
				queryId:      3,
				panelName:    "Reverse Throughput of Pod-to-External",
				expectResult: []string{toExternalClientName, toExternalServerIP},
			},
		},
	},
	{
		dashboardName: "node_to_node_dashboard",
		dashboardUid:  nodeToNodeDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Cumulative Bytes of Node-to-Node",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
			{
				queryId:      1,
				panelName:    "Cumulative Reverse Bytes of Node-to-Node",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
			{
				queryId:      2,
				panelName:    "Throughput of Node-to-Node",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
			{
				queryId:      3,
				panelName:    "Reverse Throughput of Node-to-Node",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
			{
				queryId:      4,
				panelName:    "Throughput of Node as Source",
				expectResult: []string{controlPlaneNodeName()},
			},
			{
				queryId:      5,
				panelName:    "Cumulative Bytes of Node as Source",
				expectResult: []string{controlPlaneNodeName()},
			},
			{
				queryId:      6,
				panelName:    "Throughput of Node as Destination",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
			{
				queryId:      7,
				panelName:    "Cumulative Bytes of Node as Destination",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1)},
			},
		},
	},
	{
		dashboardName: "networkpolicy_dashboard",
		dashboardUid:  networkPolicyDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Cumulative Bytes of Flows with NetworkPolicy Information",
				expectResult: []string{ingressAllowNetworkPolicyName, egressAllowNetworkPolicyName, ingressAntreaNetworkPolicyName, egressAntreaNetworkPolicyName, ingressRejectANPName, ingressDropANPName, egressRejectANPName, egressDropANPName},
			},
			{
				queryId:      1,
				panelName:    "Cumulative Bytes of Ingress Network Policy",
				expectResult: []string{ingressAllowNetworkPolicyName, ingressAntreaNetworkPolicyName, ingressRejectANPName, ingressDropANPName},
			},
			{
				queryId:      2,
				panelName:    "Cumulative Bytes of Egress Network Policy",
				expectResult: []string{egressAllowNetworkPolicyName, egressAntreaNetworkPolicyName, egressRejectANPName, egressDropANPName},
			},
			{
				queryId:      3,
				panelName:    "Throughput of Ingress Allow NetworkPolicy",
				expectResult: []string{ingressAllowNetworkPolicyName, ingressAntreaNetworkPolicyName},
			},
			{
				queryId:      4,
				panelName:    "Throughput of Egress Allow NetworkPolicy",
				expectResult: []string{egressAllowNetworkPolicyName, egressAntreaNetworkPolicyName},
			},
			{
				queryId:      5,
				panelName:    "Throughput of Ingress Deny NetworkPolicy",
				expectResult: []string{ingressRejectANPName, ingressDropANPName},
			},
			{
				queryId:      6,
				panelName:    "Throughput of Egress Deny NetworkPolicy",
				expectResult: []string{egressRejectANPName, egressDropANPName},
			},
		},
	},
	{
		dashboardName: "network_topology_dashboard",
		dashboardUid:  networkTopologyDashboardUid,
		queryList: []query{
			{
				queryId:      0,
				panelName:    "Network Topology",
				expectResult: []string{controlPlaneNodeName(), workerNodeName(1), "perftest-a", "perftest-b", "perftest-c", "perftest-d", "perftest-e", "antrea-test/perftest-b", "antrea-test/perftest-c"},
			},
		},
	},
}

func getNetowrkPolicyYaml(kind string) string {
	switch kind {
	case "acnp":
		return `
apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-allow-acnp-flow-aggregator-5p4xy
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: flow-aggregator
  egress:
  - action: Allow
    to:
    - podSelector: {}
  ingress:
  - action: Allow
    from:
    - podSelector: {}
  priority: 5
  tier: Platform
`
	case "acg":
		return `
apiVersion: crd.antrea.io/v1alpha2
kind: ClusterGroup
metadata:
  name: antrea-test-perftest-b
spec:
  serviceReference:
    name: perftest-b
    namespace: antrea-test
`
	case "knp":
		return `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: recommend-k8s-np-3da3f
  namespace: antrea-test
spec:
  podSelector:
    matchLabels:
      app: nginx
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: nginx
    ports:
    - protocol: TCP
      port: 80
  egress: []
`
	case "anp":
		return `
apiVersion: crd.antrea.io/v1alpha1
kind: NetworkPolicy
metadata:
  name: recommend-allow-anp-qw2nk
  namespace: antrea-test
spec:
  appliedTo:
  - podSelector:
      matchLabels:
        antrea-e2e: test-client-l1z7m9bx
        app: busybox
  egress:
  - action: Allow
    ports:
    - port: 80
      protocol: TCP
    to:
    - ipBlock:
        cidr: 172.18.0.3/32
  ingress: []
  priority: 5
  tier: Application
`
	default:
		return ""
	}
}

func RunJob(t *testing.T, data *TestData, jobcmd string) (stdout string, jobName string, err error) {
	cmd := "chmod +x ./theia"
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	rc, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), jobcmd)
	if err != nil || rc != 0 {
		return "", "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	stdout = strings.TrimSuffix(stdout, "\n")
	stdoutSlice := strings.Split(stdout, " ")
	jobName = stdoutSlice[len(stdoutSlice)-1]
	return stdout, jobName, nil
}

func GetJobStatus(t *testing.T, data *TestData, cmd string) (stdout string, err error) {
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func ListJobs(t *testing.T, data *TestData, cmd string) (stdout string, err error) {
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func DeleteJob(t *testing.T, data *TestData, cmd string) (stdout string, err error) {
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func RetrieveJobResult(t *testing.T, data *TestData, cmd string) (stdout string, err error) {
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}
