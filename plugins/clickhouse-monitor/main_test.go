// Copyright 2022 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestMonitorWithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	initEnv()

	foreverRun = func(f func(), period time.Duration) {
		f()
	}

	testCases := []struct {
		name                       string
		remainingRoundsNum         int
		expectedRemainingRoundsNum int
		setUpMock                  func(mock sqlmock.Sqlmock)
	}{
		{
			name:                       "Monitor memory with deletion",
			remainingRoundsNum:         0,
			expectedRemainingRoundsNum: 3,
			setUpMock: func(mock sqlmock.Sqlmock) {
				baseTime := time.Now()
				diskRow := sqlmock.NewRows([]string{"free_space", "total_space"}).AddRow(4, 10)
				partsRow := sqlmock.NewRows([]string{"SUM(bytes)"}).AddRow(5)
				countRow := sqlmock.NewRows([]string{"count"}).AddRow(10)
				timeRow := sqlmock.NewRows([]string{"timeInserted"}).AddRow(baseTime.Add(5 * time.Second))
				mock.ExpectQuery("SELECT free_space, total_space FROM system.disks").WillReturnRows(diskRow)
				mock.ExpectQuery("SELECT SUM(bytes) FROM system.parts").WillReturnRows(partsRow)
				mock.ExpectQuery("SELECT COUNT() FROM flows").WillReturnRows(countRow)
				mock.ExpectQuery("SELECT timeInserted FROM flows LIMIT 1 OFFSET (?)").WithArgs(4).WillReturnRows(timeRow)
				for _, table := range []string{"flows", "flows_pod_view", "flows_node_view", "flows_policy_view"} {
					query := fmt.Sprintf("ALTER TABLE %s DELETE WHERE timeInserted < toDateTime(?)", table)
					mock.ExpectExec(query).WithArgs(baseTime.Add(5 * time.Second).Format(timeFormat)).WillReturnResult(sqlmock.NewResult(0, 5))
				}
			},
		},
		{
			name:                       "Monitor memory without deletion",
			remainingRoundsNum:         0,
			expectedRemainingRoundsNum: 0,
			setUpMock: func(mock sqlmock.Sqlmock) {
				diskRow := sqlmock.NewRows([]string{"free_space", "total_space"}).AddRow(6, 10)
				partsRow := sqlmock.NewRows([]string{"SUM(bytes)"}).AddRow(5)
				mock.ExpectQuery("SELECT free_space, total_space FROM system.disks").WillReturnRows(diskRow)
				mock.ExpectQuery("SELECT SUM(bytes) FROM system.parts").WillReturnRows(partsRow)
			},
		},
		{
			name:                       "Skip a round",
			remainingRoundsNum:         2,
			expectedRemainingRoundsNum: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			remainingRoundsNum = tc.remainingRoundsNum
			if tc.setUpMock != nil {
				tc.setUpMock(mock)
			}
			startMonitor(db)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
			assert.Equal(t, tc.expectedRemainingRoundsNum, remainingRoundsNum)
		})
	}

	testConnection(t, db, mock)
	testCheckStorageCondition(t, db, mock)

}

func initEnv() {
	tableName = "flows"
	mvNames = []string{"flows_pod_view", "flows_node_view", "flows_policy_view"}
	allocatedSpace = 10
	threshold = 0.5
	deletePercentage = 0.5
	skipRoundsNum = 3
	monitorExecInterval = 1 * time.Minute
}

