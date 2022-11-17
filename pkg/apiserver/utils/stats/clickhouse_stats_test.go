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
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"antrea.io/theia/pkg/theia/commands/config"
)

func TestGetDataFromClickHouse(t *testing.T) {
	testCases := []struct {
		name           string
		query          int
		expectedError  error
		expectedResult [][]string
		returnedRow    *sqlmock.Rows
	}{
		{
			name:           "Get diskInfo",
			query:          diskQuery,
			expectedError:  nil,
			returnedRow:    sqlmock.NewRows([]string{"Shard", "DatabaseName", "Path", "Free", "Total", "Used_Percentage"}).AddRow("a", "b", "c", "d", "e", "f"),
			expectedResult: [][]string{{"Shard", "DatabaseName", "Path", "Free", "Total", "Used_Percentage"}, {"a", "b", "c", "d", "e", "f %"}},
		},
		{
			name:           "Get tableInfo",
			query:          tableInfoQuery,
			expectedError:  nil,
			returnedRow:    sqlmock.NewRows([]string{"Shard", "DatabaseName", "TableName", "TotalRows", "TotalBytes", "TotalCols"}).AddRow("a", "b", "c", "d", "e", "f"),
			expectedResult: [][]string{{"Shard", "DatabaseName", "TableName", "TotalRows", "TotalBytes", "TotalCols"}, {"a", "b", "c", "d", "e", "f"}},
		},
		{
			name:           "Get insertRate",
			query:          insertRateQuery,
			expectedError:  nil,
			returnedRow:    sqlmock.NewRows([]string{"Shard", "RowsPerSecond", "BytesPerSecond"}).AddRow("a", "b", "c"),
			expectedResult: [][]string{{"Shard", "RowsPerSecond", "BytesPerSecond"}, {"a", "b", "c"}},
		},
		{
			name:           "Get stackTraces",
			query:          stackTracesQuery,
			expectedError:  nil,
			returnedRow:    sqlmock.NewRows([]string{"Shard", "trace_function", "count()"}).AddRow("a", "b", "c"),
			expectedResult: [][]string{{"Shard", "trace_function", "count()"}, {"a", "b", "c"}},
		},
		{
			name:           "Empty result",
			query:          stackTracesQuery,
			expectedError:  fmt.Errorf("no data is returned by database"),
			returnedRow:    sqlmock.NewRows([]string{"Shard", "trace_function", "count()"}),
			expectedResult: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			mock.ExpectQuery(regexp.QuoteMeta(queryMap[tc.query])).WillReturnRows(tc.returnedRow)
			controller := ClickHouseStatQuerierImpl{clickhouseConnect: db}
			result, err := controller.getDataFromClickHouse(tc.query, config.FlowVisibilityNS)

			if tc.expectedError == nil {
				assert.NoError(t, err)
				assert.ElementsMatch(t, result, tc.expectedResult)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			}
		})
	}
}
