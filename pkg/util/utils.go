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

package util

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var SqlOpenFunc = sql.Open

func ParseRecommendationName(npName string) error {
	if !strings.HasPrefix(npName, "pr-") {
		return fmt.Errorf("input name %s is not a valid policy recommendation job name", npName)

	}
	id := npName[3:]
	_, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("input name %s does not contain a valid UUID, parsing error: %v", npName, err)
	}
	return nil
}

func GetServiceAddr(client kubernetes.Interface, serviceName, serviceNamespace string, protocol v1.Protocol) (string, int, error) {
	var serviceIP string
	var servicePort int
	service, err := client.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	serviceIP = service.Spec.ClusterIP
	for _, port := range service.Spec.Ports {
		if port.Protocol == protocol {
			servicePort = int(port.Port)
		}
	}
	if servicePort == 0 {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: no %s service port", serviceName, protocol)
	}
	return serviceIP, servicePort, nil
}

func GetClickHouseSecret(client kubernetes.Interface, namespace string) (username []byte, password []byte, err error) {
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), "clickhouse-secret", metav1.GetOptions{})
	if err != nil {
		return username, password, fmt.Errorf("error when finding the ClickHouse secret. Error: %v", err)
	}
	username, ok := secret.Data["username"]
	if !ok {
		return username, password, fmt.Errorf("error when getting the ClickHouse username")
	}
	password, ok = secret.Data["password"]
	if !ok {
		return username, password, fmt.Errorf("error when getting the ClickHouse password")
	}
	return username, password, nil
}

func ConnectClickHouse(url string) (*sql.DB, error) {
	var connect *sql.DB
	// Open the database and ping it
	var err error
	connect, err = SqlOpenFunc("clickhouse", url)
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

func SetupClickHouseConnection(client kubernetes.Interface, namespace string) (connect *sql.DB, err error) {
	serviceIP, servicePort, err := GetServiceAddr(client, "clickhouse-clickhouse", namespace, v1.ProtocolTCP)
	if err != nil {
		return connect, fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
	}
	endpoint := fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)

	// Connect to ClickHouse and execute query
	username, password, err := GetClickHouseSecret(client, namespace)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err = ConnectClickHouse(url)
	if err != nil {
		return nil, fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	return connect, nil
}