func testConnection(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock) {
	mock.ExpectPing()

	getEnv = func(key string) string {
		switch key {
		case "CLICKHOUSE_USERNAME":
			return "username"
		case "CLICKHOUSE_PASSWORD":
			return "password"
		case "DB_URL":
			return "tcp://localhost:9000"
		default:
			return ""
		}
	}

	openSql = func(driverName, dataSourceName string) (*sql.DB, error) {
		assert.Equal(t, driverName, "clickhouse")
		assert.Equal(t, dataSourceName, "tcp://localhost:9000?debug=true&username=username&password=password")
		return db, nil
	}

	_, err := connectLoop()
	assert.NoError(t, err)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func testCheckStorageCondition(t *testing.T, db *sql.DB, mock sqlmock.Sqlmock) {
	diskRow := sqlmock.NewRows([]string{"free_space", "total_space"}).AddRow(9, 10)
	partsRow := sqlmock.NewRows([]string{"SUM(bytes)"}).AddRow(1)
	mock.ExpectQuery("SELECT free_space, total_space FROM system.disks").WillReturnError(fmt.Errorf("error in database, please retry"))
	mock.ExpectQuery("SELECT free_space, total_space FROM system.disks").WillReturnRows(diskRow)
	mock.ExpectQuery("SELECT SUM(bytes) FROM system.parts").WillReturnError(fmt.Errorf("error in database, please retry"))
	mock.ExpectQuery("SELECT SUM(bytes) FROM system.parts").WillReturnRows(partsRow)
	checkStorageCondition(db)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestSanitizeIdentifier(t *testing.T) {
	testCases := []struct {
		identifier string
		err        error
	}{
		{
			identifier: "default.flows_local",
			err:        nil,
		},
		{
			identifier: "flows",
			err:        nil,
		},
		{
			identifier: "a.b.c",
			err:        errNotAValidIdentifier,
		},
		{
			identifier: "a b",
			err:        errNotAValidIdentifier,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.identifier, func(t *testing.T) {
			identifier, err := sanitizeIdentifier(tc.identifier)
			assert.Equal(t, tc.err, err)
			if err == nil {
				assert.Equal(t, tc.identifier, identifier)
			}
		})
	}
}

func TestLoadEnvVariables(t *testing.T) {
	defaultGetEnv := func(key string) string {
		switch key {
		case "TABLE_NAME":
			return "flows"
		case "MV_NAMES":
			return "flows_pod_view flows_node_view flows_pod_view"
		case "STORAGE_SIZE":
			return "8Gi"
		case "THRESHOLD":
			return "0.1"
		case "DELETE_PERCENTAGE":
			return "0.5"
		case "SKIP_ROUNDS_NUM":
			return "3"
		case "EXEC_INTERVAL":
			return "1m"
		default:
			return ""
		}
	}
	testCases := []struct {
		name          string
		getEnv        func(key string) string
		expectedError error
	}{
		{
			name:   "valid variables",
			getEnv: defaultGetEnv,
		},
		{
			name: "missing environment variables",
			getEnv: func(key string) string {
				if key == "DELETE_PERCENTAGE" {
					return ""
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("unable to load environment variables, TABLE_NAME, MV_NAMES, STORAGE_SIZE, THRESHOLD, DELETE_PERCENTAGE, SKIP_ROUNDS_NUM, and EXEC_INTERVAL must be defined"),
		},
		{
			name: "invalid table name",
			getEnv: func(key string) string {
				if key == "TABLE_NAME" {
					return "a b"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("invalid TABLE_NAME: "),
		},
		{
			name: "invalid view name",
			getEnv: func(key string) string {
				if key == "MV_NAMES" {
					return "a b,c"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("invalid MV_NAMES: "),
		},
		{
			name: "invalid storage size",
			getEnv: func(key string) string {
				if key == "STORAGE_SIZE" {
					return "G8.0"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("error when parsing STORAGE_SIZE:"),
		},
		{
			name: "invalid threshold",
			getEnv: func(key string) string {
				if key == "THRESHOLD" {
					return "threshold"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("error when parsing THRESHOLD: "),
		},
		{
			name: "invalid delete percentage",
			getEnv: func(key string) string {
				if key == "DELETE_PERCENTAGE" {
					return "t1"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("error when parsing DELETE_PERCENTAGE: "),
		},
		{
			name: "invalid number of skip rounds",
			getEnv: func(key string) string {
				if key == "SKIP_ROUNDS_NUM" {
					return "0.1"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("error when parsing SKIP_ROUNDS_NUM: "),
		},
		{
			name: "invalid execution interval",
			getEnv: func(key string) string {
				if key == "EXEC_INTERVAL" {
					return "1"
				} else {
					return defaultGetEnv(key)
				}
			},
			expectedError: fmt.Errorf("error when parsing EXEC_INTERVAL: "),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getEnv = tc.getEnv
			err := loadEnvVariables()
			if tc.expectedError != nil {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
