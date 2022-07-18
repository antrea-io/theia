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
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/theia/pkg/theia/commands/config"
)

func TestGetServiceAddr(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		serviceName      string
		expectedIP       string
		expectedPort     int
		expectedErrorMsg string
	}{
		{
			name: "valid case",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clickhouse-clickhouse",
						Namespace: config.FlowVisibilityNS,
					},
					Spec: v1.ServiceSpec{
						Ports:     []v1.ServicePort{{Name: "tcp", Port: 9000}},
						ClusterIP: "10.98.208.26",
					},
				},
			),
			serviceName:      "clickhouse-clickhouse",
			expectedIP:       "10.98.208.26",
			expectedPort:     9000,
			expectedErrorMsg: "",
		},
		{
			name:             "service not found",
			fakeClientset:    fake.NewSimpleClientset(),
			serviceName:      "clickhouse-clickhouse",
			expectedIP:       "",
			expectedPort:     0,
			expectedErrorMsg: `error when finding the Service clickhouse-clickhouse: services "clickhouse-clickhouse" not found`,
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := GetServiceAddr(tt.fakeClientset, tt.serviceName)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			}
			assert.Equal(t, tt.expectedIP, ip)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestPolicyRecoPreCheck(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		expectedErrorMsg string
	}{
		{
			name: "valid case",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clickhouse",
						Namespace: config.FlowVisibilityNS,
						Labels:    map[string]string{"app": "clickhouse"},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				},
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "spark-operator",
						Namespace: config.FlowVisibilityNS,
						Labels: map[string]string{
							"app.kubernetes.io/name": "spark-operator",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				},
			),
			expectedErrorMsg: "",
		},
		{
			name:             "spark operator pod not found",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedErrorMsg: "can't find the policy-recommendation-spark-operator Pod, please check the deployment of the Spark Operator",
		},
		{
			name: "clickhouse pod not found",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "spark-operator",
						Namespace: config.FlowVisibilityNS,
						Labels: map[string]string{
							"app.kubernetes.io/name": "spark-operator",
						},
					},
					Status: v1.PodStatus{
						Phase: v1.PodRunning,
					},
				},
			),
			expectedErrorMsg: "can't find the ClickHouse Pod, please check the deployment of ClickHouse",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			err := PolicyRecoPreCheck(tt.fakeClientset)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
