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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPolicyRecommendationProgress(t *testing.T) {
	sparkAppID := "spark-0fa6cc19ae23439794747a306d5ad705"
	testCases := []struct {
		name             string
		testServer       *httptest.Server
		expectedProgress string
		expectedErrorMsg string
	}{
		{
			name: "valid case",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				case fmt.Sprintf("/api/v1/applications/%s/stages", sparkAppID):
					responses := []map[string]interface{}{
						{"status": "COMPLETE"},
						{"status": "COMPLETE"},
						{"status": "SKIPPED"},
						{"status": "PENDING"},
						{"status": "ACTIVE"},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedProgress: ": 3/5 (60%) stages completed",
			expectedErrorMsg: "",
		},
		{
			name: "found more than one spark application",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedProgress: "",
			expectedErrorMsg: "wrong Spark Application number, expected 1, got 2",
		},
		{
			name: "no spark application stage found",
			testServer: httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch strings.TrimSpace(r.URL.Path) {
				case "/api/v1/applications":
					responses := []map[string]interface{}{
						{"id": sparkAppID},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				case fmt.Sprintf("/api/v1/applications/%s/stages", sparkAppID):
					responses := []map[string]interface{}{}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(responses)
				}
			})),
			expectedProgress: "",
			expectedErrorMsg: "wrong Spark Application stages number, expected at least 1, got 0",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			defer tt.testServer.Close()
			progress, err := getPolicyRecommendationProgress(tt.testServer.URL)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedProgress, progress)
		})
	}
}
