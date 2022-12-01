// Copyright 2022 Antrea Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
