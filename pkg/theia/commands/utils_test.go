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

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/theia/pkg/apis"
	"antrea.io/theia/pkg/theia/commands/config"
)

const (
	ErrorMsgUnspecifiedCase        = "flag accessed but not defined"
	TheiaClientSetupDeniedTestCase = "Unable to setup theia manager client"
	TheiaClientSetupDeniedErr      = "couldn't setup Theia manager client"
	tadName                        = "tad-1234abcd-1234-abcd-12ab-12345678abcd"
	nprName                        = "pr-e292395c-3de1-11ed-b878-0242ac120002"
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

func TestSetupTheiaClientAndConnection(t *testing.T) {
	testCases := []struct {
		name             string
		expectedErrorMsg string
	}{
		{
			name:             "Valid case",
			expectedErrorMsg: "",
		},
		{
			name:             "Unspecified kubeconfig",
			expectedErrorMsg: ErrorMsgUnspecifiedCase,
		},
		{
			name:             "Unable to create k8sclient",
			expectedErrorMsg: "couldn't create k8s client using given kubeconfig",
		},
		{
			name:             "Unable to create TheiaManagerClient",
			expectedErrorMsg: "couldn't create Theia manager client",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			oldFunc := CreateK8sClient
			cmd := new(cobra.Command)
			switch tt.name {
			case "Unspecified kubeconfig":
				cmd.Flags().String("mock_arg", "", "")
			case "Unable to create k8sclient":
				cmd.Flags().String("kubeconfig", "mock_wrong_path", "")
			case "Unable to create TheiaManagerClient":
				cmd.Flags().String("kubeconfig", "", "")
				fakeClientset := fake.NewSimpleClientset()
				CreateK8sClient = func(kubeconfig string) (client kubernetes.Interface, err error) {
					return fakeClientset, nil
				}
			default:
				cmd.Flags().String("kubeconfig", "", "")
			}
			defer func() {
				CreateK8sClient = oldFunc
			}()
			_, _, err := SetupTheiaClientAndConnection(cmd, false)
			if tt.expectedErrorMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}

func TestCreateTheiaManagerClient(t *testing.T) {
	testCases := []struct {
		name             string
		fakeClientset    *fake.Clientset
		expectedErrorMsg string
	}{
		{
			name:             "No ca-crt",
			fakeClientset:    fake.NewSimpleClientset(),
			expectedErrorMsg: "error when getting ca-crt",
		},
		{
			name: "No token",
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
			expectedErrorMsg: "error when getting token",
		},
		{
			name: "No Service address",
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
			expectedErrorMsg: "error when getting the Theia Manager Service address",
		},
		{
			name: "no root certificate",
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
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.TheiaCliAccountName,
						Namespace: config.FlowVisibilityNS,
					},
					Data: map[string][]byte{
						config.ServiceAccountTokenKey: []byte("tokenTest"),
					},
				},
				&v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      config.TheiaManagerServiceName,
						Namespace: config.FlowVisibilityNS,
					},
					Spec: v1.ServiceSpec{
						Ports:     []v1.ServicePort{{Port: int32(apis.TheiaManagerAPIPort), Protocol: v1.ProtocolTCP}},
						ClusterIP: "10.98.208.26",
					},
				},
			),
			expectedErrorMsg: "unable to load root certificates",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := CreateTheiaManagerClient(tt.fakeClientset, "", true)
			if tt.expectedErrorMsg != "" {
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
			}
		})
	}
}
