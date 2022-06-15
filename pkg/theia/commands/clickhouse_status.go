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

package commands

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

type chOptions struct {
	diskInfo    bool
	tableInfo   bool
	insertRate  bool
	stackTraces bool
}

type diskInfo struct {
	shard          string
	name           string
	path           string
	freeSpace      string
	totalSpace     string
	usedPercentage string
}

type tableInfo struct {
	shard      string
	database   string
	tableName  string
	totalRows  string
	totalBytes string
	totalCols  string
}

type insertRate struct {
	shard       string
	rowsPerSec  string
	bytesPerSec string
}

type stackTraces struct {
	shard          string
	traceFunctions string
	count          string
}

const (
	diskQuery int = iota
	tableInfoQuery
	// average writing rate for all tables per second
	insertRateQuery
	stackTracesQuery
)

var queryMap = map[int]string{
	diskQuery: `
SELECT
	shardNum() as Shard,
	name as DatabaseName,
	path as Path,
	formatReadableSize(free_space) as Free,
	formatReadableSize(total_space) as Total,
	TRUNCATE((1 - free_space/total_space) * 100, 2) as Used_Percentage
FROM cluster('{cluster}', system.disks);`,
	tableInfoQuery: `
SELECT
	Shard,
	DatabaseName,
	TableName,
	TotalRows,
	TotalBytes,
	TotalCols
FROM (
	SELECT
		shardNum() as Shard,
		database AS DatabaseName,
		name AS TableName,
		total_rows AS TotalRows,
		formatReadableSize(total_bytes) AS TotalBytes
	FROM cluster('{cluster}', system.tables) WHERE database = 'default'
	) as t1
	INNER JOIN (
		SELECT
			shardNum() as Shard,
			table_catalog as DatabaseName,
			table_name as TableName,
			COUNT(*) as TotalCols
		FROM cluster('{cluster}', INFORMATION_SCHEMA.COLUMNS)
		WHERE table_catalog == 'default'
		GROUP BY table_name, table_catalog, Shard
		) as t2
		ON t1.DatabaseName = t2.DatabaseName and t1.TableName = t2.TableName and t1.Shard = t2.Shard`,
	// average writing rate for all tables per second
	insertRateQuery: `
SELECT
	sd.Shard,
	sd.RowsPerSecond,
	sd.BytesPerSecond
FROM (
	SELECT
		shardNum() as Shard,
		(intDiv(toUInt32(date_trunc('minute', toDateTime(event_time))), 2) * 2) * 1000 as t,
		TRUNCATE(avg(ProfileEvent_InsertedRows),0) as RowsPerSecond,
		formatReadableSize(avg(ProfileEvent_InsertedBytes)) as BytesPerSecond,
		ROW_NUMBER() OVER(PARTITION BY shardNum() ORDER BY t DESC) rowNumber
	FROM cluster('{cluster}', system.metric_log)
	GROUP BY t, shardNum()
	ORDER BY t DESC, shardNum()
	) sd
WHERE sd.rowNumber=1`,
	stackTracesQuery: `
SELECT
	shardNum() as Shard,
	arrayStringConcat(arrayMap(x -> demangle(addressToSymbol(x)), trace), '\\n') AS trace_function,
	count()
FROM cluster('{cluster}', system.stack_trace)
GROUP BY trace_function, Shard
ORDER BY count()
DESC SETTINGS allow_introspection_functions=1`,
}

var options *chOptions

var clickHouseStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Get diagnostic infos of ClickHouse database",
	Example: example,
	Args:    cobra.NoArgs,
	RunE:    getClickHouseStatus,
}

var example = strings.Trim(`
theia clickhouse status --diskInfo
theia clickhouse status --diskInfo --tableInfo
theia clickhouse status --diskInfo --tableInfo --insertRate
`, "\n")

