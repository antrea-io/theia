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

package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestAnomalyDetectionRun(t *testing.T) {
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedMsg      []string
		expectedErrorMsg string
	}{
		{
			name: "Valid case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
				}
				if r.Method == "GET" && strings.Contains(r.URL.Path, "throughputanomalydetectors/tad-") {
					npr := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(npr)
				}
			})),
			expectedMsg: []string{
				"testOutcome",
			},
			expectedErrorMsg: "",
		},
		{
			name: "Failed to Post Throughput Anomaly Detection job",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors":
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				}
			})),
			expectedMsg:      []string{},
			expectedErrorMsg: "failed to Post Throughput Anomaly Detection job",
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
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 15:04:05", "")
			cmd.Flags().Bool("exclude-labels", true, "")
			cmd.Flags().Bool("to-services", true, "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")

			err := throughputAnomalyDetectionAlgo(cmd, []string{})
			if tt.expectedErrorMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}
