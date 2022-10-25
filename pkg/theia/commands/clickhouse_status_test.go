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
	"fmt"
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
		expectedMsg      string
		options          *chOptions
	}{
		{
			name: "Get diskInfo",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/stats.theia.antrea.io/v1alpha1/clickhouse/diskInfo"):
					status := &stats.ClickHouseStats{
						Result: [][]string{{"test_diskInfo"}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{diskInfo: true},
			expectedErrorMsg: "",
			expectedMsg:      "test_diskInfo",
		},
		{
			name: "Get tableInfo",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/stats.theia.antrea.io/v1alpha1/clickhouse/tableInfo"):
					status := &stats.ClickHouseStats{
						Result: [][]string{{"test_tableInfo"}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{tableInfo: true},
			expectedErrorMsg: "",
			expectedMsg:      "test_tableInfo",
		},
		{
			name: "Get insertRate",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/stats.theia.antrea.io/v1alpha1/clickhouse/insertRate"):
					status := &stats.ClickHouseStats{
						Result: [][]string{{"test_insertRate"}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{insertRate: true},
			expectedErrorMsg: "",
			expectedMsg:      "test_insertRate",
		},
		{
			name: "Get stackTraces",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/stats.theia.antrea.io/v1alpha1/clickhouse/stackTraces"):
					status := &stats.ClickHouseStats{
						Result: [][]string{{"test_stackTraces"}, {"fakeData"}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(status)
				}
			})),
			options:          &chOptions{stackTraces: true},
			expectedErrorMsg: "",
			expectedMsg:      "test_stackTraces: fakeData",
		},
		{
			name:             "No metrics specified",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			options:          &chOptions{},
			expectedErrorMsg: "no metric related flag is specified",
			expectedMsg:      "",
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
				assert.Contains(t, outcome, tt.expectedMsg)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}
