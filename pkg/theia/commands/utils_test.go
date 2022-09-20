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

	"antrea.io/theia/pkg/apis"
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
						Name:      config.TheiaManagerServiceName,
						Namespace: config.FlowVisibilityNS,
					},
					Spec: v1.ServiceSpec{
						Ports:     []v1.ServicePort{{Port: apis.TheiaManagerAPIPort, Protocol: "TCP"}},
						ClusterIP: "10.98.208.26",
					},
				},
			),
			serviceName:      config.TheiaManagerServiceName,
			expectedIP:       "10.98.208.26",
			expectedPort:     apis.TheiaManagerAPIPort,
			expectedErrorMsg: "",
		},
		{
			name:             "service not found",
			fakeClientset:    fake.NewSimpleClientset(),
			serviceName:      config.TheiaManagerServiceName,
			expectedIP:       "",
			expectedPort:     0,
			expectedErrorMsg: `error when finding the Service theia-manager: services "theia-manager" not found`,
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

func TestGetClickHouseSecret(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		expectedUsername string
		expectedPassword string
		expectedErrorMsg string
	}{
		{
			name: "valid case",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clickhouse-secret",
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{
						"username": []byte("clickhouse_operator"),
						"password": []byte("clickhouse_operator_password"),
					},
				},
			),
			expectedUsername: "clickhouse_operator",
			expectedPassword: "clickhouse_operator_password",
			expectedErrorMsg: "",
		},
		{
			name:             "clickhouse secret not found",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedUsername: "",
			expectedPassword: "",
			expectedErrorMsg: `error secrets "clickhouse-secret" not found when finding the ClickHouse secret, please check the deployment of ClickHouse`,
		},
		{
			name: "username not found",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clickhouse-secret",
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{
						"password": []byte("clickhouse_operator_password"),
					},
				},
			),
			expectedUsername: "",
			expectedPassword: "",
			expectedErrorMsg: "error when getting the ClickHouse username",
		},
		{
			name: "password not found",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clickhouse-secret",
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{
						"username": []byte("clickhouse_operator"),
					},
				},
			),
			expectedUsername: "clickhouse_operator",
			expectedPassword: "",
			expectedErrorMsg: "error when getting the ClickHouse password",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			username, password, err := getClickHouseSecret(tt.fakeClientset)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			}
			assert.Equal(t, tt.expectedUsername, string(username))
			assert.Equal(t, tt.expectedPassword, string(password))
		})
	}
}

func TestGetCaCrt(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		expectedErrorMsg string
		expectedCaCrt    string
	}{
		{
			name: "Valid case",
			fakeClientset: fake.NewSimpleClientset(
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.CAConfigMapName,
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string]string{
						config.CAConfigMapKey: "key",
					},
				},
			),
			expectedErrorMsg: "",
			expectedCaCrt:    "key",
		},
		{
			name:             "Not found",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedErrorMsg: "error when getting ConfigMap theia-ca",
			expectedCaCrt:    "",
		},
		{
			name: "No data in configmap",
			fakeClientset: fake.NewSimpleClientset(
				&v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.CAConfigMapName,
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string]string{},
				},
			),
			expectedErrorMsg: "error when checking ca.crt in data",
			expectedCaCrt:    "",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			caCrt, err := GetCaCrt(tt.fakeClientset)
			if tt.expectedErrorMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
			assert.Equal(t, tt.expectedCaCrt, caCrt)
		})
	}
}

func TestGetToken(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		expectedErrorMsg string
		expectedToken    string
	}{
		{
			name: "Valid case",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.TheiaCliAccountName,
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{
						config.ServiceAccountTokenKey: []byte("tokenTest"),
					},
				},
			),
			expectedErrorMsg: "",
			expectedToken:    "tokenTest",
		},
		{
			name:             "Not found",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedErrorMsg: "error when getting secret",
			expectedToken:    "",
		},
		{
			name: "No data in secret",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.TheiaCliAccountName,
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{},
				},
			),
			expectedErrorMsg: "secret 'theia-cli-account-token' does not include token",
			expectedToken:    "",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			caCrt, err := GetToken(tt.fakeClientset)
			if tt.expectedErrorMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
			assert.Equal(t, tt.expectedToken, caCrt)
		})
	}
}
