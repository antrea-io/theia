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
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"antrea.io/theia/pkg/theia/commands"
)

const (
	getDiskInfoCmd   = "./theia clickhouse status --diskInfo"
	getTableInfoCmd  = "./theia clickhouse status --tableInfo"
	getInsertRateCmd = "./theia clickhouse status --insertionRate"
	insertQuery      = `INSERT INTO flows (
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
                   reverseThroughputFromDestinationNode)
                   VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
                           ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
                           ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
                           ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
                           ?, ?, ?, ?, ?, ?, ?)`
	recordPerCommit = 1000
	insertInterval  = 1
	threshold       = 25
	MaxInt32        = 1<<31 - 1
)

var targetTable = map[string]string{
	".inner.flows_node_view":   "16",
	".inner.flows_pod_view":    "20",
	".inner.flows_policy_view": "27",
	"flows":                    "49",
}

func TestTheiaGetCommand(t *testing.T) {
	data, _, _, err := setupTestForFlowVisibility(t, false, false)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer func() {
		teardownTest(t, data)
		teardownFlowVisibility(t, data, false)
	}()

	clientset := data.clientset
	kubeconfig, err := data.provider.GetKubeconfigPath()
	require.NoError(t, err)
	connect, pf, err := commands.SetupClickHouseConnection(clientset, kubeconfig, "", false)
	require.NoError(t, err)
	if pf != nil {
		defer pf.Stop()
	}

	t.Run("testTheiaGetClickHouseDiskInfo", func(t *testing.T) {
		testTheiaGetClickHouseDiskInfo(t, data)
	})
	t.Run("testTheiaGetClickHouseTableInfo", func(t *testing.T) {
		testTheiaGetClickHouseTableInfo(t, data, connect)
	})
	t.Run("testTheiaGetClickHouseInsertRate", func(t *testing.T) {
		testTheiaGetClickHouseInsertRate(t, data, connect)
	})

}

func testTheiaGetClickHouseDiskInfo(t *testing.T, data *TestData) {
	// retrieve metrics
	stdout, err := getClickHouseDBInfo(t, data, getDiskInfoCmd)
	require.NoError(t, err)
	resultArray := strings.Split(stdout, "\n")
	assert := assert.New(t)
	length := len(resultArray)
	assert.GreaterOrEqualf(length, 2, "stdout: %s", stdout)
	// Check header component
	assert.Containsf(stdout, "shard", "stdout: %s", stdout)
	assert.Containsf(stdout, "Name", "stdout: %s", stdout)
	assert.Containsf(stdout, "Path", "stdout: %s", stdout)
	assert.Containsf(stdout, "Free", "stdout: %s", stdout)
	assert.Containsf(stdout, "Total", "stdout: %s", stdout)
	assert.Containsf(stdout, "Used_Percentage", "stdout: %s", stdout)
	for i := 1; i < length; i++ {
		// check metrics' value
		diskInfoArray := strings.Split(resultArray[i], " ")
		assert.Equal(8, len(diskInfoArray), "number of columns is not correct")
		assert.Equalf("default", diskInfoArray[1], "diskInfoArray: %s", diskInfoArray)
		assert.Equalf("/var/lib/clickhouse/", diskInfoArray[2], "diskInfoArray: %s", diskInfoArray)
		usedStorage, err := strconv.ParseFloat(strings.TrimSuffix(diskInfoArray[7], "]"), 64)
		assert.NoError(err)
		assert.LessOrEqualf(int(usedStorage), threshold, "diskInfoArray: %s", diskInfoArray)
		size, err := strconv.ParseFloat(diskInfoArray[5], 64)
		assert.NoError(err)
		assert.LessOrEqualf(int((chStorageSize-size)*100/chStorageSize), threshold, "diskInfoArray: %s", diskInfoArray)
	}
}

func testTheiaGetClickHouseTableInfo(t *testing.T, data *TestData, connect *sql.DB) {
	// send 10000 records to clickhouse
	commitNum := 10
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendTraffic(t, commitNum, connect)
	}()
	wg.Wait()
	// retrieve metrics
	stdout, err := getClickHouseDBInfo(t, data, getTableInfoCmd)
	require.NoError(t, err)
	resultArray := strings.Split(stdout, "\n")
	assert := assert.New(t)
	length := len(resultArray)
	assert.GreaterOrEqualf(length, 2, "stdout: %s", stdout)
	// check header component
	assert.Containsf(stdout, "shard", "stdout: %s", stdout)
	assert.Containsf(stdout, "DatabaseName", "stdout: %s", stdout)
	assert.Containsf(stdout, "TableName", "stdout: %s", stdout)
	assert.Containsf(stdout, "TotalRows", "stdout: %s", stdout)
	assert.Containsf(stdout, "TotalBytes", "stdout: %s", stdout)
	assert.Containsf(stdout, "TotalCols", "stdout: %s", stdout)
	// check four tables are in db
	assert.Containsf(stdout, ".inner.flows_node_view", "stdout: %s", stdout)
	assert.Containsf(stdout, ".inner.flows_pod_view", "stdout: %s", stdout)
	assert.Containsf(stdout, ".inner.flows_policy_view", "stdout: %s", stdout)
	assert.Containsf(stdout, "flows", "stdout: %s", stdout)
	assert.Containsf(stdout, "recommendations", "stdout: %s", stdout)

	flowNum := 0
	for i := 1; i < length; i++ {
		// check metrics' value
		tableInfoArray := strings.Split(resultArray[i], " ")
		tableName := tableInfoArray[2]
		expectedColNum, ok := targetTable[tableName]
		if !ok {
			continue
		}
		assert.Equal(7, len(tableInfoArray), "tableInfoArray: %s", tableInfoArray)
		assert.Equalf("default", tableInfoArray[1], "tableInfoArray: %s", tableInfoArray)
		assert.Equal(expectedColNum, strings.TrimSuffix(tableInfoArray[6], "]"), "tableInfoArray: %s", tableInfoArray)
		if tableName == "flows" {
			num, error := strconv.Atoi(tableInfoArray[3])
			assert.NoError(error)
			flowNum += num
		}
	}
	// sum of records in table flows in each shard should be the total number of records sent to db
	assert.Equal(commitNum*recordPerCommit, flowNum)
}

