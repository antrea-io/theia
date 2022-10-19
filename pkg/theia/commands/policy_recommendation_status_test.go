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
	nprName := "pr-e292395c-3de1-11ed-b878-0242ac120002"
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
			nprName: "pr-e292395c-3de1-11ed-b878-0242ac120002",
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
			nprName:          "pr-e292395c-3de1-11ed-b878-0242ac120002",
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
			nprName:          "pr-e292395c-3de1-11ed-b878-0242ac120001",
			expectedMsg:      []string{},
			expectedErrorMsg: "error when getting policy recommendation job",
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
			cmd.Flags().String("name", tt.nprName, "")

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := policyRecommendationStatus(cmd, []string{})
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
