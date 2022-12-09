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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	stats "antrea.io/theia/pkg/apis/stats/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestGetStatus(t *testing.T) {
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedErrorMsg string
		expectedMsg      []string
		options          *chOptions
	}{
		{
			name: "Get diskInfo",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/stats.theia.antrea.io/v1alpha1/clickhouse/diskInfo":
					status := &stats.ClickHouseStats{
						DiskInfos: []stats.DiskInfo{{
							Shard:          "Shard_test",
							Database:       "Database_test",
							Path:           "Path_test",
							FreeSpace:      "FreeSpace_test",
							TotalSpace:     "TotalSpace",
							UsedPercentage: "UsedPercentage_test",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{diskInfo: true},
			expectedErrorMsg: "",
			expectedMsg: []string{"Shard", "DatabaseName", "Path", "Free", "Total", "Used_Percentage",
				"Shard_test", "Database_test", "Path_test", "FreeSpace_test", "TotalSpace", "UsedPercentage_test"},
		},
		{
			name: "Get tableInfo",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/stats.theia.antrea.io/v1alpha1/clickhouse/tableInfo":
					status := &stats.ClickHouseStats{
						TableInfos: []stats.TableInfo{{
							Shard:      "Shard_test",
							Database:   "Database_test",
							TableName:  "TableName_test",
							TotalRows:  "TotalRows_test",
							TotalBytes: "TotalBytes_test",
							TotalCols:  "TotalCols_test",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{tableInfo: true},
			expectedErrorMsg: "",
			expectedMsg: []string{"Shard", "DatabaseName", "TableName", "TotalRows", "TotalBytes", "TotalCols",
				"Shard_test", "Database_test", "TableName_test", "TotalRows_test", "TotalBytes_test", "TotalCols_test"},
		},
		{
			name: "Get insertRate",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/stats.theia.antrea.io/v1alpha1/clickhouse/insertRate":
					status := &stats.ClickHouseStats{
						InsertRates: []stats.InsertRate{{
							Shard:       "Shard_test",
							RowsPerSec:  "RowsPerSec_test",
							BytesPerSec: "RowsPerSec_test",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{insertRate: true},
			expectedErrorMsg: "",
			expectedMsg: []string{"Shard", "RowsPerSecond", "RowsPerSecond",
				"Shard_test", "RowsPerSec_test", "RowsPerSec_test"},
		},
		{
			name: "Get stackTraces",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/stats.theia.antrea.io/v1alpha1/clickhouse/stackTrace":
					status := &stats.ClickHouseStats{
						StackTraces: []stats.StackTrace{{
							Shard:          "Shard_test",
							TraceFunctions: "TraceFunctions_test",
							Count:          "Count_test",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{stackTrace: true},
			expectedErrorMsg: "",
			expectedMsg: []string{"Shard", "TraceFunctions", "Count()",
				"Shard_test", "TraceFunctions_test", "Count_test"},
		},
		{
			name:             "No metrics specified",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			options:          &chOptions{},
			expectedErrorMsg: "no metric related flag is specified",
			expectedMsg:      nil,
		},
		{
			name: "ErrorMsg in response is not empty",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/stats.theia.antrea.io/v1alpha1/clickhouse/stackTrace":
					status := &stats.ClickHouseStats{
						StackTraces: []stats.StackTrace{{
							Shard:          "Shard_test",
							TraceFunctions: "TraceFunctions_test",
							Count:          "Count_test",
						}},
						ErrorMsg: []string{"converting NULL to string is unsupported"},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{stackTrace: true},
			expectedErrorMsg: "",
			expectedMsg: []string{"Shard", "TraceFunctions", "Count()",
				"Shard_test", "TraceFunctions_test", "Count_test",
				"converting NULL to string is unsupported"},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.testServer.Close()
			oldFunc := SetupTheiaClientAndConnection
			SetupTheiaClientAndConnection = func(cmd *cobra.Command, useClusterIP bool) (restclient.Interface, *portforwarder.PortForwarder, error) {
				clientConfig := &restclient.Config{Host: tt.testServer.URL, TLSClientConfig: restclient.TLSClientConfig{Insecure: true}}
				clientset, _ := kubernetes.NewForConfig(clientConfig)
				return clientset.CoreV1().RESTClient(), nil, nil
			}
			defer func() {
				SetupTheiaClientAndConnection = oldFunc
			}()
			cmd := new(cobra.Command)
			cmd.Flags().Bool("use-cluster-ip", true, "")
			options = tt.options

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := getStatus(cmd, []string{})
			if tt.expectedErrorMsg == "" {
				assert.NoError(t, err)
				outcome := readStdout(t, r, w)
				os.Stdout = orig
				for _, msg := range tt.expectedMsg {
					assert.Contains(t, outcome, msg)
				}
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}
