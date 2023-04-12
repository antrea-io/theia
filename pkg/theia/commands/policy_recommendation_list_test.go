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
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/portforwarder"
)

func TestPolicyRecommendationList(t *testing.T) {
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
				case "/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations":
					nprList := &intelligence.NetworkPolicyRecommendationList{
						Items: []intelligence.NetworkPolicyRecommendation{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "pr-test1",
								},
								Status: intelligence.NetworkPolicyRecommendationStatus{
									SparkApplication: "test1",
								}},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(nprList)
				}
			})),
			expectedMsg:      []string{"pr-test1"},
			expectedErrorMsg: "",
		},
		{
			name: "NetworkPolicyRecommendationList not found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/apis/intelligence.theia.antrea.io/v1alpha1/networkpolicyrecommendations":
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
				}
			})),
			expectedMsg:      []string{},
			expectedErrorMsg: "error when getting policy recommendation job list",
		},
		{
			name:             "Unspecified use-cluster-ip",
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
			expectedMsg:      []string{},
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             TheiaClientSetupDeniedTestCase,
			testServer:       httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})),
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
			if tt.name != "Unspecified use-cluster-ip" {
				cmd.Flags().Bool("use-cluster-ip", true, "")
			}

			orig := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := policyRecommendationList(cmd, []string{})
			if tt.expectedErrorMsg == "" {
				assert.NoError(t, err)
				outcome := readStdout(t, r, w)
				os.Stdout = orig
				assert.Contains(t, outcome, "test1")
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
