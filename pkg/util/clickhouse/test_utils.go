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

package clickhouse

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func CreateFakeClickHouse(t *testing.T, kubeClient kubernetes.Interface, namespace string) (db *sql.DB, mock sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	openSql = func(driverName, dataSourceName string) (*sql.DB, error) {
		assert.Equal(t, driverName, "clickhouse")
		assert.Equal(t, dataSourceName, "tcp://localhost:9000?debug=false&username=username&password=password")
		return db, nil
	}
	clickHousePod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clickhouse",
			Namespace: namespace,
			Labels:    map[string]string{"app": "clickhouse"},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}
	kubeClient.CoreV1().Pods(namespace).Create(context.TODO(), clickHousePod, metav1.CreateOptions{})
	clickHouseService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Ports:     []v1.ServicePort{{Port: 9000, Protocol: ServicePortProtocal}},
			ClusterIP: "localhost",
		},
	}
	kubeClient.CoreV1().Services(namespace).Create(context.TODO(), clickHouseService, metav1.CreateOptions{})
	clickhouseSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"username": []byte("username"),
			"password": []byte("password"),
		},
	}
	kubeClient.CoreV1().Secrets(namespace).Create(context.TODO(), clickhouseSecret, metav1.CreateOptions{})
	mock.ExpectPing()
	return db, mock
}
