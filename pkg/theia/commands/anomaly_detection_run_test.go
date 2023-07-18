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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	anomalydetector "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
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
				case "/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors":
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
				case "/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors":
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				}
			})),
			expectedMsg:      []string{},
			expectedErrorMsg: "failed to Post Throughput Anomaly Detection job",
		},
		{
			name:             TheiaClientSetupDeniedTestCase,
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			expectedMsg:      []string{},
			expectedErrorMsg: TheiaClientSetupDeniedErr,
		},
		{
			name: "Valid case with args",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/intelligence.theia.antrea.io/v1alpha1/throughputanomalydetectors":
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
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var err error
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
			cmd.Flags().Bool("use-cluster-ip", true, "")
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().String("agg-flow", "external", "")
			cmd.Flags().String("pod-label", "app:label1", "")
			cmd.Flags().String("pod-name", "testpodname", "")
			cmd.Flags().String("pod-namespace", "testpodnamespace", "")
			cmd.Flags().String("external-ip", "10.0.0.1", "")
			cmd.Flags().String("svc-port-name", "testportname", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			if tt.name == "Valid case with args" {
				err = throughputAnomalyDetectionAlgo(cmd, []string{"tadName"})
			} else {
				err = throughputAnomalyDetectionAlgo(cmd, []string{})
			}
			if tt.expectedErrorMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}

func TestThroughputAnomalyDetectionAlgo(t *testing.T) {
	testCases := []struct {
		name             string
		expectedErrorMsg string
	}{
		{
			name:             "Unspecified Algo",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid Algo",
			expectedErrorMsg: "not a valid Throughput Anomaly Detection algorithm name",
		},
		{
			name:             "Unspecified start-time",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid start-time",
			expectedErrorMsg: "start-time should be in \n'YYYY-MM-DDThh:mm:ss' format, for example: 2006-01-02T15:04:05",
		},
		{
			name:             "Unspecified end-time",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid end-time",
			expectedErrorMsg: "end-time should be in \n'YYYY-MM-DDThh:mm:ss' format, for example: 2006-01-02T15:04:05",
		},
		{
			name:             "end-time before start-time",
			expectedErrorMsg: "end-time should be after start-time",
		},
		{
			name:             "Unspecified ns-ignore-list",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid ns-ignore-list",
			expectedErrorMsg: "ns-ignore-list should \nbe a list of namespace string",
		},
		{
			name:             "Unspecified executor-instances",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid executor-instances",
			expectedErrorMsg: "executor-instances should be an integer >= 0",
		},
		{
			name:             "Unspecified driver-core-request",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid driver-core-request",
			expectedErrorMsg: "driver-core-request should conform to the Kubernetes resource quantity convention",
		},
		{
			name:             "Unspecified driver-memory",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid driver-memory",
			expectedErrorMsg: "driver-memory should conform to the Kubernetes resource quantity convention",
		},
		{
			name:             "Unspecified executor-core-request",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid executor-core-request",
			expectedErrorMsg: "executor-core-request should conform to the Kubernetes resource quantity convention",
		},
		{
			name:             "Unspecified executor-memory",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid executor-memory",
			expectedErrorMsg: "executor-memory should conform to the Kubernetes resource quantity convention",
		},
		{
			name:             "Unspecified agg-flow",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified pod-label",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified pod-name",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified pod-namespace",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid pod-namespace",
			expectedErrorMsg: "argument can not be used alone",
		},
		{
			name:             "Unspecified external-ip",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified svc-port-name",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid agg-flow",
			expectedErrorMsg: "aggregated flow type should be 'pod' or 'external' or 'svc'",
		},
		{
			name:             "Unspecified use-cluster-ip",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
	}
	for _, tt := range testCases {
		cmd := new(cobra.Command)
		switch tt.name {
		case "Invalid Algo":
			cmd.Flags().String("algo", "mock_wrong_Algo", "")
		case "Unspecified start-time":
			cmd.Flags().String("algo", "ARIMA", "")
		case "Invalid start-time":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "mock_wrongtime", "")
		case "Unspecified end-time":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-03 15:04:05", "")
		case "Invalid end-time":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-03 15:04:05", "")
			cmd.Flags().String("end-time", "mock_wrongtime", "")
		case "end-time before start-time":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-03 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 15:04:05", "")
		case "Unspecified ns-ignore-list":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-03 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
		case "Invalid ns-ignore-list":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "mock_error", "")
		case "Unspecified executor-instances":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
		case "Invalid executor-instances":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", -1, "")
		case "Unspecified driver-core-request":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
		case "Invalid driver-core-request":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "mock_driver-core-request", "")
		case "Unspecified driver-memory":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
		case "Invalid driver-memory":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "mock_driver-memory", "")
		case "Unspecified executor-core-request":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
		case "Invalid executor-core-request":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "mock_executor-core-request", "")
		case "Unspecified executor-memory":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
		case "Invalid executor-memory":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "mock_executor-memory", "")
		case "Unspecified agg-flow":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
		case "Unspecified pod-label":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "pod", "")
		case "Unspecified pod-name":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "pod", "")
			cmd.Flags().String("pod-label", "mock_pod-label", "")
		case "Unspecified pod-namespace":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "pod", "")
			cmd.Flags().String("pod-label", "mock_pod_label", "")
			cmd.Flags().String("pod-name", "mock_pod-name", "")
		case "Invalid pod-namespace":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "pod", "")
			cmd.Flags().String("pod-label", "", "")
			cmd.Flags().String("pod-name", "", "")
			cmd.Flags().String("pod-namespace", "mock_pod-namespace", "")
		case "Unspecified external-ip":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "external", "")
		case "Unspecified svc-port-name":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "svc", "")
		case "Invalid agg-flow":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "mock_agg-flow", "")
		case "Unspecified use-cluster-ip":
			cmd.Flags().String("algo", "ARIMA", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 16:04:05", "")
			cmd.Flags().String("ns-ignore-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().String("agg-flow", "svc", "")
			cmd.Flags().String("svc-port-name", "mock_svc_name", "")
		}
		err := throughputAnomalyDetectionAlgo(cmd, []string{})
		if tt.expectedErrorMsg == "" {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrorMsg)
		}
	}
}
