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

package clickhouse

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/theia/util"
)

type chOptions struct {
	diskUsage   bool
	numRecords  bool
	insertRate  bool
	formatTable bool
}

type diskInfo struct {
	shard          string
	name           string
	path           string
	freeSpace      string
	totalSpace     string
	usedPercentage string
}

type tableInfoBasic struct {
	shard      string
	database   string
	tableName  string
	totalRows  string
	totalBytes string
	totalCols  string
}

type writeRowsPerSec struct {
	shard       string
	rowsPerSec  string
	bytesPerSec string
}

const (
	diskQuery = "SELECT shardNum() as shard, name as Name, path as Path, formatReadableSize(free_space) as Free," +
		" formatReadableSize(total_space) as Total, TRUNCATE((1 - free_space/total_space) * 100, 2) as Used_Percentage FROM " +
		"cluster('{cluster}', system.disks) ;"
	tableInfoBasicQuery = "SELECT shard, DatabaseName, TableName, TotalRows, TotalBytes, TotalCols FROM (SELECT " +
		"shardNum() as shard, database AS DatabaseName, name AS TableName, total_rows AS TotalRows, " +
		"formatReadableSize(total_bytes) AS TotalBytes FROM cluster('{cluster}', system.tables) WHERE database = " +
		"'default') as t1 INNER JOIN(SELECT shardNum() as shard, table_catalog as DatabaseName, table_name as " +
		"TableName, COUNT(*) as TotalCols FROM cluster('{cluster}', INFORMATION_SCHEMA.COLUMNS) WHERE table_catalog == " +
		"'default' GROUP BY table_name, table_catalog, shard) as t2 ON t1.DatabaseName = t2.DatabaseName and " +
		"t1.TableName = t2.TableName and t1.shard = t2.shard"
	// average writing rate for all tables per second
	writePerSecQuery = "SELECT sd.shard, sd.Rows_per_second, sd.Bytes_per_second  FROM (SELECT shardNum() as " +
		"shard, (intDiv(toUInt32(date_trunc('minute', toDateTime(event_time))), 2) * 2) * 1000 as t, " +
		"TRUNCATE(avg(ProfileEvent_InsertedRows),0) as Rows_per_second, formatReadableSize(avg(ProfileEvent_InsertedBytes)) as Bytes_per_second, " +
		"ROW_NUMBER() OVER(PARTITION BY shardNum() ORDER BY t DESC) rowNumber FROM cluster('{cluster}', " +
		"system.metric_log) GROUP BY t, shardNum() ORDER BY t DESC, shardNum()) sd WHERE sd.rowNumber=1"
)

var options *chOptions

// Command is the support bundle command implementation.
var Command *cobra.Command

var example = strings.Trim(`
theia get clickhouse --storage
theia get clickhouse --storage --print-table
theia get clickhouse --storage --record-number --insertion-rate --print-table
`, "\n")

func init() {
	Command = &cobra.Command{
		Use:     "clickhouse",
		Short:   "check clickhouse status",
		Example: example,
		Args:    cobra.NoArgs,
		RunE:    getClickHouseStatus,
	}
	options = &chOptions{}
	Command.Flags().BoolVar(&options.diskUsage, "storage", false, "check storage")
	Command.Flags().BoolVar(&options.numRecords, "record-number", false, "check number of records")
	Command.Flags().BoolVar(&options.insertRate, "insertion-rate", false, "check insertion-rate")
	Command.Flags().BoolVar(&options.formatTable, "print-table", false, "output data in table format")
	Command.Flags().String("clickhouse-endpoint", "", "The ClickHouse service endpoint.")
	Command.Flags().Bool(
		"use-cluster-ip",
		false,
		`Enable this option will use Service ClusterIP instead of port forwarding when connecting to the ClickHouse service.
It can only be used when running theia in cluster.`,
	)
}

