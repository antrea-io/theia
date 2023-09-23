// Copyright 2023 Antrea Authors.
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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"antrea.io/theia/test/e2e"
)

const (
	iperfTimeSec = 12
	// Set target bandwidth(bits/sec) of iPerf traffic to a relatively small value
	// (default unlimited for TCP), to reduce the variances caused by network performance
	// during 12s, and make the throughput test more stable.
	iperfBandwidth = "10m"
)

var expectedNumDataRecords = 3

func TestMultiCluster(t *testing.T) {
	testData, _, v6Enabled, err := setupTestForMultiCluster(t)
	if err != nil {
		t.Errorf("Error when setting up test: %v", err)
		failOnError(err, t, testData)
	}
	defer flowVisibilityCleanup(t, testData)
	eastClusterTD := testData.clusterTestDataMap[eastCluster]
	westClusterTD := testData.clusterTestDataMap[westCluster]
	t.Logf("Creating Perftest Pods")
	podAIPs, podBIPs, podCIPs, podDIPs, err := createPerftestPods(testData)
	if err != nil {
		failOnError(fmt.Errorf("error when creating perftest Pods: %v", err), t, testData)
	}
	t.Logf("Sending traffic in east cluster")
	var cmdStr string
	if !v6Enabled {
		cmdStr = fmt.Sprintf("iperf3 -c %s -t %d -b %s", podBIPs.GetIPv4(), iperfTimeSec, iperfBandwidth)
	} else {
		cmdStr = fmt.Sprintf("iperf3 -6 -c %s -t %d -b %s", podBIPs.GetIPv6(), iperfTimeSec, iperfBandwidth)
	}
	_, _, err = eastClusterTD.RunCommandFromPod(multiClusterTestNamespace, "perftest-a", "perftool", []string{"bash", "-c", cmdStr})
	require.NoErrorf(t, err, "Error when running iperf3 client: %v", err)
	var clickHouseRecordsE []*e2e.ClickHouseFullRow
	if !v6Enabled {
		clickHouseRecordsE = e2e.GetClickHouseOutput(t, eastClusterTD, podAIPs.GetIPv4().String(), podBIPs.GetIPv4().String(), "", false, true)
	} else {
		clickHouseRecordsE = e2e.GetClickHouseOutput(t, eastClusterTD, podAIPs.GetIPv6().String(), podBIPs.GetIPv6().String(), "", false, true)
	}
	t.Logf("Verifying records in ClickHouse")
	require.GreaterOrEqualf(t, len(clickHouseRecordsE), expectedNumDataRecords, "ClickHouse should receive expected number of flow records. Considered records: %v", clickHouseRecordsE)
	t.Logf("Sending traffic in west cluster")
	if !v6Enabled {
		cmdStr = fmt.Sprintf("iperf3 -c %s -t %d -b %s", podDIPs.GetIPv4(), iperfTimeSec, iperfBandwidth)
	} else {
		cmdStr = fmt.Sprintf("iperf3 -6 -c %s -t %d -b %s", podDIPs.GetIPv6(), iperfTimeSec, iperfBandwidth)
	}
	_, _, err = westClusterTD.RunCommandFromPod(multiClusterTestNamespace, "perftest-c", "perftool", []string{"bash", "-c", cmdStr})
	require.NoErrorf(t, err, "Error when running iperf3 client: %v", err)
	var clickHouseRecordsW []*e2e.ClickHouseFullRow
	if !v6Enabled {
		// We use eastClusterTD here because ClickHouse is deployed in the EAST cluster only. In order to get the records
		// from ClickHouse we need to use the clientset in the EAST cluster instead of the one in the WEST cluster.
		clickHouseRecordsW = e2e.GetClickHouseOutput(t, eastClusterTD, podCIPs.GetIPv4().String(), podDIPs.GetIPv4().String(), "", false, true)
	} else {
		clickHouseRecordsW = e2e.GetClickHouseOutput(t, eastClusterTD, podCIPs.GetIPv6().String(), podDIPs.GetIPv6().String(), "", false, true)
	}
	t.Logf("Verifying records in ClickHouse")
	require.GreaterOrEqualf(t, len(clickHouseRecordsW), expectedNumDataRecords, "ClickHouse should receive expected number of flow records. Considered records: %v", clickHouseRecordsW)
	t.Logf("Verifying cluster UUID in East/West cluster are different")
	require.NotEqualf(t, clickHouseRecordsE[0].ClusterUUID, clickHouseRecordsW[0].ClusterUUID, "ClusterUUID for EAST/WEST cluster should be different.\n Records of EAST cluster: %v\nRecords of EAST cluster: %v", clickHouseRecordsE, clickHouseRecordsW)
}

func createPerftestPods(data *MCTestData) (podAIPs *e2e.PodIPs, podBIPs *e2e.PodIPs, podCIPs *e2e.PodIPs, podDIPs *e2e.PodIPs, err error) {
	eastClusterTD := testData.clusterTestDataMap[eastCluster]
	westClusterTD := testData.clusterTestDataMap[westCluster]
	if err := data.createPodOnNode(eastClusterTD, "perftest-a", multiClusterTestNamespace, data.controlPlaneNames[eastCluster], perftoolImage, nil, nil, nil, nil, false, nil); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when creating the perftest client Pod: %v", err)
	}
	podAIPs, err = data.podWaitForIPs(defaultTimeout, eastCluster, "perftest-a", multiClusterTestNamespace)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when waiting for the perftest client Pod: %v", err)
	}
	if err := data.createPodOnNode(eastClusterTD, "perftest-b", multiClusterTestNamespace, data.controlPlaneNames[eastCluster], perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podBIPs, err = data.podWaitForIPs(defaultTimeout, eastCluster, "perftest-b", multiClusterTestNamespace)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}
	if err := data.createPodOnNode(westClusterTD, "perftest-c", multiClusterTestNamespace, data.controlPlaneNames[westCluster], perftoolImage, nil, nil, nil, nil, false, nil); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podCIPs, err = data.podWaitForIPs(defaultTimeout, westCluster, "perftest-c", multiClusterTestNamespace)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}
	if err := data.createPodOnNode(westClusterTD, "perftest-d", multiClusterTestNamespace, data.controlPlaneNames[westCluster], perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podDIPs, err = data.podWaitForIPs(defaultTimeout, westCluster, "perftest-d", multiClusterTestNamespace)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}
	return podAIPs, podBIPs, podCIPs, podDIPs, nil
}

func failOnError(err error, t *testing.T, data *MCTestData) {
	flowVisibilityCleanup(t, data)
	t.Fatalf("test failed: %v", err)
}
