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
)

func TestGetClickHouseSecret(t *testing.T) {
	var fakeClientset = fake.NewSimpleClientset(
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "clickhouse-secret",
				Namespace: flowVisibilityNS,
			},
			Data: map[string][]byte{
				"password": []byte("clickhouse_operator_password"),
				"username": []byte("clickhouse_operator"),
			},
		},
	)
	username, password, err := getClickHouseSecret(fakeClientset)
	assert.NoError(t, err)
	assert.Equal(t, "clickhouse_operator", string(username))
	assert.Equal(t, "clickhouse_operator_password", string(password))
}

func TestGetResultFromClickHouse(t *testing.T) {
	recoID := "db2134ea-7169-46f8-b56d-d643d4751d1d"
	expectedResult := "recommend-allow-acnp-kube-system-rpeal"
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	assert.NoError(t, err)
	defer db.Close()
	resultRow := sqlmock.NewRows([]string{"yamls"}).AddRow(expectedResult)
	mock.ExpectQuery("SELECT yamls FROM recommendations WHERE id = (?);").WithArgs(recoID).WillReturnRows(resultRow)
	result, err := getResultFromClickHouse(db, recoID)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}
