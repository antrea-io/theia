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
	"net"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"antrea.io/theia/pkg/apis"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/apiserver/certificate"
	"antrea.io/theia/pkg/theia/commands/config"
	"antrea.io/theia/pkg/theia/portforwarder"
)

var (
	SetupTheiaClientAndConnection = setupTheiaClientAndConnection
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

func setupTheiaClientAndConnection(cmd *cobra.Command, useClusterIP bool) (restclient.Interface, *portforwarder.PortForwarder, error) {
	kubeconfig, err := ResolveKubeConfig(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't resolve kubeconfig: %v", err)
	}
	clientset, err := CreateK8sClient(kubeconfig)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
	}
	theiaClient, portForward, err := CreateTheiaManagerClient(clientset, kubeconfig, useClusterIP)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create Theia manager client: %v", err)
	}
	return theiaClient.CoreV1().RESTClient(), portForward, err
}

func CreateTheiaManagerClient(k8sClient kubernetes.Interface, kubeconfig string, useClusterIP bool) (kubernetes.Interface, *portforwarder.PortForwarder, error) {
	// check and get ca-cert.pem file
	caCrt, err := GetCaCrt(k8sClient)
	if err != nil {
		return nil, nil, fmt.Errorf("error when getting ca-crt: %v", err)
	}
	// check and get token
	token, err := GetToken(k8sClient)
	if err != nil {
		return nil, nil, fmt.Errorf("error when getting token: %v", err)
	}
	var host string
	var portForward *portforwarder.PortForwarder
	if useClusterIP {
		serviceIP, servicePort, err := GetServiceAddr(k8sClient, config.TheiaManagerServiceName)
		if err != nil {
			return nil, nil, fmt.Errorf("error when getting the Theia Manager Service address: %v", err)
		}
		host = net.JoinHostPort(serviceIP, fmt.Sprint(servicePort))
	} else {
		listenAddress := "localhost"
		listenPort := apis.TheiaManagerAPIPort
		_, servicePort, err := GetServiceAddr(k8sClient, config.TheiaManagerServiceName)
		if err != nil {
			return nil, nil, fmt.Errorf("error when getting the Theia Manager Service port: %v", err)
		}
		// Forward the Theia Manager service port
		portForward, err = StartPortForward(kubeconfig, config.TheiaManagerServiceName, servicePort, listenAddress, listenPort)
		if err != nil {
			return nil, nil, fmt.Errorf("error when forwarding port: %v", err)
		}
		host = net.JoinHostPort(listenAddress, fmt.Sprint(listenPort))
	}

	clientConfig := &restclient.Config{
		Host:        host,
		BearerToken: token,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure:   false,
			ServerName: certificate.GetTheiaServerNames(certificate.TheiaServiceName)[0],
			CAData:     []byte(caCrt),
		},
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("error when creating Theia manager client: %v", err)
	}
	return clientset, portForward, nil
}

func GetCaCrt(clientset kubernetes.Interface) (string, error) {
	caConfigMap, err := clientset.CoreV1().ConfigMaps(config.FlowVisibilityNS).Get(context.TODO(), config.CAConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error when getting ConfigMap theia-ca: %v", err)
	}
	caCrt, ok := caConfigMap.Data[config.CAConfigMapKey]
	if !ok {
		return "", fmt.Errorf("error when checking ca.crt in data: %v", err)
	}
	return caCrt, nil
}

func GetToken(clientset kubernetes.Interface) (string, error) {
	secret, err := clientset.CoreV1().Secrets(config.FlowVisibilityNS).Get(context.TODO(), config.TheiaCliAccountName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error when getting secret %s: %v", config.TheiaCliAccountName, err)
	}
	token := string(secret.Data[config.ServiceAccountTokenKey])
	if len(token) == 0 {
		return "", fmt.Errorf("secret '%s' does not include token", config.TheiaCliAccountName)
	}
	return token, nil
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
	pods, err := clientset.CoreV1().Pods(config.FlowVisibilityNS).List(context.TODO(), metav1.ListOptions{
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
		return fmt.Errorf("can't find a running Spark Operator Pod, please check the deployment of Spark")
	}
	return nil
}

func CheckClickHousePod(clientset kubernetes.Interface) error {
	// Check the ClickHouse deployment in flow-visibility namespace
	pods, err := clientset.CoreV1().Pods(config.FlowVisibilityNS).List(context.TODO(), metav1.ListOptions{
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

func GetServiceAddr(clientset kubernetes.Interface, serviceName string) (string, int, error) {
	var serviceIP string
	var servicePort int
	service, err := clientset.CoreV1().Services(config.FlowVisibilityNS).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	serviceIP = service.Spec.ClusterIP
	for _, port := range service.Spec.Ports {
		if port.Name == "tcp" || port.Protocol == "TCP" {
			servicePort = int(port.Port)
		}
	}
	if servicePort == 0 {
		return serviceIP, servicePort, fmt.Errorf("error when finding the Service %s: %v", serviceName, err)
	}
	return serviceIP, servicePort, nil
}

func StartPortForward(kubeconfig string, service string, servicePort int, listenAddress string, listenPort int) (*portforwarder.PortForwarder, error) {
	configuration, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	// Forward the service port
	pf, err := portforwarder.NewServicePortForwarder(configuration, config.FlowVisibilityNS, service, servicePort, listenAddress, listenPort)
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
	secret, err := clientset.CoreV1().Secrets(config.FlowVisibilityNS).Get(context.TODO(), "clickhouse-secret", metav1.GetOptions{})
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

func SetupClickHouseConnection(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool) (connect *sql.DB, portForward *portforwarder.PortForwarder, err error) {
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

func TableOutputVertical(table [][]string) {
	header := table[0]
	writer := tabwriter.NewWriter(os.Stdout, 15, 0, 1, ' ', 0)
	for i := 1; i < len(table); i++ {
		fmt.Fprintln(writer, fmt.Sprintf("Row %d:\t", i))
		fmt.Fprintln(writer, fmt.Sprint("-------"))
		for j, val := range table[i] {
			fmt.Fprintln(writer, fmt.Sprintf("%s:\t%s", header[j], val))
		}
		fmt.Fprintln(writer)
		writer.Flush()
	}
}

func FormatTimestamp(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "N/A"
	}
	return timestamp.UTC().Format("2006-01-02 15:04:05")
}

func getPolicyRecommendationByName(theiaClient restclient.Interface, name string) (npr intelligence.NetworkPolicyRecommendation, err error) {
	err = theiaClient.Get().
		AbsPath("/apis/intelligence.theia.antrea.io/v1alpha1/").
		Resource("networkpolicyrecommendations").
		Name(name).
		Do(context.TODO()).
		Into(&npr)
	if err != nil {
		return npr, fmt.Errorf("failed to get policy recommendation job %s: %v", name, err)
	}
	return npr, nil
}
