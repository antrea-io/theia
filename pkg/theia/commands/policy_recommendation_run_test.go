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

	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestPolicyRecommendationRun(t *testing.T) {
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedMsg      []string
		expectedErrorMsg string
		waitFlag         bool
	}{
		{
			name: "Valid case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
				}
				if r.Method == "GET" && strings.Contains(r.URL.Path, "networkpolicyrecommendations/pr-") {
					npr := &intelligence.NetworkPolicyRecommendation{
						Status: intelligence.NetworkPolicyRecommendationStatus{
							State:                 "COMPLETED",
							RecommendationOutcome: "testOutcome",
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
			waitFlag:         true,
		},
		{
			name: "waitFlag is false",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations":
					if r.Method != "POST" {
						http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
				}
			})),
			expectedMsg: []string{
				"Successfully created policy recommendation job with name",
			},
			expectedErrorMsg: "",
		},
		{
			name: "Fail to post policy recommendation job",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations":
					http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				}
			})),
			expectedMsg:      []string{},
			expectedErrorMsg: "failed to post policy recommendation job",
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
			cmd.Flags().String("type", "initial", "")
			cmd.Flags().Int("limit", 0, "")
			cmd.Flags().String("policy-type", "anp-deny-applied", "")
			cmd.Flags().String("start-time", "2006-01-02 15:04:05", "")
			cmd.Flags().String("end-time", "2006-01-03 15:04:05", "")
			cmd.Flags().String("ns-allow-list", "[\"kube-system\",\"flow-aggregator\",\"flow-visibility\"]", "")
			cmd.Flags().Bool("exclude-labels", true, "")
			cmd.Flags().Bool("to-services", true, "")
			cmd.Flags().Int32("executor-instances", 1, "")
			cmd.Flags().String("driver-core-request", "1", "")
			cmd.Flags().String("driver-memory", "1m", "")
			cmd.Flags().String("executor-core-request", "1", "")
			cmd.Flags().String("executor-memory", "1m", "")
			cmd.Flags().Bool("wait", tt.waitFlag, "")
			cmd.Flags().String("file", "", "")

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := policyRecommendationRun(cmd, []string{})
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
