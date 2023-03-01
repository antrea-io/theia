// Copyright 2023 Antrea Authors
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
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Use a long timeout as it takes ~500s to complete a single Spark job on
	// Kind testbed
	tadjobCompleteTimeout = 10 * time.Minute
	tadstartCmd           = "./theia throughput-anomaly-detection run"
	tadstatusCmd          = "./theia throughput-anomaly-detection status"
	tadlistCmd            = "./theia throughput-anomaly-detection list"
	taddeleteCmd          = "./theia throughput-anomaly-detection delete"
	tadretrieveCmd        = "./theia throughput-anomaly-detection retrieve"
)

func TestAnomalyDetection(t *testing.T) {
	config := FlowVisibiltiySetUpConfig{
		withSparkOperator:     true,
		withGrafana:           false,
		withClickHouseLocalPv: false,
		withFlowAggregator:    true,
	}
	data, _, _, err := setupTestForFlowVisibility(t, config)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer func() {
		teardownTest(t, data)
		teardownFlowVisibility(t, data, config)
	}()

	clientset := data.clientset
	kubeconfig, err := data.provider.GetKubeconfigPath()
	require.NoError(t, err)
	connect, pf, err := SetupClickHouseConnection(clientset, kubeconfig)
	require.NoError(t, err)
	if pf != nil {
		defer pf.Stop()
	}

	t.Run("testThroughputAnomalyDetectionAlgo", func(t *testing.T) {
		testThroughputAnomalyDetectionAlgo(t, data, connect)
	})

	t.Run("testAnomalyDetectionStatus", func(t *testing.T) {
		testAnomalyDetectionStatus(t, data, connect)
	})

	t.Run("testAnomalyDetectionList", func(t *testing.T) {
		testAnomalyDetectionList(t, data, connect)
	})

	t.Run("TestAnomalyDetectionDelete", func(t *testing.T) {
		testAnomalyDetectionDelete(t, data, connect)
	})

	t.Run("TestAnomalyDetectionRetrieve", func(t *testing.T) {
		testAnomalyDetectionRetrieve(t, data, connect)
	})
}

func prepareFlowTable(t *testing.T, connect *sql.DB) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		populateFlowTable(t, connect)
	}()
	wg.Wait()
}