func testTheiaGetClickHouseInsertRate(t *testing.T, data *TestData, connect *sql.DB) {
	commitNum := 70
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sendTraffic(t, commitNum, connect)
	}()
	// need to wait at least 1 min to get the insertion rate.
	// insertion rate is the average ProfileEvent_InsertedRows in system.metric_log in current minute
	time.Sleep(1 * time.Minute)
	// retrieve metrics
	stdout, err := getClickHouseDBInfo(t, data, getInsertRateCmd)
	require.NoError(t, err)
	resultArray := strings.Split(stdout, "\n")
	assert := assert.New(t)
	length := len(resultArray)
	assert.GreaterOrEqualf(length, 2, "stdout: %s", stdout)
	// check header component
	assert.Containsf(stdout, "shard", "stdout: %s", stdout)
	assert.Containsf(stdout, "Rows_per_second", "stdout: %s", stdout)
	assert.Containsf(stdout, "Bytes_per_second", "stdout: %s", stdout)

	for i := 1; i < length; i++ {
		// check metrics' value
		tableInfoArray := strings.Split(resultArray[i], " ")
		assert.Equal(4, len(tableInfoArray), "tableInfoArray: %s", tableInfoArray)
		actualInsertRate, error := strconv.Atoi(tableInfoArray[1])
		assert.NoError(error)
		tableNum := len(targetTable)
		percent := (actualInsertRate/tableNum - recordPerCommit/insertInterval) * 100 / (recordPerCommit / insertInterval)
		assert.LessOrEqualf(percent, threshold, "stdout: %s, expectedInsertRate: %s", stdout, recordPerCommit/insertInterval)
	}
	wg.Wait()
}

func getClickHouseDBInfo(t *testing.T, data *TestData, query string) (stdout string, err error) {
	cmd := "chmod +x ./theia"
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	rc, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), query)

	if err != nil || rc != 0 {
		return "", fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	return strings.TrimSuffix(stdout, "\n"), nil
}

func getRandIP(t *testing.T) string {
	return fmt.Sprintf("%d.%d.%d.%d", randInt(t, 256), randInt(t, 256), randInt(t, 256), randInt(t, 256))
}

func addFakeRecord(t *testing.T, stmt *sql.Stmt) {
	_, err := stmt.Exec(
		time.Now(),
		time.Now(),
		time.Now(),
		time.Now(),
		0,
		getRandIP(t),
		getRandIP(t),
		uint16(randInt(t, 65535)),
		uint16(randInt(t, 65535)),
		6,
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
		uint64(randInt(t, MaxInt32)),
		uint64(randInt(t, MaxInt32)),
		uint64(randInt(t, MaxInt32)),
		uint64(randInt(t, MaxInt32)),
		uint64(randInt(t, MaxInt32)),
		uint64(randInt(t, MaxInt32)),
	)
	require.NoError(t, err)
}

func writeRecords(t *testing.T, connect *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()
	// Test ping DB
	var err error
	err = connect.Ping()
	require.NoError(t, err)
	// Test open Transaction
	tx, err := connect.Begin()
	require.NoError(t, err)
	stmt, _ := tx.Prepare(insertQuery)
	defer stmt.Close()
	for j := 0; j < recordPerCommit; j++ {
		addFakeRecord(t, stmt)
	}
	err = tx.Commit()
	assert.NoError(t, err)
}

func sendTraffic(t *testing.T, commitNum int, connect *sql.DB) {
	var wg sync.WaitGroup
	for i := 0; i < commitNum; i++ {
		wg.Add(1)
		go writeRecords(t, connect, &wg)
		time.Sleep(time.Duration(insertInterval) * time.Second)
	}
	wg.Wait()
}

func randInt(t *testing.T, limit int64) int64 {
	assert := assert.New(t)
	randNum, error := rand.Int(rand.Reader, big.NewInt(limit))
	assert.NoError(error)
	return randNum.Int64()
}
