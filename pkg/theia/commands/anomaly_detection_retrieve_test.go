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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestAnomalyDetectorRetrieve(t *testing.T) {
	tadName := "tad-1234abcd-1234-abcd-12ab-12345678abcd"
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedMsg      []string
		expectedErrorMsg string
		tadName          string
		filePath         string
	}{
		{
			name: "Valid case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:       "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly:  "true",
							AlgoCalc: "1234567",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"1234567\t\ttrue"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case for No Anomaly Found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:      "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly: "NO ANOMALY DETECTED",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"No Anomaly found"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case with filePath",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:      "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly: "[true]",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{},
			expectedErrorMsg: "",
			filePath:         "/tmp/testResult",
		},
		{
			name: "Throughput Anomaly Detection not found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/anomalydetector.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{},
			expectedErrorMsg: "error when getting anomaly detection job",
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
			cmd.Flags().String("file", tt.filePath, "")
			cmd.Flags().String("name", tt.tadName, "")

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := throughputAnomalyDetectionRetrieve(cmd, []string{})
			if tt.expectedErrorMsg == "" {
				if tt.filePath != "" {
					result, err := os.ReadFile(tt.filePath)
					assert.NoError(t, err)
					for _, msg := range tt.expectedMsg {
						assert.Contains(t, string(result), msg)
					}
					defer os.RemoveAll(tt.filePath)
				} else {
					assert.NoError(t, err)
					outcome := readStdouttad(t, r, w)
					os.Stdout = orig
					for _, msg := range tt.expectedMsg {
						assert.Contains(t, outcome, msg)
					}
				}
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}

func readStdouttad(t *testing.T, r *os.File, w *os.File) string {
	var buf bytes.Buffer
	exit := make(chan bool)
	go func() {
		_, _ = io.Copy(&buf, r)
		exit <- true
	}()
	err := w.Close()
	assert.NoError(t, err)
	<-exit
	err = r.Close()
	assert.NoError(t, err)
	return buf.String()
}
