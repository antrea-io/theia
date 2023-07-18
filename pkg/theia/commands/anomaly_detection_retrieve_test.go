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
	"errors"
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

	anomalydetector "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestAnomalyDetectorRetrieve(t *testing.T) {
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedMsg      []string
		expectedErrorMsg string
		tadName          string
		filePath         string
	}{
		{
			name: "Valid case No agg_type",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:       tadName,
							Anomaly:  "true",
							AlgoCalc: "1234567",
							AggType:  "None",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"id                                       sourceIP       sourceTransportPort destinationIP  destinationTransportPort flowStartSeconds flowEndSeconds throughput     aggType        algoType       algoCalc       anomaly", "None                          1234567        true"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case agg_type: external",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:       "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly:  "true",
							AlgoCalc: "1234567",
							AggType:  "external",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"id                                       destinationIP  flowEndSeconds throughput     aggType        algoType       algoCalc       anomaly", "external                      1234567        true"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case agg_type: pod podlabels",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:        "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly:   "true",
							AlgoCalc:  "1234567",
							AggType:   "pod",
							PodLabels: "testlabels",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"id                                       podNamespace   podLabels      direction      flowEndSeconds throughput     aggType        algoType       algoCalc       anomaly", "pod                           1234567        true"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case agg_type: pod podname",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:       "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly:  "true",
							AlgoCalc: "1234567",
							AggType:  "pod",
							PodName:  "testpodname",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"id                                       podNamespace   podName        direction      flowEndSeconds throughput     aggType        algoType       algoCalc       anomaly", "pod                           1234567        true"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case agg_type: svc",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:       "tad-1234abcd-1234-abcd-12ab-12345678abcd",
							Anomaly:  "true",
							AlgoCalc: "1234567",
							AggType:  "svc",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          "tad-1234abcd-1234-abcd-12ab-12345678abcd",
			expectedMsg:      []string{"id                                       destinationServicePortName flowEndSeconds throughput     aggType        algoType       algoCalc       anomaly", "svc                           1234567        true"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case for No Anomaly Found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:      tadName,
							Anomaly: "NO ANOMALY DETECTED",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          tadName,
			expectedMsg:      []string{"No Anomaly found"},
			expectedErrorMsg: "",
		},
		{
			name: "Valid case with filePath",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					tad := &anomalydetector.ThroughputAnomalyDetector{
						Status: anomalydetector.ThroughputAnomalyDetectorStatus{
							State: "COMPLETED",
						},
						Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
							Id:      tadName,
							Anomaly: "[true]",
						}},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(tad)
				}
			})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: "",
			filePath:         "/tmp/testResult",
		},
		{
			name: "Throughput Anomaly Detection not found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors/%s", tadName):
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				}
			})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: "error when getting anomaly detection job",
		},
		{
			name:             "Unspecified name",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified file",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified use-cluster-ip",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid tadName",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			tadName:          "mock_tadName",
			expectedMsg:      []string{},
			expectedErrorMsg: "not a valid Throughput Anomaly Detection job name",
		},
		{
			name:             TheiaClientSetupDeniedTestCase,
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			tadName:          tadName,
			expectedMsg:      []string{},
			expectedErrorMsg: TheiaClientSetupDeniedErr,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.testServer.Close()
			oldFunc := SetupTheiaClientAndConnection
			if tt.name == TheiaClientSetupDeniedTestCase {
				SetupTheiaClientAndConnection = func(cmd *cobra.Command, useClusterIP bool) (restclient.Interface, *portforwarder.PortForwarder, error) {
					return nil, nil, errors.New("mock_error")
				}
			} else {
				SetupTheiaClientAndConnection = func(cmd *cobra.Command, useClusterIP bool) (restclient.Interface, *portforwarder.PortForwarder, error) {
					clientConfig := &restclient.Config{Host: tt.testServer.URL, TLSClientConfig: restclient.TLSClientConfig{Insecure: true}}
					clientset, _ := kubernetes.NewForConfig(clientConfig)
					return clientset.CoreV1().RESTClient(), nil, nil
				}
			}
			defer func() {
				SetupTheiaClientAndConnection = oldFunc
			}()
			cmd := new(cobra.Command)
			switch tt.name {
			case "Unspecified name":
				cmd.Flags().String("file", tt.filePath, "")
			case "Unspecified file":
				cmd.Flags().String("name", tt.tadName, "")
			case "Unspecified use-cluster-ip":
				cmd.Flags().String("name", tt.tadName, "")
				cmd.Flags().String("file", tt.filePath, "")
			default:
				cmd.Flags().String("name", tt.tadName, "")
				cmd.Flags().String("file", tt.filePath, "")
				cmd.Flags().Bool("use-cluster-ip", true, "")
			}

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
