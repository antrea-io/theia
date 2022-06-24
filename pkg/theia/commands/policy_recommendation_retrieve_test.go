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

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"antrea.io/theia/pkg/theia/commands/config"
)

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

func TestGetResultFromClickHouse(t *testing.T) {
	testCases := []struct {
		name             string
		recommendationID string
		expectedResult   string
		expectedErrorMsg string
	}{
		{
			name:             "valid case",
			recommendationID: "db2134ea-7169-46f8-b56d-d643d4751d1d",
			expectedResult:   "recommend-allow-acnp-kube-system-rpeal",
			expectedErrorMsg: "",
		},
		{
			name:             "no result given recommendation ID",
			recommendationID: "db2134ea-7169",
			expectedResult:   "",
			expectedErrorMsg: "failed to get recommendation result with id db2134ea-7169: sql: no rows in result set",
		},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			assert.NoError(t, err)
			defer db.Close()
			resultRow := &sqlmock.Rows{}
			if tt.expectedResult != "" {
				resultRow = sqlmock.NewRows([]string{"yamls"}).AddRow(tt.expectedResult)
			}
			mock.ExpectQuery("SELECT yamls FROM recommendations WHERE id = (?);").WithArgs(tt.recommendationID).WillReturnRows(resultRow)
			result, err := getResultFromClickHouse(db, tt.recommendationID)
			if tt.expectedErrorMsg != "" {
				assert.EqualErrorf(t, err, tt.expectedErrorMsg, "Error should be: %v, got: %v", tt.expectedErrorMsg, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
