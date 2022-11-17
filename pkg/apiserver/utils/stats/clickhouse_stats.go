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

package stats

import (
	"database/sql"
	"fmt"

	"k8s.io/client-go/kubernetes"

	"antrea.io/theia/pkg/util"
)

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
	totalRows  sql.NullString
	totalBytes sql.NullString
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

type ClickHouseStatQuerierImpl struct {
	kubeClient        kubernetes.Interface
	clickhouseConnect *sql.DB
}

func NewClickHouseStatQuerierImpl(
	kubeClient kubernetes.Interface,
) *ClickHouseStatQuerierImpl {
	c := &ClickHouseStatQuerierImpl{
		kubeClient: kubeClient,
	}
	return c
}

func (c *ClickHouseStatQuerierImpl) GetDiskInfo(namespace string) ([][]string, error) {
	data, err := c.getDataFromClickHouse(diskQuery, namespace)
	if err != nil {
		return nil, fmt.Errorf("error when getting diskInfo from clickhouse: %v", err)
	}
	return data, nil
}

func (c *ClickHouseStatQuerierImpl) GetTableInfo(namespace string) ([][]string, error) {
	data, err := c.getDataFromClickHouse(tableInfoQuery, namespace)
	if err != nil {
		return nil, fmt.Errorf("error when getting tableInfo from clickhouse: %v", err)
	}
	return data, nil
}

func (c *ClickHouseStatQuerierImpl) GetInsertRate(namespace string) ([][]string, error) {
	data, err := c.getDataFromClickHouse(insertRateQuery, namespace)
	if err != nil {
		return nil, fmt.Errorf("error when getting insertRate from clickhouse: %v", err)
	}
	return data, nil
}

func (c *ClickHouseStatQuerierImpl) GetStackTraces(namespace string) ([][]string, error) {
	data, err := c.getDataFromClickHouse(stackTracesQuery, namespace)
	if err != nil {
		return nil, fmt.Errorf("error when getting stackTraces from clickhouse: %v", err)
	}
	return data, nil
}

func (c *ClickHouseStatQuerierImpl) getDataFromClickHouse(query int, namespace string) ([][]string, error) {
	var err error
	if c.clickhouseConnect == nil {
		c.clickhouseConnect, err = util.SetupClickHouseConnection(c.kubeClient, namespace)
		if err != nil {
			return nil, err
		}
	}
	result, err := c.clickhouseConnect.Query(queryMap[query])
	if err != nil {
		c.clickhouseConnect = nil
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
		var err error
		switch query {
		case diskQuery:
			var res diskInfo
			err = result.Scan(&res.shard, &res.name, &res.path, &res.freeSpace, &res.totalSpace, &res.usedPercentage)
			data = append(data, []string{res.shard, res.name, res.path, res.freeSpace, res.totalSpace, res.usedPercentage + " %"})
		case tableInfoQuery:
			res := tableInfo{}
			err = result.Scan(&res.shard, &res.database, &res.tableName, &res.totalRows, &res.totalBytes, &res.totalCols)
			if !res.totalRows.Valid || !res.totalBytes.Valid {
				continue
			}
			data = append(data, []string{res.shard, res.database, res.tableName, res.totalRows.String, res.totalBytes.String, res.totalCols})
		case insertRateQuery:
			res := insertRate{}
			err = result.Scan(&res.shard, &res.rowsPerSec, &res.bytesPerSec)
			data = append(data, []string{res.shard, res.rowsPerSec, res.bytesPerSec})
		case stackTracesQuery:
			res := stackTraces{}
			err = result.Scan(&res.shard, &res.traceFunctions, &res.count)
			data = append(data, []string{res.shard, res.traceFunctions, res.count})
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse the data returned by database: %v", err)
		}
	}
	if len(data) <= 1 {
		return nil, fmt.Errorf("no data is returned by database")
	}
	return data, nil
}
