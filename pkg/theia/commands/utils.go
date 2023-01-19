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
	"fmt"
	"net"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"antrea.io/theia/pkg/apis"
	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	stats "antrea.io/theia/pkg/apis/stats/v1alpha1"
	"antrea.io/theia/pkg/apiserver/certificate"
	"antrea.io/theia/pkg/theia/commands/config"
	"antrea.io/theia/pkg/theia/portforwarder"
	"antrea.io/theia/pkg/util/k8s"
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
	serviceIP, servicePort, err := k8s.GetServiceAddr(k8sClient, config.TheiaManagerServiceName, config.FlowVisibilityNS, v1.ProtocolTCP)
	if err != nil {
		return nil, nil, fmt.Errorf("error when getting the Theia Manager Service address: %v", err)
	}
	if useClusterIP {
		host = net.JoinHostPort(serviceIP, fmt.Sprint(servicePort))
	} else {
		listenAddress := "localhost"
		listenPort := apis.TheiaManagerAPIPort
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
		fmt.Fprintf(writer, "Row %d:\t\n", i)
		fmt.Fprintf(writer, "-------\n")
		for j, val := range table[i] {
			fmt.Fprintf(writer, "%s:\t%s\n", header[j], val)
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

func getClickHouseStatusByCategory(theiaClient restclient.Interface, name string) (status stats.ClickHouseStats, err error) {
	err = theiaClient.Get().
		AbsPath("/apis/stats.theia.antrea.io/v1alpha1/").
		Resource("clickhouse").
		Name(name).
		Do(context.TODO()).
		Into(&status)
	if err != nil {
		return status, fmt.Errorf("failed to get clickhouse %s status: %v", name, err)
	}
	return status, nil
}

func GetThroughputAnomalyDetectorByID(theiaClient restclient.Interface, name string) (tad anomalydetector.ThroughputAnomalyDetector, err error) {
	err = theiaClient.Get().
		AbsPath("/apis/anomalydetector.theia.antrea.io/v1alpha1/").
		Resource("throughputanomalydetectors").
		Name(name).
		Do(context.TODO()).
		Into(&tad)
	if err != nil {
		return tad, fmt.Errorf("failed to get Throughput Anomaly Detector job %s: %v", name, err)
	}
	return tad, nil
}
