// Copyright 2023 Antrea Authors
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

package controller

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/theia/pkg/util/clickhouse"
	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

var (
	testNamespace = "controller-test"
)

func TestHandleStaleDbEntries(t *testing.T) {
	testCases := []struct {
		name             string
		setupClient      func(kubernetes.Interface)
		mock_arg_func    func(string, string) error
		getSparkJobIds   func(connect *sql.DB, tableName string) ([]string, error)
		expectedErrorMsg string
	}{
		{
			name: "Anomaly Detector Ids not found",
			mock_arg_func: func(str1 string, str2 string) error {
				return errors.New("mock_error")
			},
			getSparkJobIds: func(connect *sql.DB, tableName string) ([]string, error) {
				return []string{}, errors.New("mock_error")
			},
			expectedErrorMsg: "failed to get tadetector ids from ClickHouse",
		},
		{
			name: "Clickhouse Stale Entries present",
			mock_arg_func: func(str1 string, str2 string) error {
				return errors.New("mock_error")
			},
			getSparkJobIds: func(connect *sql.DB, tableName string) ([]string, error) {
				return []string{"mock_id"}, nil
			},
			expectedErrorMsg: "failed to remove all stale ClickHouse entries",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			db, _ := clickhouse.CreateFakeClickHouse(t, kubeClient, testNamespace)
			defer db.Close()
			getSparkJobIds = tc.getSparkJobIds
			err := HandleStaleDbEntries(db, kubeClient, "tadetector", "tadetector_local", tc.mock_arg_func, "tad-")
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestHandleStaleSparkApp(t *testing.T) {
	testCases := []struct {
		name                 string
		setupClient          func(kubernetes.Interface)
		mock_arg_func        func(string, string) error
		ListSparkApplication func(client kubernetes.Interface, sparkAppLabel string) (*sparkv1.SparkApplicationList, error)
		expectedErrorMsg     string
	}{
		{
			name: "Failed to list spark app",
			mock_arg_func: func(str1 string, str2 string) error {
				return errors.New("mock_error")
			},
			ListSparkApplication: func(client kubernetes.Interface, sparkAppLabel string) (*sparkv1.SparkApplicationList, error) {
				return &sparkv1.SparkApplicationList{}, errors.New("mock_error")
			},
			expectedErrorMsg: "failed to list Spark Application",
		},
		{
			name: "Failed to remove stale spark app",
			mock_arg_func: func(str1 string, str2 string) error {
				return errors.New("mock_error")
			},
			ListSparkApplication: func(client kubernetes.Interface, sparkAppLabel string) (*sparkv1.SparkApplicationList, error) {
				list := make([]sparkv1.SparkApplication, 1)
				saList := &sparkv1.SparkApplicationList{
					Items: list,
				}
				return saList, nil
			},
			expectedErrorMsg: "failed to remove stale Spark Applications and database entries",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			ListSparkApplication = tc.ListSparkApplication
			err := HandleStaleSparkApp(kubeClient, "tadetector", tc.mock_arg_func)
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestValidateCluster(t *testing.T) {
	testCases := []struct {
		name             string
		setupClient      func(kubernetes.Interface)
		expectedErrorMsg string
	}{
		{
			name:             "clickhouse pod not found",
			setupClient:      func(i kubernetes.Interface) {},
			expectedErrorMsg: "failed to find the ClickHouse Pod, please check the deployment",
		},
		{
			name: "spark operator pod not found",
			setupClient: func(client kubernetes.Interface) {
				db, _ := clickhouse.CreateFakeClickHouse(t, client, testNamespace)
				db.Close()
			},
			expectedErrorMsg: "failed to find the Spark Operator Pod, please check the deployment",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			tc.setupClient(kubeClient)
			err := ValidateCluster(kubeClient, testNamespace)
			assert.Contains(t, err.Error(), tc.expectedErrorMsg)
		})
	}
}

func TestGetSparkAppProgress(t *testing.T) {
	sparkAppID := "spark-application-id"
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedErrorMsg string
	}{
		{
			name: "more than one spark application",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedErrorMsg: "wrong Spark Application number, expected 1, got 2",
		},
		{
			name:             "no spark monitor service",
			testServer:       nil,
			expectedErrorMsg: "failed to get response from the Spark Monitoring Service",
		},
		{
			name: "Successful Case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				case "/api/v1/applications/spark-application-id/stages":
					responses := []map[string]interface{}{
						{"status": "COMPLETE"},
						{"status": "SKIPPED"},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedErrorMsg: "wrong Spark Application number, expected 1, got 2",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				err             error
				completedStages int
				totalStages     int
			)
			if tc.testServer != nil {
				defer tc.testServer.Close()
				completedStages, totalStages, err = GetSparkAppProgress(tc.testServer.URL)
			} else {
				_, _, err = GetSparkAppProgress("http://127.0.0.1")
			}
			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			} else {
				assert.Equal(t, 2, completedStages)
				assert.Equal(t, 2, totalStages)
			}
		})
	}
}

func TestGetSparkJobIds(t *testing.T) {
	testCases := []struct {
		name             string
		tableName        string
		expectedErrorMsg string
		returnedRow      *sqlmock.Rows
	}{
		{
			name:             "Failed to read from ClickHouse",
			tableName:        "mock_name",
			returnedRow:      sqlmock.NewRows([]string{}),
			expectedErrorMsg: "failed to read from ClickHouse",
		},
		{
			name:             "Success Case",
			tableName:        "tadetector",
			returnedRow:      sqlmock.NewRows([]string{"id"}).AddRow("mock_id"),
			expectedErrorMsg: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			query := fmt.Sprintf("SELECT DISTINCT id FROM " + tc.tableName + ";")
			if tc.tableName == "mock_name" {
				mock.ExpectQuery(query).WillReturnError(errors.New("mock_error"))
			} else {
				mock.ExpectQuery(query).WillReturnRows(tc.returnedRow)
			}
			var idlist []string
			idlist, err = GetSparkJobIds(db, tc.tableName)
			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			} else {
				assert.EqualValues(t, idlist, []string{"mock_id"})
			}
		})
	}
}

func TestRunClickHouseQuery(t *testing.T) {
	query := "mock_query"
	id := "mock_id"
	testCases := []struct {
		name             string
		expectedErrorMsg string
		returnedRow      *sqlmock.Rows
	}{
		{
			name:             "Failed to read from ClickHouse",
			returnedRow:      sqlmock.NewRows([]string{}),
			expectedErrorMsg: "query failed for Spark Application id",
		},
		{
			name:             "Success Case",
			returnedRow:      sqlmock.NewRows([]string{"id"}).AddRow(id),
			expectedErrorMsg: "",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			assert.NoError(t, err)
			if tc.name != "Success Case" {
				mock.ExpectQuery(query).WillReturnError(errors.New("mock_error"))
			} else {
				mock.ExpectQuery(query).WillReturnRows(tc.returnedRow)
			}
			err = RunClickHouseQuery(db, query, id)
			if err != nil {
				assert.Contains(t, err.Error(), tc.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
