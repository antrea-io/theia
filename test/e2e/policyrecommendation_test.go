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
	startCmd           = "./theia policy-recommendation run"
	statusCmd          = "./theia policy-recommendation status"
	listCmd            = "./theia policy-recommendation list"
	deleteCmd          = "./theia policy-recommendation delete"
	retrieveCmd        = "./theia policy-recommendation retrieve"
	// With the workload traffic perftest-a -> perftest-b, we expect the policy
	// recommendation job recommends two allow ANP, and two default deny ACNP.
	// Besides, there will always be three allow ACNP recommended for the
	// 'kube-system', 'flow-aggregator', and 'flow-visibility' Namespace.
	expectedAllowANPCnt   = 2
	expectedAllowACNPCnt  = 3
	expectedRejectANPCnt  = 0
	expectedRejectACNPCnt = 2
)

func TestPolicyRecommendation(t *testing.T) {
	data, v4Enabled, v6Enabled, err := setupTestForFlowVisibility(t, true)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer func() {
		teardownTest(t, data)
		deleteRecommendedPolicies(t, data)
		teardownFlowVisibility(t, data, true)
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

	podAIPs, podBIPs, err := createTestPods(data)
	if err != nil {
		t.Fatalf("Error when creating test Pods: %v", err)
	}

	if v4Enabled {
		srcIP := podAIPs.ipv4.String()
		dstIP := podBIPs.ipv4.String()
		testFlow := testFlow{
			srcIP:      srcIP,
			dstIP:      dstIP,
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		t.Run("testPolicyRecommendationRetrieve/IPv4", func(t *testing.T) {
			testPolicyRecommendationRetrieve(t, data, false, testFlow)
		})
	}
	if v6Enabled {
		srcIP := podAIPs.ipv6.String()
		dstIP := podBIPs.ipv6.String()
		testFlow := testFlow{
			srcIP:      srcIP,
			dstIP:      dstIP,
			srcPodName: "perftest-a",
			dstPodName: "perftest-b",
		}
		t.Run("testPolicyRecommendationRetrieve/IPv6", func(t *testing.T) {
			testPolicyRecommendationRetrieve(t, data, true, testFlow)
		})
	}
}

// Example output: Successfully created policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
func testPolicyRecommendationRun(t *testing.T, data *TestData) {
	stdout, jobId, err := runJob(t, data)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, fmt.Sprintf("Successfully created policy recommendation job with ID %s", jobId), "stdout: %s", stdout)
}

// Example output: Status of this policy recommendation job is COMPLETED
func testPolicyRecommendationStatus(t *testing.T, data *TestData) {
	_, jobId, err := runJob(t, data)
	require.NoError(t, err)
	stdout, err := getJobStatus(t, data, jobId)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Status of this policy recommendation job is", "stdout: %s", stdout)
}

// Example output:

// CreationTime          CompletionTime        ID                                   Status
// 2022-06-17 15:03:24 N/A                 615026a0-1856-4107-87d9-08f7d69819ae RUNNING
// 2022-06-17 15:03:22 2022-06-17 18:08:37 7bebe4f9-408b-4dd8-9d63-9dc538073089 COMPLETED
// 2022-06-17 15:03:39 N/A                 c7a9e768-559a-4bfb-b0c8-a0291b4c208c SUBMITTED
func testPolicyRecommendationList(t *testing.T, data *TestData) {
	_, jobId, err := runJob(t, data)
	require.NoError(t, err)
	stdout, err := listJobs(t, data)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "CreationTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "CompletionTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "ID", "stdout: %s", stdout)
	assert.Containsf(stdout, "Status", "stdout: %s", stdout)
	assert.Containsf(stdout, jobId, "stdout: %s", stdout)
}

