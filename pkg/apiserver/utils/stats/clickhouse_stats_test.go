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
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"antrea.io/theia/pkg/apis/stats/v1alpha1"
	"antrea.io/theia/pkg/theia/commands/config"
)

func TestGetDataFromClickHouse(t *testing.T) {
	testCases := []struct {
		name           string
		query          int
		expectedResult *v1alpha1.ClickHouseStats
		returnedRow    *sqlmock.Rows
	}{
		{
			name:        "Get diskInfo",
			query:       diskQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "DatabaseName", "Path", "Free", "Total", "Used_Percentage"}).AddRow("a", "b", "c", "d", "e", "f"),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				DiskInfos: []v1alpha1.DiskInfo{
					{Shard: "a", Database: "b", Path: "c", FreeSpace: "d", TotalSpace: "e", UsedPercentage: "f %"},
				},
				TableInfos:  nil,
				InsertRates: nil,
				StackTraces: nil,
			},
		},
		{
			name:        "Get tableInfo",
			query:       tableInfoQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "DatabaseName", "TableName", "TotalRows", "TotalBytes", "TotalCols"}).AddRow("a", "b", "c", "d", "e", "f"),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:    metav1.TypeMeta{},
				ObjectMeta:  metav1.ObjectMeta{},
				DiskInfos:   nil,
				TableInfos:  []v1alpha1.TableInfo{{Shard: "a", Database: "b", TableName: "c", TotalRows: "d", TotalBytes: "e", TotalCols: "f"}},
				InsertRates: nil,
				StackTraces: nil,
			},
		},
		{
			name:        "Get insertRate",
			query:       insertRateQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "RowsPerSecond", "BytesPerSecond"}).AddRow("a", "b", "c"),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:    metav1.TypeMeta{},
				ObjectMeta:  metav1.ObjectMeta{},
				DiskInfos:   nil,
				TableInfos:  nil,
				InsertRates: []v1alpha1.InsertRate{{Shard: "a", RowsPerSec: "b", BytesPerSec: "c"}},
				StackTraces: nil,
			},
		},
		{
			name:        "Get stackTraces",
			query:       stackTraceQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "trace_function", "count()"}).AddRow("a", "b", "c"),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:    metav1.TypeMeta{},
				ObjectMeta:  metav1.ObjectMeta{},
				DiskInfos:   nil,
				TableInfos:  nil,
				InsertRates: nil,
				StackTraces: []v1alpha1.StackTrace{{Shard: "a", TraceFunctions: "b", Count: "c"}},
			},
		},
		{
			name:        "Empty result",
			query:       stackTraceQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "trace_function", "count()"}),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:    metav1.TypeMeta{},
				ObjectMeta:  metav1.ObjectMeta{},
				DiskInfos:   nil,
				TableInfos:  nil,
				InsertRates: nil,
				StackTraces: nil,
			},
		},
		{
			name:        "Scan error",
			query:       stackTraceQuery,
			returnedRow: sqlmock.NewRows([]string{"Shard", "trace_function", "count()"}).AddRow(nil, "b", "c").AddRow("test1", "test2", "test3"),
			expectedResult: &v1alpha1.ClickHouseStats{
				TypeMeta:    metav1.TypeMeta{},
				ObjectMeta:  metav1.ObjectMeta{},
				DiskInfos:   nil,
				TableInfos:  nil,
				InsertRates: nil,
				StackTraces: []v1alpha1.StackTrace{{Shard: "test1", TraceFunctions: "test2", Count: "test3"}},
				ErrorMsg:    []string{"failed to parse the data returned by database: sql: Scan error on column index 0, name \"Shard\": converting NULL to string is unsupported"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			mock.ExpectQuery(regexp.QuoteMeta(queryMap[tc.query])).WillReturnRows(tc.returnedRow)
			controller := ClickHouseStatQuerierImpl{clickhouseConnect: db}
			var result v1alpha1.ClickHouseStats
			err = controller.getDataFromClickHouse(tc.query, config.FlowVisibilityNS, &result)

			assert.NoError(t, err)
			assert.EqualValues(t, tc.expectedResult, &result)

		})
	}
}
