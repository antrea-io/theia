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
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Use a long timeout as it takes ~500s to complete a single Spark job on
	// Kind testbed
	jobCompleteTimeout = 10 * time.Minute
	jobSubmitTimeout   = 2 * time.Minute
	jobFailedTimeout   = 2 * time.Minute
	startCmd           = "./theia policy-recommendation run"
	statusCmd          = "./theia policy-recommendation status"
	listCmd            = "./theia policy-recommendation list"
	deleteCmd          = "./theia policy-recommendation delete"
	retrieveCmd        = "./theia policy-recommendation retrieve"
	serverPodPort      = int32(80)
)

func TestPolicyRecommendation(t *testing.T) {
	config := FlowVisibiltiySetUpConfig{
		withSparkOperator:     true,
		withGrafana:           false,
		withClickHouseLocalPv: false,
		withFlowAggregator:    true,
	}
	data, v4Enabled, v6Enabled, err := setupTestForFlowVisibility(t, config)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer func() {
		teardownTest(t, data)
		deleteRecommendedPolicies(t, data)
		teardownFlowVisibility(t, data, config)
	}()

	t.Run("testPolicyRecommendationRun", func(t *testing.T) {
		testPolicyRecommendationRun(t, data)
	})

	t.Run("testPolicyRecommendationStatus", func(t *testing.T) {
		testPolicyRecommendationStatus(t, data)
	})

	t.Run("testPolicyRecommendationList", func(t *testing.T) {
		testPolicyRecommendationList(t, data)
	})

	t.Run("testPolicyRecommendationDelete", func(t *testing.T) {
		testPolicyRecommendationDelete(t, data)
	})

	t.Run("testPolicyRecommendationFailed", func(t *testing.T) {
		testPolicyRecommendationFailed(t, data)
	})

	podAIPs, podBIPs, err := createTestPods(data)
	if err != nil {
		t.Fatalf("Error when creating test Pods: %v", err)
	}

	svcB, err := createTestService(data, v6Enabled)
	if err != nil {
		t.Fatalf("Error when creating perftest-b Service: %v", err)
	}
	// In dual stack cluster, Service IP can be assigned as different IP family from specified.
	// In that case, source IP and destination IP will align with IP family of Service IP.
	// For IPv4-only and IPv6-only cluster, IP family of Service IP will be same as Pod IPs.
	isServiceIPv6 := net.ParseIP(svcB.Spec.ClusterIP).To4() == nil

	// Creating an agnhost server as a host network Pod
	_, serverIPs, cleanupFunc := createAndWaitForPod(t, data, func(name string, ns string, nodeName string, hostNetwork bool) error {
		return data.createServerPod(name, testNamespace, "", serverPodPort, false, true)
	}, "test-server-", "", testNamespace, false)
	defer cleanupFunc()

	clientName, clientIPs, cleanupFunc := createAndWaitForPod(t, data, data.createBusyboxPodOnNode, "test-client-", nodeName(0), testNamespace, false)
	defer cleanupFunc()

	isExternalFlowIPv4 := clientIPs.ipv4 != nil && serverIPs.ipv4 != nil
	isExternalFlowIPv6 := clientIPs.ipv6 != nil && serverIPs.ipv6 != nil

	if v4Enabled {
		srcIP := podAIPs.ipv4.String()
		dstIP := podBIPs.ipv4.String()
		testFlowPodToPod := testFlow{
			srcIP:      srcIP,
			dstIP:      dstIP,
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		var testFlowPodToSvc testFlow
		if !isServiceIPv6 {
			testFlowPodToSvc = testFlow{
				srcIP:      srcIP,
				dstIP:      svcB.Spec.ClusterIP,
				srcPodName: "perftest-a",
				dstPodName: "perftest-b",
			}
		}
		var testFlowPodToExternal testFlow
		if isExternalFlowIPv4 {
			testFlowPodToExternal = testFlow{
				srcIP:      clientIPs.ipv4.String(),
				dstIP:      serverIPs.ipv4.String(),
				srcPodName: clientName,
				dstPodName: "",
			}
		}

		t.Run("testPolicyRecommendationRetrieve/IPv4", func(t *testing.T) {
			testPolicyRecommendationRetrieve(t, data, false, testFlowPodToPod, testFlowPodToSvc, testFlowPodToExternal)
		})
	}
	if v6Enabled {
		srcIP := podAIPs.ipv6.String()
		dstIP := podBIPs.ipv6.String()
		testFlowPodToPod := testFlow{
			srcIP:      srcIP,
			dstIP:      dstIP,
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		var testFlowPodToSvc testFlow
		if isServiceIPv6 {
			testFlowPodToSvc = testFlow{
				srcIP:      srcIP,
				dstIP:      svcB.Spec.ClusterIP,
				srcPodName: "perftest-a",
				dstPodName: "perftest-b",
			}
		}
		var testFlowPodToExternal testFlow
		if isExternalFlowIPv6 {
			testFlowPodToExternal = testFlow{
				srcIP:      clientIPs.ipv6.String(),
				dstIP:      serverIPs.ipv6.String(),
				srcPodName: clientName,
				dstPodName: "",
			}
		}
		t.Run("testPolicyRecommendationRetrieve/IPv6", func(t *testing.T) {
			testPolicyRecommendationRetrieve(t, data, true, testFlowPodToPod, testFlowPodToSvc, testFlowPodToExternal)
		})
	}

	t.Run("testTheiaManagerRestart", func(t *testing.T) {
		testTheiaManagerRestart(t, data)
	})
}

func testTheiaManagerRestart(t *testing.T, data *TestData) {
	_, jobName1, err := runJob(t, data)
	require.NoError(t, err)
	_, jobName2, err := runJob(t, data)
	require.NoError(t, err)

	// Simulate the Theia Manager downtime
	cmd := "kubectl delete deployment theia-manager -n flow-visibility"
	_, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	require.NoErrorf(t, err, fmt.Sprintf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr))

	// Delete the first job during the Theia Manager downtime
	cmd = fmt.Sprintf("kubectl delete npr %s -n flow-visibility", jobName1)
	// Sometimes the deletion will fail with 'error: the server doesn't have a resource type "npr"'
	// Retry under this condition
	err = wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		_, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if err == nil && stderr == "" {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	require.NoError(t, err, "error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)

	// Redeploy the Theia Manager
	err = data.deployFlowVisibilityCommon(flowVisibilityWithSparkYML)
	require.NoError(t, err)
	// Sleep for a short period to make sure the previous deletion of the theia management ends
	time.Sleep(3 * time.Second)
	theiaManagerPodName, err := data.getPodByLabel(theiaManagerPodLabel, flowVisibilityNamespace)
	require.NoError(t, err)
	err = data.podWaitForReady(defaultTimeout, theiaManagerPodName, flowVisibilityNamespace)
	require.NoError(t, err, "error when waiting for Theia Manager %s", theiaManagerPodName)

	// Check the status of jobName2
	stdout, err = getJobStatus(t, data, jobName2)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Status of this policy recommendation job is", "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName2+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = deleteJob(t, data, jobName2)
	require.NoError(t, err)

	// Check the SparkApplication and database entries of jobName1 do not exist
	// Allow some time for Theia Manager to delete the stale resources
	var queryOutput string
	err = wait.PollImmediate(defaultInterval, defaultTimeout, func() (bool, error) {
		cmd = fmt.Sprintf("kubectl get sparkapplication %s -n flow-visibility", jobName1)
		_, stdout, stderr, _ = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
		if !strings.Contains(stderr, fmt.Sprintf("sparkapplications.sparkoperator.k8s.io \"%s\" not found", jobName1)) {
			// Keep trying
			return false, nil
		}
		cmd = fmt.Sprintf("clickhouse client -q \"SELECT COUNT() FROM recommendations WHERE id='%s'\"", jobName1[3:])
		queryOutput, stderr, err = data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", cmd})
		require.NoErrorf(t, err, "fail to get recommendations from ClickHouse, stderr: %v", stderr)
		if queryOutput != "0\n" {
			// Keep trying
			return false, nil
		}
		return true, nil
	})
	require.NoErrorf(t, err, "stale resources expected to be deleted, but got stdout: %s, stderr: %s, ClickHouse query result expected to be 0, got: %s", stdout, stderr, queryOutput)
}

// Example output: Successfully created policy recommendation job with name pr-e998433e-accb-4888-9fc8-06563f073e86
func testPolicyRecommendationRun(t *testing.T, data *TestData) {
	stdout, jobName, err := runJob(t, data)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, fmt.Sprintf("Successfully created policy recommendation job with name %s", jobName), "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = deleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output: Status of this policy recommendation job is COMPLETED
func testPolicyRecommendationStatus(t *testing.T, data *TestData) {
	_, jobName, err := runJob(t, data)
	require.NoError(t, err)
	stdout, err := getJobStatus(t, data, jobName)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Status of this policy recommendation job is", "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = deleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output:
// CreationTime          CompletionTime        Name                                    Status
// 2022-06-17 15:03:24   N/A                   pr-615026a0-1856-4107-87d9-08f7d69819ae RUNNING
// 2022-06-17 15:03:22   2022-06-17 18:08:37   pr-7bebe4f9-408b-4dd8-9d63-9dc538073089 COMPLETED
// 2022-06-17 15:03:39   N/A                   pr-c7a9e768-559a-4bfb-b0c8-a0291b4c208c SUBMITTED
func testPolicyRecommendationList(t *testing.T, data *TestData) {
	_, jobName, err := runJob(t, data)
	require.NoError(t, err)
	stdout, err := listJobs(t, data)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "CreationTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "CompletionTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "Name", "stdout: %s", stdout)
	assert.Containsf(stdout, "Status", "stdout: %s", stdout)
	assert.Containsf(stdout, jobName, "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = deleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output: Successfully deleted policy recommendation job with name pr-e998433e-accb-4888-9fc8-06563f073e86
func testPolicyRecommendationDelete(t *testing.T, data *TestData) {
	_, jobName, err := runJob(t, data)
	require.NoError(t, err)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	stdout, err := deleteJob(t, data, jobName)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Successfully deleted policy recommendation job with name", "stdout: %s", stdout)
	stdout, err = listJobs(t, data)
	require.NoError(t, err)
	assert.NotContainsf(stdout, jobName, "Still found deleted job in list command stdout: %s", stdout)
}

// Example output:
// Status of this policy recommendation job is Failed
// error message: driver pod not found
// Or
// error message: driver container failed
func testPolicyRecommendationFailed(t *testing.T, data *TestData) {
	stdout, jobName, err := runJob(t, data)
	require.NoError(t, err)
	err = wait.PollImmediate(defaultInterval, jobSubmitTimeout, func() (bool, error) {
		stdout, err = getJobStatus(t, data, jobName)
		require.NoError(t, err)
		if strings.Contains(stdout, "Status of this policy recommendation job is RUNNING") {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	require.NoError(t, err)
	driverPodName := fmt.Sprintf("%s-driver", jobName)
	err = data.podWaitForReady(defaultTimeout, driverPodName, flowVisibilityNamespace)
	require.NoError(t, err)
	if err := data.DeletePod(flowVisibilityNamespace, driverPodName); err != nil {
		t.Logf("Error when deleting Driver Pod: %v", err)
	}
	err = wait.PollImmediate(defaultInterval, jobFailedTimeout, func() (bool, error) {
		stdout, err = getJobStatus(t, data, jobName)
		require.NoError(t, err)
		if strings.Contains(stdout, "Status of this policy recommendation job is FAILED") {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Truef(strings.Contains(stdout, "error message: driver pod not found") || strings.Contains(stdout, "error message: driver container failed") || strings.Contains(stdout, "error message: driver container status missing"), "stdout: %s", stdout)
	_, err = deleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output:
//
//	apiVersion: crd.antrea.io/v1alpha1
//	kind: NetworkPolicy
//	metadata:
//	  name: recommend-allow-anp-fj3hd
//
// ...
func testPolicyRecommendationRetrieve(t *testing.T, data *TestData, isIPv6 bool, testFlowPodToPod, testFlowPodToSvc, testFlowPodToExternal testFlow) {
	// With the workload traffic perftest-a -> perftest-b, perftest-a ->
	// perftest-svc-b, and test-client -> test-server, we expect the policy
	// recommendation job recommends 3 allow ANP, and 3 default deny ACNP.
	// Besides, there will always be 3 allow ACNP recommended for the
	// 'kube-system', 'flow-aggregator', and 'flow-visibility' Namespace.
	expectedAllowANPCnt := 3
	expectedAllowACNPCnt := 3
	expectedRejectANPCnt := 0
	expectedRejectACNPCnt := 3

	testFlows := []testFlow{testFlowPodToPod, testFlowPodToSvc}
	var cmdStr string
	for _, flow := range testFlows {
		if (flow != testFlow{}) {
			if !isIPv6 {
				cmdStr = fmt.Sprintf("iperf3 -c %s", flow.dstIP)
			} else {
				cmdStr = fmt.Sprintf("iperf3 -6 -c %s", flow.dstIP)
			}
			stdout, stderr, err := data.RunCommandFromPod(testNamespace, flow.srcPodName, "perftool", []string{"bash", "-c", cmdStr})
			require.NoErrorf(t, err, "Error when running iPerf3 client: %v,\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
	}

	if (testFlowPodToExternal != testFlow{}) {
		if !isIPv6 {
			cmdStr = fmt.Sprintf("wget -O- %s:%d", testFlowPodToExternal.dstIP, serverPodPort)
		} else {
			cmdStr = fmt.Sprintf("wget -O- [%s]:%d", testFlowPodToExternal.dstIP, serverPodPort)
		}
		stdout, stderr, err := data.RunCommandFromPod(testNamespace, testFlowPodToExternal.srcPodName, busyboxContainerName, strings.Fields(cmdStr))
		require.NoErrorf(t, err, "Error when running wget command, stdout: %s, stderr: %s", stdout, stderr)
	} else {
		expectedAllowANPCnt -= 1
		expectedRejectACNPCnt -= 1
	}

	_, jobName, err := runJob(t, data)
	require.NoError(t, err)
	err = waitJobComplete(t, data, jobName, jobCompleteTimeout)
	require.NoErrorf(t, err, "Policy recommendation Spark job failed to complete")

	// Apply the recommended policies, and check the results
	err = retrieveJobResult(t, data, jobName)
	require.NoError(t, err)
	cmd := fmt.Sprintf("kubectl apply -f %s", policyOutputYML)
	_, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	require.NoErrorf(t, err, "Error when running %v from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	_, allPolicies, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), fmt.Sprintf("cat %s", policyOutputYML))
	require.NoErrorf(t, err, "Error when running %v from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)

	// Check recommended ANP counts
	cmd = fmt.Sprintf("kubectl get anp -n %s", testNamespace)
	_, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	require.NoErrorf(t, err, "Error when running %v from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	outputLines := strings.Split(stdout, "\n")
	allowANPCnt := 0
	rejectANPCnt := 0
	for _, line := range outputLines {
		if strings.Contains(line, "recommend-allow") {
			allowANPCnt += 1
		}
		if strings.Contains(line, "recommend-reject") {
			rejectANPCnt += 1
		}
	}
	assert := assert.New(t)
	assert.Equalf(expectedAllowANPCnt, allowANPCnt, fmt.Sprintf("Expected allow ANP count is: %d. Actual count is: %d. Recommended policies:\n%s\nCheck command output:\n%s", expectedAllowANPCnt, allowANPCnt, allPolicies, stdout))
	assert.Equalf(expectedRejectANPCnt, rejectANPCnt, fmt.Sprintf("Expected reject ANP count is: %d. Actual count is: %d. Recommended policies:\n%s\nCheck command output:\n%s", expectedRejectANPCnt, rejectANPCnt, allPolicies, stdout))

	// Check recommended ACNP counts
	cmd = "kubectl get acnp"
	_, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	require.NoErrorf(t, err, "Error when running %v from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	outputLines = strings.Split(stdout, "\n")
	allowACNPCnt := 0
	rejectACNPCnt := 0
	for _, line := range outputLines {
		if strings.Contains(line, "recommend-allow") {
			allowACNPCnt += 1
		}
		if strings.Contains(line, "recommend-reject") {
			rejectACNPCnt += 1
		}
	}
	assert.Equalf(expectedAllowACNPCnt, allowACNPCnt, fmt.Sprintf("Expected allow ACNP count is: %d. Actual count is: %d. Recommended policies:\n%s\nCheck command output:\n%s", expectedAllowACNPCnt, allowACNPCnt, allPolicies, stdout))
	assert.Equalf(expectedRejectACNPCnt, rejectACNPCnt, fmt.Sprintf("Expected reject ACNP count is: %d. Actual count is: %d. Recommended policies:\n%s\nCheck command output:\n%s", expectedRejectACNPCnt, rejectACNPCnt, allPolicies, stdout))
}

func runJob(t *testing.T, data *TestData) (stdout string, jobName string, err error) {
	stdout, jobName, err = RunJob(t, data, startCmd)
	if err != nil {
		return "", "", err
	}
	return stdout, jobName, nil
}

func getJobStatus(t *testing.T, data *TestData, jobName string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", statusCmd, jobName)
	stdout, err = GetJobStatus(t, data, cmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func listJobs(t *testing.T, data *TestData) (stdout string, err error) {
	stdout, err = ListJobs(t, data, listCmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func deleteJob(t *testing.T, data *TestData, jobName string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", deleteCmd, jobName)
	stdout, err = DeleteJob(t, data, cmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func retrieveJobResult(t *testing.T, data *TestData, jobName string) error {
	cmd := fmt.Sprintf("%s %s -f %s", retrieveCmd, jobName, policyOutputYML)
	_, err := RetrieveJobResult(t, data, cmd)
	if err != nil {
		return err
	}
	return nil
}

// waitJobComplete waits for the policy recommendation Spark job completes
func waitJobComplete(t *testing.T, data *TestData, jobName string, timeout time.Duration) error {
	stdout := ""
	err := wait.PollImmediate(defaultInterval, timeout, func() (bool, error) {
		stdout, err := getJobStatus(t, data, jobName)
		require.NoError(t, err)
		if strings.Contains(stdout, "Status of this policy recommendation job is COMPLETED") {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("policy recommendation Spark job not completed after %v\nstatus:%s", timeout, stdout)
	} else if err != nil {
		return err
	}
	return nil
}

func createTestPods(data *TestData) (podAIPs *PodIPs, podBIPs *PodIPs, err error) {
	if err := data.createPodOnNode("perftest-a", testNamespace, controlPlaneNodeName(), perftoolImage, nil, nil, nil, nil, false, nil); err != nil {
		return nil, nil, fmt.Errorf("error when creating the perftest client Pod: %v", err)
	}
	podAIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-a", testNamespace)
	if err != nil {
		return nil, nil, fmt.Errorf("error when waiting for the perftest client Pod: %v", err)
	}

	if err := data.createPodOnNode("perftest-b", testNamespace, controlPlaneNodeName(), perftoolImage, nil, nil, nil, []corev1.ContainerPort{{Protocol: corev1.ProtocolTCP, ContainerPort: iperfPort}}, false, nil); err != nil {
		return nil, nil, fmt.Errorf("error when creating the perftest server Pod: %v", err)
	}
	podBIPs, err = data.podWaitForIPs(defaultTimeout, "perftest-b", testNamespace)
	if err != nil {
		return nil, nil, fmt.Errorf("error when getting the perftest server Pod's IPs: %v", err)
	}
	return podAIPs, podBIPs, nil
}

func createTestService(data *TestData, isIPv6 bool) (svcB *corev1.Service, err error) {
	svcIPFamily := corev1.IPv4Protocol
	if isIPv6 {
		svcIPFamily = corev1.IPv6Protocol
	}

	svcB, err = data.CreateService("perftest-b", testNamespace, iperfPort, iperfPort, map[string]string{"antrea-e2e": "perftest-b"}, false, false, corev1.ServiceTypeClusterIP, &svcIPFamily)
	if err != nil {
		return nil, fmt.Errorf("error when creating perftest-b Service: %v", err)
	}

	return svcB, nil
}