// Example output: Successfully deleted policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
func testPolicyRecommendationDelete(t *testing.T, data *TestData) {
	_, jobId, err := runJob(t, data)
	require.NoError(t, err)
	stdout, err := deleteJob(t, data, jobId)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Successfully deleted policy recommendation job with ID", "stdout: %s", stdout)
	stdout, err = listJobs(t, data)
	require.NoError(t, err)
	assert.NotContainsf(stdout, jobId, "Still found deleted job in list command stdout: %s", stdout)
}

// Example output:
// apiVersion: crd.antrea.io/v1alpha1
// kind: NetworkPolicy
// metadata:
//   name: recommend-allow-anp-fj3hd
// ...
func testPolicyRecommendationRetrieve(t *testing.T, data *TestData, isIPv6 bool, testFlow testFlow) {
	var cmdStr string
	if !isIPv6 {
		cmdStr = fmt.Sprintf("iperf3 -c %s", testFlow.dstIP)
	} else {
		cmdStr = fmt.Sprintf("iperf3 -6 -c %s", testFlow.dstIP)
	}
	stdout, stderr, err := data.RunCommandFromPod(testNamespace, testFlow.srcPodName, "perftool", []string{"bash", "-c", cmdStr})
	require.NoErrorf(t, err, "Error when running iPerf3 client: %v,\nstdout:%s\nstderr:%s", err, stdout, stderr)

	_, jobId, err := runJob(t, data)
	require.NoError(t, err)
	err = waitJobComplete(t, data, jobId, jobCompleteTimeout)
	require.NoErrorf(t, err, "Policy recommendation Spark job failed to complete")

	// Apply the recommended policies, and check the results
	err = retrieveJobResult(t, data, jobId)
	require.NoError(t, err)
	cmd := fmt.Sprintf("kubectl apply -f %s", policyOutputYML)
	_, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), cmd)
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
	assert.Equalf(expectedAllowANPCnt, allowANPCnt, fmt.Sprintf("Expected allow ANP count is: %d. Actual count is: %d. Recommended policies:\n%s", expectedAllowANPCnt, allowANPCnt, allPolicies))
	assert.Equalf(expectedRejectANPCnt, rejectANPCnt, fmt.Sprintf("Expected reject ANP count is: %d. Actual count is: %d. Recommended policies:\n%s", expectedRejectANPCnt, rejectANPCnt, allPolicies))

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
	assert.Equalf(expectedAllowACNPCnt, allowACNPCnt, fmt.Sprintf("Expected allow ACNP count is: %d. Actual count is: %d. Recommended policies:\n%s", expectedAllowACNPCnt, allowACNPCnt, allPolicies))
	assert.Equalf(expectedRejectACNPCnt, rejectACNPCnt, fmt.Sprintf("Expected reject ACNP count is: %d. Actual count is: %d. Recommended policies:\n%s", expectedRejectACNPCnt, rejectACNPCnt, allPolicies))
}

func runJob(t *testing.T, data *TestData) (stdout string, jobId string, err error) {
	cmd := "chmod +x ./theia"
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	rc, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), startCmd)
	if err != nil || rc != 0 {
		return "", "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	stdout = strings.TrimSuffix(stdout, "\n")
	stdoutSlice := strings.Split(stdout, " ")
	jobId = stdoutSlice[len(stdoutSlice)-1]
	return stdout, jobId, nil
}

func getJobStatus(t *testing.T, data *TestData, jobId string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", statusCmd, jobId)
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func listJobs(t *testing.T, data *TestData) (stdout string, err error) {
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), listCmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", listCmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func deleteJob(t *testing.T, data *TestData, jobId string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", deleteCmd, jobId)
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func retrieveJobResult(t *testing.T, data *TestData, jobId string) error {
	cmd := fmt.Sprintf("%s %s -f %s", retrieveCmd, jobId, policyOutputYML)
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return nil
}

// waitJobComplete waits for the policy recommendation Spark job completes
func waitJobComplete(t *testing.T, data *TestData, jobId string, timeout time.Duration) error {
	stdout := ""
	err := wait.PollImmediate(defaultInterval, timeout, func() (bool, error) {
		stdout, err := getJobStatus(t, data, jobId)
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
