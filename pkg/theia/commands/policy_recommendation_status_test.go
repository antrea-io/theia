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
	"errors"
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

	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestPolicyRecommendationStatus(t *testing.T) {
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedMsg      []string
		expectedErrorMsg string
		nprName          string
	}{
		{
			name: "Valid case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations/%s", nprName):
					npr := &intelligence.NetworkPolicyRecommendation{
						Status: intelligence.NetworkPolicyRecommendationStatus{
							State:           "RUNNING",
							CompletedStages: 1,
							TotalStages:     5,
							ErrorMsg:        "testErrorMsg",
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(npr)
				}
			})),
			nprName: nprName,
			expectedMsg: []string{
				"Status of this policy recommendation job is RUNNING: 1/5 (20%) stages completed",
				"Error message: testErrorMsg",
			},
			expectedErrorMsg: "",
		},
		{
			name: "total stage is zero ",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations/%s", nprName):
					npr := &intelligence.NetworkPolicyRecommendation{
						Status: intelligence.NetworkPolicyRecommendationStatus{
							State:       "RUNNING",
							TotalStages: 0,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(npr)
				}
			})),
			nprName:          nprName,
			expectedMsg:      []string{"Status of this policy recommendation job is RUNNING: 0/0 (0%) stages completed"},
			expectedErrorMsg: "",
		},
		{
			name: "NetworkPolicyRecommendation not found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations/%s", nprName):
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				}
			})),
			nprName:          nprName,
			expectedMsg:      []string{},
			expectedErrorMsg: "error when getting policy recommendation job",
		},
		{
			name:             "Unspecified name",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			nprName:          nprName,
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unspecified use-cluster-ip",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			nprName:          nprName,
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Invalid nprName",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			nprName:          "mock_nprName",
			expectedMsg:      []string{},
			expectedErrorMsg: "not a valid policy recommendation job name",
		},
		{
			name:             TheiaClientSetupDeniedTestCase,
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			nprName:          nprName,
			expectedMsg:      []string{},
			expectedErrorMsg: TheiaClientSetupDeniedErr,
		},
		{
			name: "Valid case with args",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case fmt.Sprintf("/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations/%s", nprName):
					npr := &intelligence.NetworkPolicyRecommendation{
						Status: intelligence.NetworkPolicyRecommendationStatus{
							State:       "RUNNING",
							TotalStages: 0,
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(npr)
				}
			})),
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
			switch tt.name {
			case "Unspecified name":
				cmd.Flags().Bool("use-cluster-ip", true, "")
			case "Unspecified use-cluster-ip":
				cmd.Flags().String("name", tt.nprName, "")
			default:
				cmd.Flags().String("name", tt.nprName, "")
				cmd.Flags().Bool("use-cluster-ip", true, "")
			}

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			if tt.name == "Valid case with args" {
				err = policyRecommendationStatus(cmd, []string{nprName})
			} else {
				err = policyRecommendationStatus(cmd, []string{})
			}
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
