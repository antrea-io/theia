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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	iperfTimeSec = 12
	// Set target bandwidth(bits/sec) of iPerf traffic to a relatively small value
	// (default unlimited for TCP), to reduce the variances caused by network performance
	// during 12s, and make the throughput test more stable.
	iperfBandwidth                     = "10m"
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
	id1                                = "abcd1234-abcd-1234-ab12-abcd12345678"
	id2                                = "1234abcd-1234-abcd-12ab-12345678abcd"
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
		randIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(lettersAndDigits))))
		b[i] = lettersAndDigits[randIdx.Int64()]
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

func getNetworkPolicyYaml(kind string) string {
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

func TheiaManagerRestart(t *testing.T, data *TestData, jobName1 string, job string) error {
	// Simulate the Theia Manager downtime
	cmd := "kubectl delete deployment theia-manager -n flow-visibility"
	_, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil {
		return fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}

	// Delete the first job during the Theia Manager downtime
	cmd = fmt.Sprintf("kubectl delete %s %s -n flow-visibility", job, jobName1)
	// Sometimes the deletion will fail with 'error: the server doesn't have a resource type "npr" or "tad"'
	// Retry under this condition
	err = wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		_, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if err == nil && stderr == "" {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}

	// Redeploy the Theia Manager
	err = data.deployFlowVisibilityCommon(clickHouseOperatorYML, flowVisibilityWithSparkYML)
	if err != nil {
		return err
	}
	// Sleep for a short period to make sure the previous deletion of the theia management ends
	time.Sleep(3 * time.Second)
	theiaManagerPodName, err := data.getPodByLabel(theiaManagerPodLabel, flowVisibilityNamespace)
	if err != nil {
		return err
	}
	err = data.PodWaitForReady(defaultTimeout, theiaManagerPodName, flowVisibilityNamespace)
	if err != nil {
		return fmt.Errorf("error when waiting for Theia Manager %s", theiaManagerPodName)
	}
	return nil
}

func VerifyJobCleaned(t *testing.T, data *TestData, jobName string, tablename string, prefixlen int) error {
	// Check the SparkApplication and database entries of jobName do not exist
	// Allow some time for Theia Manager to delete the stale resources
	var (
		queryOutput string
		stderr      string
		stdout      string
		err         error
	)
	err = wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		cmd := fmt.Sprintf("kubectl get sparkapplication %s -n flow-visibility", jobName)
		_, stdout, stderr, _ = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if !strings.Contains(stderr, fmt.Sprintf("sparkapplications.sparkoperator.k8s.io \"%s\" not found", jobName)) {
			// Keep trying
			return false, nil
		}
		cmd = fmt.Sprintf("clickhouse client -q \"SELECT COUNT() FROM %s WHERE id='%s'\"", tablename, jobName[prefixlen:])
		queryOutput, stderr, err = data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", cmd})
		require.NoErrorf(t, err, "fail to get %v from ClickHouse, stderr: %v", tablename, stderr)
		if queryOutput != "0\n" {
			// Keep trying
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("stale resources expected to be deleted, but got stdout: %s, stderr: %s, ClickHouse query result expected to be 0, got: %s", stdout, stderr, queryOutput)
	}
	return nil
}

func ApplyNewVersion(t *testing.T, data *TestData, antreaYML, chOperatorYML, flowVisibilityYML string) {
	t.Logf("Changing Antrea YAML to %s", antreaYML)
	// Do not wait for agent rollout as its updateStrategy is set to OnDelete for upgrade test.
	if err := data.deployAntreaCommon(antreaYML, "", false); err != nil {
		t.Fatalf("Error upgrading Antrea: %v", err)
	}
	t.Logf("Restarting all Antrea DaemonSet Pods")
	if err := data.restartAntreaAgentPods(defaultTimeout); err != nil {
		t.Fatalf("Error when restarting Antrea: %v", err)
	}

	t.Logf("Changing ClickHouse Operator YAML to %s,\nFlow Visibility YAML to %s", chOperatorYML, flowVisibilityYML)
	if err := data.deployFlowVisibilityCommon(chOperatorYML, flowVisibilityYML); err != nil {
		t.Fatalf("Error upgrading Flow Visibility: %v", err)
	}
	t.Logf("Waiting for the ClickHouse Pod restarting")
	if err := data.waitForClickHousePod(); err != nil {
		t.Fatalf("Error when waiting for the ClickHouse Pod restarting: %v", err)
	}
}

func CreateFlowVisibilitySetUpConfig(sparkOperator, grafana, clickHouseLocalPv, flowAggregator, flowAggregatorOnly bool) FlowVisibilitySetUpConfig {
	return FlowVisibilitySetUpConfig{
		withSparkOperator:     sparkOperator,
		withGrafana:           grafana,
		withClickHouseLocalPv: clickHouseLocalPv,
		withFlowAggregator:    flowAggregator,
		flowAggregatorOnly:    flowAggregatorOnly,
	}
}

func CopyCovFolder(nodeName, covDir, covPrefix string) error {
	covDirAbs, err := filepath.Abs("../../" + covDir)
	if err != nil {
		return fmt.Errorf("error while creating absolute file path: %v", err)
	}
	pathOnNode := nodeName + ":" + "/var/log/" + covPrefix + "-coverage/."
	cmd := exec.Command("docker", "cp", pathOnNode, covDirAbs)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		errStr := errb.String()
		outStr := outb.String()
		if !strings.Contains(errb.String(), "Could not find the file") {
			return fmt.Errorf("error while running docker cp command[%v] from node: %s: stdout: %s, stderr: %s", cmd, nodeName, outStr, errStr)
		}
	}
	return nil
}

func ClearCovFolder(nodeName, covPrefix string) error {
	nestedCmd := "`rm -rf /var/log/" + covPrefix + "-coverage/*`"
	cmd := exec.Command("docker", "exec", nodeName, "sh", "-c", nestedCmd)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		errStr := errb.String()
		outStr := outb.String()
		if !strings.Contains(errb.String(), "not found") {
			return fmt.Errorf("error while running docker exec command[%v] from node: %s: stdout: %s, stderr: %s", cmd, nodeName, outStr, errStr)
		}
	}
	return nil
}

// GetClickHouseOutput queries clickhouse with built-in client and checks if we have
// received all the expected records for a given flow with source IP, destination IP
// and source port. We send source port to ignore the control flows during the iperf test.
// Polling timeout is coded assuming IPFIX output has been checked first.
func GetClickHouseOutput(t *testing.T, data *TestData, srcIP, dstIP, srcPort string, isDstService, checkAllRecords bool) []*ClickHouseFullRow {
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
