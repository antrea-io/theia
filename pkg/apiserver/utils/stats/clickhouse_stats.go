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

	"antrea.io/theia/pkg/apis/stats/v1alpha1"
	"antrea.io/theia/pkg/util/clickhouse"
)

const (
	diskQuery int = iota
	tableInfoQuery
	// average writing rate for all tables per second
	insertRateQuery
	stackTraceQuery
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
	stackTraceQuery: `
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

func (c *ClickHouseStatQuerierImpl) GetDiskInfo(namespace string, stats *v1alpha1.ClickHouseStats) error {
	err := c.getDataFromClickHouse(diskQuery, namespace, stats)
	if err != nil {
		return fmt.Errorf("error when getting diskInfo from clickhouse: %v", err)
	}
	return nil
}

func (c *ClickHouseStatQuerierImpl) GetTableInfo(namespace string, stats *v1alpha1.ClickHouseStats) error {
	err := c.getDataFromClickHouse(tableInfoQuery, namespace, stats)
	if err != nil {
		return fmt.Errorf("error when getting tableInfo from clickhouse: %v", err)
	}
	return nil
}

func (c *ClickHouseStatQuerierImpl) GetInsertRate(namespace string, stats *v1alpha1.ClickHouseStats) error {
	err := c.getDataFromClickHouse(insertRateQuery, namespace, stats)
	if err != nil {
		return fmt.Errorf("error when getting insertRate from clickhouse: %v", err)
	}
	return nil
}

func (c *ClickHouseStatQuerierImpl) GetStackTrace(namespace string, stats *v1alpha1.ClickHouseStats) error {
	err := c.getDataFromClickHouse(stackTraceQuery, namespace, stats)
	if err != nil {
		return fmt.Errorf("error when getting stackTrace from clickhouse: %v", err)
	}
	return nil
}

func (c *ClickHouseStatQuerierImpl) getDataFromClickHouse(query int, namespace string, stats *v1alpha1.ClickHouseStats) error {
	var err error
	if c.clickhouseConnect == nil {
		c.clickhouseConnect, err = clickhouse.SetupConnection(nil)
		if err != nil {
			return err
		}
	}
	result, err := c.clickhouseConnect.Query(queryMap[query])
	if err != nil {
		c.clickhouseConnect = nil
		return fmt.Errorf("failed to get data from clickhouse: %v", err)
	}
	defer result.Close()
	for result.Next() {
		var err error
		switch query {
		case diskQuery:
			res := v1alpha1.DiskInfo{}
			err = result.Scan(&res.Shard, &res.Database, &res.Path, &res.FreeSpace, &res.TotalSpace, &res.UsedPercentage)
			if err != nil {
				stats.ErrorMsg = append(stats.ErrorMsg, fmt.Sprintf("failed to parse the data returned by database: %v", err))
				continue
			}
			res.UsedPercentage = res.UsedPercentage + " %"
			stats.DiskInfos = append(stats.DiskInfos, res)
		case tableInfoQuery:
			res := v1alpha1.TableInfo{}
			var totalRows sql.NullString
			var totalBytes sql.NullString
			err = result.Scan(&res.Shard, &res.Database, &res.TableName, &totalRows, &totalBytes, &res.TotalCols)
			if err != nil {
				stats.ErrorMsg = append(stats.ErrorMsg, fmt.Sprintf("failed to parse the data returned by database: %v", err))
				continue
			}
			if !totalRows.Valid || !totalBytes.Valid {
				continue
			}
			res.TotalRows = totalRows.String
			res.TotalBytes = totalBytes.String
			stats.TableInfos = append(stats.TableInfos, res)
		case insertRateQuery:
			res := v1alpha1.InsertRate{}
			err = result.Scan(&res.Shard, &res.RowsPerSec, &res.BytesPerSec)
			if err != nil {
				stats.ErrorMsg = append(stats.ErrorMsg, fmt.Sprintf("failed to parse the data returned by database: %v", err))
				continue
			}
			stats.InsertRates = append(stats.InsertRates, res)
		case stackTraceQuery:
			res := v1alpha1.StackTrace{}
			err = result.Scan(&res.Shard, &res.TraceFunctions, &res.Count)
			if err != nil {
				stats.ErrorMsg = append(stats.ErrorMsg, fmt.Sprintf("failed to parse the data returned by database: %v", err))
				continue
			}
			stats.StackTraces = append(stats.StackTraces, res)
		}
	}
	return nil
}