func getClickHouseStatus(cmd *cobra.Command, args []string) error {
	if !options.diskUsage && !options.numRecords && !options.insertRate {
		return fmt.Errorf("no flag is specified")
	}
	kubeconfig, err := util.ResolveKubeConfig(cmd)
	if err != nil {
		return err
	}
	clientset, err := util.CreateK8sClient(kubeconfig)
	if err != nil {
		return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
	}

	endpoint, err := cmd.Flags().GetString("clickhouse-endpoint")
	if err != nil {
		return err
	}
	if endpoint != "" {
		_, err := url.ParseRequestURI(endpoint)
		if err != nil {
			return fmt.Errorf("failed to decode input endpoint %s into a url, err: %v", endpoint, err)
		}
	}
	useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
	if err != nil {
		return err
	}
	if endpoint == "" {
		service := "clickhouse-clickhouse"
		if useClusterIP {
			serviceIP, servicePort, err := util.GetServiceAddr(clientset, service)
			if err != nil {
				return fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		} else {
			listenAddress := "localhost"
			listenPort := 9000
			_, servicePort, err := util.GetServiceAddr(clientset, service)
			if err != nil {
				return fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
			}
			// Forward the ClickHouse service port
			pf, err := util.StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
			if err != nil {
				return fmt.Errorf("error when forwarding port: %v", err)
			}
			defer pf.Stop()
			endpoint = fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)
			fmt.Println(endpoint)
		}
	}

	if err := util.CheckClickHousePod(clientset); err != nil {
		return err
	}
	// Connect to ClickHouse and get the result
	username, password, err := util.GetClickHouseSecret(clientset)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err := util.ConnectClickHouse(clientset, url)
	if err != nil {
		return fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	if options.diskUsage {
		data, err := getDiskInfoFromClickHouse(connect)
		if err != nil {
			return err
		}
		if options.formatTable {
			printTable(data)
		} else {
			for _, arr := range data {
				fmt.Println(arr)
			}
		}
	}
	if options.numRecords {
		data, err := getTableInfoBasicFromClickHouse(connect)
		if err != nil {
			return err
		}
		if options.formatTable {
			printTable(data)
		} else {
			for _, arr := range data {
				fmt.Println(arr)
			}
		}
	}
	if options.insertRate {
		data, err := getWritingRateFromClickHouse(connect)
		if err != nil {
			return err
		}
		if options.formatTable {
			printTable(data)
		} else {
			for _, arr := range data {
				fmt.Println(arr)
			}
		}
	}
	return nil
}

func getDiskInfoFromClickHouse(connect *sql.DB) ([][]string, error) {
	result, err := connect.Query(diskQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk information: %v", err)
	}
	defer result.Close()
	columnName, err := result.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get the head of disk information: %v", err)
	}
	var data [][]string
	data = append(data, columnName)
	for result.Next() {
		var res diskInfo
		result.Scan(&res.shard, &res.name, &res.path, &res.freeSpace, &res.totalSpace, &res.usedPercentage)
		data = append(data, []string{res.shard, res.name, res.path, res.freeSpace, res.totalSpace, res.usedPercentage})
	}
	if len(data) <= 1 {
		return nil, fmt.Errorf("no data is returned by database")
	}
	return data, nil
}

func getTableInfoBasicFromClickHouse(connect *sql.DB) ([][]string, error) {
	result, err := connect.Query(tableInfoBasicQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get table information: %v", err)
	}
	defer result.Close()
	columnName, err := result.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get the head of table information: %v", err)
	}
	var data [][]string
	data = append(data, columnName)
	for result.Next() {
		res := tableInfoBasic{}
		result.Scan(&res.shard, &res.database, &res.tableName, &res.totalRows, &res.totalBytes, &res.totalCols)
		data = append(data, []string{res.shard, res.database, res.tableName, res.totalRows, res.totalBytes, res.totalCols})
	}
	if len(data) <= 1 {
		return nil, fmt.Errorf("no data is returned by database")
	}
	return data, nil
}

func getWritingRateFromClickHouse(connect *sql.DB) ([][]string, error) {
	result, err := connect.Query(writePerSecQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get insertion rate: %v", err)
	}
	defer result.Close()
	columnName, err := result.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get the head of insertion rate: %v", err)
	}
	var data [][]string
	data = append(data, columnName)
	for result.Next() {
		res := writeRowsPerSec{}
		result.Scan(&res.shard, &res.rowsPerSec, &res.bytesPerSec)
		data = append(data, []string{res.shard, res.rowsPerSec, res.bytesPerSec})
	}
	if len(data) <= 1 {
		return nil, fmt.Errorf("no data is returned by database")
	}
	return data, nil
}

func printTable(data [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	//table, _ := tablewriter.NewCSV(os.Stdout, "./test_info.csv", true)
	//table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader(data[0])
	for i := 1; i < len(data); i++ {
		table.Append(data[i])
	}
	table.Render()
}
