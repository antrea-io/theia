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
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"antrea.io/theia/pkg/theia/portforwarder"
)

func CreateK8sClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func PolicyRecoPreCheck(clientset kubernetes.Interface) error {
	err := CheckSparkOperatorPod(clientset)
	if err != nil {
		return err
	}
	err = CheckClickHousePod(clientset)
	if err != nil {
		return err
	}
	return nil
}

func CheckSparkOperatorPod(clientset kubernetes.Interface) error {
	// Check the deployment of Spark Operator in flow-visibility ns
	pods, err := clientset.CoreV1().Pods(flowVisibilityNS).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=spark-operator",
	})
	if err != nil {
		return fmt.Errorf("error %v when finding the policy-recommendation-spark-operator Pod, please check the deployment of the Spark Operator", err)
	}
	if len(pods.Items) < 1 {
		return fmt.Errorf("can't find the policy-recommendation-spark-operator Pod, please check the deployment of the Spark Operator")
	}
	hasRunningPod := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			hasRunningPod = true
			break
		}
	}
	if !hasRunningPod {
		return fmt.Errorf("can't find a running ClickHouse Pod, please check the deployment of ClickHouse")
	}
	return nil
}

func CheckClickHousePod(clientset kubernetes.Interface) error {
	// Check the ClickHouse deployment in flow-visibility namespace
	pods, err := clientset.CoreV1().Pods(flowVisibilityNS).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=clickhouse",
	})
	if err != nil {
		return fmt.Errorf("error %v when finding the ClickHouse Pod, please check the deployment of the ClickHouse", err)
	}
	if len(pods.Items) < 1 {
		return fmt.Errorf("can't find the ClickHouse Pod, please check the deployment of ClickHouse")
	}
	hasRunningPod := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			hasRunningPod = true
			break
		}
	}
	if !hasRunningPod {
		return fmt.Errorf("can't find a running ClickHouse Pod, please check the deployment of ClickHouse")
	}
	return nil
}

func ConstStrToPointer(constStr string) *string {
	return &constStr
}

func GetServiceAddr(clientset kubernetes.Interface, serviceName string) (string, int, error) {
	var serviceIP string
	var servicePort int
	service, err := clientset.CoreV1().Services(flowVisibilityNS).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	serviceIP = service.Spec.ClusterIP
	for _, port := range service.Spec.Ports {
		if port.Name == "tcp" {
			servicePort = int(port.Port)
		}
	}
	if servicePort == 0 {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	return serviceIP, servicePort, nil
}

func StartPortForward(kubeconfig string, service string, servicePort int, listenAddress string, listenPort int) (*portforwarder.PortForwarder, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	// Forward the policy recommendation service port
	pf, err := portforwarder.NewServicePortForwarder(config, flowVisibilityNS, service, servicePort, listenAddress, listenPort)
	if err != nil {
		return nil, err
	}
	err = pf.Start()
	if err != nil {
		return nil, err
	}
	return pf, nil
}

func ResolveKubeConfig(cmd *cobra.Command) (string, error) {
	var err error
	kubeconfigPath, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return "", err
	}
	if len(kubeconfigPath) == 0 {
		var hasIt bool
		kubeconfigPath, hasIt = os.LookupEnv("KUBECONFIG")
		if !hasIt || len(strings.TrimSpace(kubeconfigPath)) == 0 {
			kubeconfigPath = clientcmd.RecommendedHomeFile
		}
	}
	return kubeconfigPath, nil
}

func getClickHouseSecret(clientset kubernetes.Interface) (username []byte, password []byte, err error) {
	secret, err := clientset.CoreV1().Secrets(flowVisibilityNS).Get(context.TODO(), "clickhouse-secret", metav1.GetOptions{})
	if err != nil {
		return username, password, fmt.Errorf("error %v when finding the ClickHouse secret, please check the deployment of ClickHouse", err)
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

func connectClickHouse(clientset kubernetes.Interface, url string) (*sql.DB, error) {
	var connect *sql.DB
	var connErr error
	connRetryInterval := 1 * time.Second
	connTimeout := 10 * time.Second

	// Connect to ClickHouse in a loop
	if err := wait.PollImmediate(connRetryInterval, connTimeout, func() (bool, error) {
		// Open the database and ping it
		var err error
		connect, err = sql.Open("clickhouse", url)
		if err != nil {
			connErr = fmt.Errorf("failed to open ClickHouse: %v", err)
			return false, nil
		}
		if err := connect.Ping(); err != nil {
			if exception, ok := err.(*clickhouse.Exception); ok {
				connErr = fmt.Errorf("failed to ping ClickHouse: %v", exception.Message)
			} else {
				connErr = fmt.Errorf("failed to ping ClickHouse: %v", err)
			}
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse after %s: %v", connTimeout, connErr)
	}
	return connect, nil
}

func setupClickHouseConnection(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool) (connect *sql.DB, portForward *portforwarder.PortForwarder, err error) {
	if endpoint == "" {
		service := "clickhouse-clickhouse"
		if useClusterIP {
			serviceIP, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return nil, nil, fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		} else {
			listenAddress := "localhost"
			listenPort := 9000
			_, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return nil, nil, fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
			}
			// Forward the ClickHouse service port
			portForward, err = StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
			if err != nil {
				return nil, nil, fmt.Errorf("error when forwarding port: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)
		}
	}

	// Connect to ClickHouse and execute query
	username, password, err := getClickHouseSecret(clientset)
	if err != nil {
		return nil, portForward, err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err = connectClickHouse(clientset, url)
	if err != nil {
		return nil, portForward, fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	return connect, portForward, nil
}

func TableOutput(table [][]string) {
	writer := tabwriter.NewWriter(os.Stdout, 15, 0, 1, ' ', 0)
	for _, line := range table {
		fmt.Fprintln(writer, strings.Join(line, "\t")+"\t")
	}
	writer.Flush()
}

func FormatTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "N/A"
	}
	return timestamp.UTC().Format("2006-01-02 15:04:05")
}

func ParseEndpoint(endpoint string) error {
	_, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return fmt.Errorf("input endpoint %s does not seem a valid URL, parsing error: %v", endpoint, err)
	}
	return nil
}

func ParseRecommendationID(recommendationID string) error {
	_, err := uuid.Parse(recommendationID)
	if err != nil {
		return fmt.Errorf("input id %s does not seem a valid UUID, parsing error:: %v", recommendationID, err)
	}
	return nil
}
