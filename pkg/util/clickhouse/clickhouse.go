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
	"fmt"
	"os"

	"github.com/ClickHouse/clickhouse-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"antrea.io/theia/pkg/util/env"
	"antrea.io/theia/pkg/util/k8s"
)

const (
	usernameKey         = "CLICKHOUSE_USERNAME"
	passwordKey         = "CLICKHOUSE_PASSWORD"
	urlKey              = "CLICKHOUSE_URL"
	ServiceName         = "clickhouse-clickhouse"
	ServicePortProtocal = "TCP"
	// #nosec G101: false positive triggered by variable name which includes "secret"
	SecretName = "clickhouse-secret"
)

var (
	openSql         = sql.Open
	createK8sClient = k8s.CreateK8sClient
)

func SetupConnection(client kubernetes.Interface) (connect *sql.DB, err error) {
	url, err := getClickHouseURL(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get ClickHouse URL: %v", err)
	}
	connect, err = Connect(url)
	if err != nil {
		return nil, fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	return connect, nil
}

func Connect(url string) (*sql.DB, error) {
	var connect *sql.DB
	// Open the database and ping it
	var err error
	connect, err = openSql("clickhouse", url)
	if err != nil {
		return connect, fmt.Errorf("failed to open ClickHouse: %v", err)
	}
	if err := connect.Ping(); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			return connect, fmt.Errorf("failed to ping ClickHouse: %v", exception.Message)
		} else {
			return connect, fmt.Errorf("failed to ping ClickHouse: %v", err)
		}
	}
	return connect, nil
}

func GetSecret(client kubernetes.Interface, namespace string) (username string, password string, err error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), SecretName, metav1.GetOptions{})
	if err != nil {
		return username, password, fmt.Errorf("error when finding the ClickHouse secret. Error: %v", err)
	}
	usernameByte, ok := secret.Data["username"]
	if !ok {
		return username, password, fmt.Errorf("error when getting the ClickHouse username")
	}
	passwordByte, ok := secret.Data["password"]
	if !ok {
		return username, password, fmt.Errorf("error when getting the ClickHouse password")
	}
	username = string(usernameByte)
	password = string(passwordByte)
	return username, password, nil
}

func getClickHouseURL(client kubernetes.Interface) (url string, err error) {
	baseURL := os.Getenv(urlKey)
	username := os.Getenv(usernameKey)
	password := os.Getenv(passwordKey)

	if baseURL == "" || username == "" || password == "" {
		if client == nil {
			client, err = createK8sClient()
			if err != nil {
				return url, fmt.Errorf("failed to create k8s client: %v", err)
			}
		}
		serviceIP, servicePort, err := k8s.GetServiceAddr(client, ServiceName, env.GetTheiaNamespace(), ServicePortProtocal)
		if err != nil {
			return url, fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
		}
		baseURL = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		username, password, err = GetSecret(client, env.GetTheiaNamespace())
		if err != nil {
			return url, err
		}
	}
	url = fmt.Sprintf("%s?debug=false&username=%s&password=%s", baseURL, username, password)
	return url, nil
}
