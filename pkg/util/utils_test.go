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

package util

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/theia/pkg/theia/commands/config"
)

var (
	serviceName      = "service"
	serviceNamespace = "serviceNamespace"
	port             = 9000
	protocol         = v1.ProtocolTCP
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
						Name:      serviceName,
						Namespace: serviceNamespace,
					},
					Spec: v1.ServiceSpec{
						Ports:     []v1.ServicePort{{Port: int32(port), Protocol: protocol}},
						ClusterIP: "10.98.208.26",
					},
				},
			),
			expectedIP:       "10.98.208.26",
			expectedPort:     port,
			expectedErrorMsg: "",
		},
		{
			name: "service port not found",
			fakeClientset: fake.NewSimpleClientset(
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: serviceNamespace,
					},
				},
			),
			expectedIP:       "",
			expectedPort:     0,
			expectedErrorMsg: fmt.Sprintf("error when finding the Service %s: no %s service port", serviceName, protocol),
		},
		{
			name:             "service not found",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedIP:       "",
			expectedPort:     0,
			expectedErrorMsg: fmt.Sprintf("error when finding the Service %s: services \"%s\" not found", serviceName, serviceName),
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ip, port, err := GetServiceAddr(tt.fakeClientset, serviceName, serviceNamespace, protocol)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			}
			assert.Equal(t, tt.expectedIP, ip)
			assert.Equal(t, tt.expectedPort, port)
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
			expectedErrorMsg: "error when finding the ClickHouse secret. Error: secrets \"clickhouse-secret\" not found",
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
			username, password, err := GetClickHouseSecret(tt.fakeClientset, config.FlowVisibilityNS)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			}
			assert.Equal(t, tt.expectedUsername, string(username))
			assert.Equal(t, tt.expectedPassword, string(password))
		})
	}
}