func init() {
	clickHouseCmd.AddCommand(clickHouseStatusCmd)
	options = &chOptions{}
	clickHouseStatusCmd.Flags().BoolVar(&options.diskInfo, "diskInfo", false, "check disk usage information")
	clickHouseStatusCmd.Flags().BoolVar(&options.tableInfo, "tableInfo", false, "check basic table information")
	clickHouseStatusCmd.Flags().BoolVar(&options.insertRate, "insertRate", false, "check the insertion-rate of clickhouse")
	clickHouseStatusCmd.Flags().BoolVar(&options.stackTraces, "stackTraces", false, "check stacktrace of clickhouse")
}

func getClickHouseStatus(cmd *cobra.Command, args []string) error {
	if !options.diskInfo && !options.tableInfo && !options.insertRate && !options.stackTraces {
		return fmt.Errorf("no metric related flag is specified")
	}
	kubeconfig, err := ResolveKubeConfig(cmd)
	if err != nil {
		return err
	}
	clientset, err := CreateK8sClient(kubeconfig)
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
	if err := CheckClickHousePod(clientset); err != nil {
		return err
	}
	// Connect to ClickHouse and get the result
	connect, pf, err := SetupClickHouseConnection(clientset, kubeconfig, endpoint, useClusterIP)
	if err != nil {
		return err
	}
	if pf != nil {
		defer pf.Stop()
	}
	if options.diskInfo {
		data, err := getDataFromClickHouse(connect, diskQuery)
		if err != nil {
			return fmt.Errorf("error when getting diskInfo from clickhouse: %v", err)
		}
		TableOutput(data)
	}
	if options.tableInfo {
		data, err := getDataFromClickHouse(connect, tableInfoQuery)
		if err != nil {
			return fmt.Errorf("error when getting tableInfo from clickhouse: %v", err)
		}
		TableOutput(data)
	}
	if options.insertRate {
		data, err := getDataFromClickHouse(connect, insertRateQuery)
		if err != nil {
			return fmt.Errorf("error when getting insertRate from clickhouse: %v", err)
		}
		TableOutput(data)
	}
	if options.stackTraces {
		data, err := getDataFromClickHouse(connect, stackTracesQuery)
		if err != nil {
			return fmt.Errorf("error when getting stackTraces from clickhouse: %v", err)
		}
		TableOutputVertical(data)
	}
	return nil
}

func getDataFromClickHouse(connect *sql.DB, query int) ([][]string, error) {
	result, err := connect.Query(queryMap[query])
	if err != nil {
		return nil, fmt.Errorf("failed to get data from clickhouse: %v", err)
	}
	defer result.Close()
	columnName, err := result.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get the name of columns: %v", err)
	}
	var data [][]string
	data = append(data, columnName)
	for result.Next() {
		switch query {
		case diskQuery:
			var res diskInfo
			result.Scan(&res.shard, &res.name, &res.path, &res.freeSpace, &res.totalSpace, &res.usedPercentage)
			data = append(data, []string{res.shard, res.name, res.path, res.freeSpace, res.totalSpace, res.usedPercentage + " %"})
		case tableInfoQuery:
			res := tableInfo{}
			result.Scan(&res.shard, &res.database, &res.tableName, &res.totalRows, &res.totalBytes, &res.totalCols)
			if !strings.Contains(res.tableName, "inner") && res.tableName != "flows" {
				continue
			}
			data = append(data, []string{res.shard, res.database, res.tableName, res.totalRows, res.totalBytes, res.totalCols})
		case insertRateQuery:
			res := insertRate{}
			result.Scan(&res.shard, &res.rowsPerSec, &res.bytesPerSec)
			data = append(data, []string{res.shard, res.rowsPerSec, res.bytesPerSec})
		case stackTracesQuery:
			res := stackTraces{}
			result.Scan(&res.shard, &res.traceFunctions, &res.count)
			data = append(data, []string{res.shard, res.traceFunctions, res.count})
		}
	}
	if len(data) <= 1 {
		return nil, fmt.Errorf("no data is returned by database")
	}
	return data, nil
}