// Example output: Successfully created Throughput Anomaly Detection job with name tad-eec9d1be-7204-4d50-8f57-d9c8757a2668
func testThroughputAnomalyDetectionAlgo(t *testing.T, data *TestData, connect *sql.DB) {
	prepareFlowTable(t, connect)
	stdout, jobName, err := tadrunJob(t, data, "ARIMA")
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, fmt.Sprintf("Successfully started Throughput Anomaly Detection job with name: %s", jobName), "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = taddeleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output: Status of this anomaly detection job is COMPLETED
func testAnomalyDetectionStatus(t *testing.T, data *TestData, connect *sql.DB) {
	prepareFlowTable(t, connect)
	_, jobName, err := tadrunJob(t, data, "ARIMA")
	require.NoError(t, err)
	stdout, err := tadgetJobStatus(t, data, jobName)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Status of this anomaly detection job is", "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = taddeleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output:
// CreationTime          CompletionTime        Name                                    Status
// 2022-06-17 15:03:24   N/A                   tad-615026a0-1856-4107-87d9-08f7d69819ae RUNNING
// 2022-06-17 15:03:22   2022-06-17 18:08:37   tad-7bebe4f9-408b-4dd8-9d63-9dc538073089 COMPLETED
// 2022-06-17 15:03:39   N/A                   tad-c7a9e768-559a-4bfb-b0c8-a0291b4c208c SUBMITTED
func testAnomalyDetectionList(t *testing.T, data *TestData, connect *sql.DB) {
	prepareFlowTable(t, connect)
	_, jobName, err := tadrunJob(t, data, "ARIMA")
	require.NoError(t, err)
	stdout, err := tadlistJobs(t, data)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "CreationTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "CompletionTime", "stdout: %s", stdout)
	assert.Containsf(stdout, "Name", "stdout: %s", stdout)
	assert.Containsf(stdout, "Status", "stdout: %s", stdout)
	assert.Containsf(stdout, jobName, "stdout: %s", stdout)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	_, err = taddeleteJob(t, data, jobName)
	require.NoError(t, err)
}

// Example output: Successfully deleted anomaly detection job with name tad-eec9d1be-7204-4d50-8f57-d9c8757a2668
func testAnomalyDetectionDelete(t *testing.T, data *TestData, connect *sql.DB) {
	prepareFlowTable(t, connect)
	_, jobName, err := tadrunJob(t, data, "ARIMA")
	require.NoError(t, err)
	err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
	require.NoError(t, err)
	stdout, err := taddeleteJob(t, data, jobName)
	require.NoError(t, err)
	assert := assert.New(t)
	assert.Containsf(stdout, "Successfully deleted anomaly detection job with name", "stdout: %s", stdout)
	stdout, err = tadlistJobs(t, data)
	require.NoError(t, err)
	assert.NotContainsf(stdout, jobName, "Still found deleted job in list command stdout: %s", stdout)
}

// Example Output
// id                                      sourceIP        sourceTransportPort     destinationIP   destinationTransportPort        flowStartSeconds        flowEndSeconds          throughput                         algoCalc                anomaly
// eec9d1be-7204-4d50-8f57-d9c8757a2668    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11T08:24:54     10004969097.000000000000000000      4.0063773860532994E9    true
// eec9d1be-7204-4d50-8f57-d9c8757a2668    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11T08:06:54     4005703059.000000000000000000       1.0001208294655691E10   true
// eec9d1be-7204-4d50-8f57-d9c8757a2668    10.10.1.25      58076                   10.10.1.33      5201                            2022-08-11T06:26:54Z    2022-08-11T08:34:54     50007861276.000000000000000000      3.9735065921281104E9    true
func testAnomalyDetectionRetrieve(t *testing.T, data *TestData, connect *sql.DB) {
	prepareFlowTable(t, connect)
	algoNames := []string{"ARIMA", "EWMA", "DBSCAN"}
	result_map := map[string]map[string]string{
		"ARIMA": {
			"4.006": "true",
			"1.000": "true",
			"3.973": "true"},
		"EWMA": {
			"2.700": "true",
			"1.550": "true",
			"9.755": "true"},
		"DBSCAN": {
			"1.000": "true",
			"1.005": "true",
			"5.000": "true",
			"3.260": "true",
			"2.058": "true"},
	}
	for _, algo := range algoNames {
		_, jobName, err := tadrunJob(t, data, algo)
		require.NoError(t, err)
		err = data.podWaitForReady(defaultTimeout, jobName+"-driver", flowVisibilityNamespace)
		require.NoError(t, err)
		err = waitTADJobComplete(t, data, jobName, tadjobCompleteTimeout)
		require.NoErrorf(t, err, "Throughput Anomaly Detection Spark job failed to complete")
		stdout, err := tadretrieveJobResult(t, data, jobName)
		resultArray := strings.Split(stdout, "\n")
		assert := assert.New(t)
		length := len(resultArray)
		assert.GreaterOrEqualf(length, 3, "stdout: %s", stdout)
		assert.Containsf(stdout, "throughput", "stdout: %s", stdout)
		assert.Containsf(stdout, "algoCalc", "stdout: %s", stdout)
		assert.Containsf(stdout, "anomaly", "stdout: %s", stdout)

		for i := 1; i < length; i++ {
			// check metrics' value
			resultArray[i] = strings.TrimSpace(resultArray[i])
			if resultArray[i] != "" {
				resultArray[i] = strings.ReplaceAll(resultArray[i], "\t", " ")
				tadoutputArray := strings.Fields(resultArray[i])
				anomaly_output := tadoutputArray[9]
				assert.Equal(10, len(tadoutputArray), "tadoutputArray: %s", tadoutputArray)
				switch algo {
				case "ARIMA":
					algo_throughput := tadoutputArray[8][:5]
					assert.Equal(result_map["ARIMA"][algo_throughput], anomaly_output, "Anomaly outputs dont match in tadoutputArray: %s", tadoutputArray)
				case "EWMA":
					algo_throughput := tadoutputArray[8][:5]
					assert.Equal(result_map["EWMA"][algo_throughput], anomaly_output, "Anomaly outputs dont match in tadoutputArray: %s", tadoutputArray)
				case "DBSCAN":
					throughput := tadoutputArray[7][:5]
					assert.Equal(result_map["DBSCAN"][throughput], anomaly_output, "Anomaly outputs dont match in tadoutputArray: %s", tadoutputArray)
				}
			}
		}
		require.NoError(t, err)
		_, err = taddeleteJob(t, data, jobName)
		require.NoError(t, err)
	}
}

// waitJobComplete waits for the anomaly detection Spark job completes
func waitTADJobComplete(t *testing.T, data *TestData, jobName string, timeout time.Duration) error {
	stdout := ""
	err := wait.PollImmediate(defaultInterval, timeout, func() (bool, error) {
		stdout, err := tadgetJobStatus(t, data, jobName)
		require.NoError(t, err)
		if strings.Contains(stdout, "Status of this anomaly detection job is COMPLETED") {
			return true, nil
		}
		// Keep trying
		return false, nil
	})
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("anomaly detection Spark job not completed after %v\nstatus:%s", timeout, stdout)
	} else if err != nil {
		return err
	}
	return nil
}

func tadrunJob(t *testing.T, data *TestData, algotype string) (stdout string, jobName string, err error) {
	newjobcmd := tadstartCmd + " --algo " + algotype + " --driver-memory 1G --start-time 2022-08-11T06:26:50 --end-time 2022-08-12T08:26:54"
	stdout, jobName, err = RunJob(t, data, newjobcmd)
	if err != nil {
		return "", "", err
	}
	return stdout, jobName, nil
}

func tadgetJobStatus(t *testing.T, data *TestData, jobName string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", tadstatusCmd, jobName)
	stdout, err = GetJobStatus(t, data, cmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func tadlistJobs(t *testing.T, data *TestData) (stdout string, err error) {
	stdout, err = ListJobs(t, data, tadlistCmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func taddeleteJob(t *testing.T, data *TestData, jobName string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", taddeleteCmd, jobName)
	stdout, err = DeleteJob(t, data, cmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func tadretrieveJobResult(t *testing.T, data *TestData, jobName string) (stdout string, err error) {
	cmd := fmt.Sprintf("%s %s", tadretrieveCmd, jobName)
	stdout, err = RetrieveJobResult(t, data, cmd)
	if err != nil {
		return "", err
	}
	return stdout, nil
}

func addFakeRecordforTAD(t *testing.T, stmt *sql.Stmt) {
	flowStartSeconds, _ := time.Parse("2006-01-02T15:04:05", "2022-08-11T06:26:54")
	flowEndSeconds, _ := time.Parse("2006-01-02T15:04:05", "2022-08-11T07:26:54")
	sourceIP := "10.10.1.25"
	sourceTransportPort := 58076
	destinationIP := "10.10.1.33"
	destinationTransportPort := 5201
	protocolIndentifier := 6

	throughputs := []int64{
		4007380032, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
		4005435632, 4004465468, 4005336400, 4006201196, 4005546675,
		4005703059, 4004631769, 4006915708, 4004834307, 4005943619,
		4005760579, 4006503308, 4006580124, 4006524102, 4005521494,
		4004706899, 4006355667, 4006373555, 4005542681, 4006120227,
		4003599734, 4005561673, 4005682768, 10004969097, 4005517222,
		1005533779, 4005370905, 4005589772, 4005328806, 4004926121,
		4004496934, 4005615814, 4005798822, 50007861276, 4005396697,
		4005148294, 4006448435, 4005355097, 4004335558, 4005389043,
		4004839744, 4005556492, 4005796992, 4004497248, 4005988134,
		205881027, 4004638304, 4006191046, 4004723289, 4006172825,
		4005561235, 4005658636, 4006005936, 3260272025, 4005589772}
	for idx, throughput := range throughputs {
		_, err := stmt.Exec(
			flowStartSeconds,
			flowEndSeconds.Add(time.Minute*time.Duration(idx)),
			time.Now(),
			time.Now(),
			0,
			sourceIP,
			destinationIP,
			sourceTransportPort,
			destinationTransportPort,
			protocolIndentifier,
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			fmt.Sprintf("PodName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PodNameSpace-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("NodeName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PodName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PodNameSpace-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("NodeName-%d", randInt(t, MaxInt32)),
			getRandIP(t),
			uint16(randInt(t, 65535)),
			fmt.Sprintf("ServicePortName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PolicyName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PolicyNameSpace-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PolicyRuleName-%d", randInt(t, MaxInt32)),
			1,
			1,
			fmt.Sprintf("PolicyName-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PolicyNameSpace-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PolicyRuleName-%d", randInt(t, MaxInt32)),
			1,
			1,
			"tcpState",
			0,
			fmt.Sprintf("PodLabels-%d", randInt(t, MaxInt32)),
			fmt.Sprintf("PodLabels-%d", randInt(t, MaxInt32)),
			uint64(throughput),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			uint64(randInt(t, MaxInt32)),
			"",
		)
		require.NoError(t, err)
	}
}

func writeTADRecords(t *testing.T, connect *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()
	err := wait.PollImmediate(5*defaultInterval, defaultTimeout, func() (bool, error) {
		// Test ping DB
		err := connect.Ping()
		if err != nil {
			return false, nil
		}
		// Test open Transaction
		tx, err := connect.Begin()
		if err != nil {
			return false, nil
		}
		stmt, _ := tx.Prepare(insertQueryflowtable)
		defer stmt.Close()
		addFakeRecordforTAD(t, stmt)

		if err != nil {
			return false, nil
		}
		err = tx.Commit()
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	assert.NoError(t, err, "Unable to commit successfully to ClickHouse")
}

func populateFlowTable(t *testing.T, connect *sql.DB) {
	var wg sync.WaitGroup
	wg.Add(1)
	go writeTADRecords(t, connect, &wg)
	time.Sleep(time.Duration(insertInterval) * time.Second)
	wg.Wait()
}
