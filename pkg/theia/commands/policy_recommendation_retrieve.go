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
	"io/ioutil"
	"net/url"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// policyRecommendationRetrieveCmd represents the policy-recommendation retrieve command
var policyRecommendationRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Short: "Get the recommendation result of a policy recommendation Spark job",
	Long: `Get the recommendation result of a policy recommendation Spark job by ID.
It will return the recommended NetworkPolicies described in yaml.`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Get the recommendation result with job ID e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation retrieve --id e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86
Use a customized ClickHouse endpoint when connecting to ClickHouse to getting the result
$ theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86 --clickhouse-endpoint 10.10.1.1
Use Service ClusterIP when connecting to ClickHouse to getting the result
$ theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
Save the recommendation result to file
$ theia policy-recommendation retrieve e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip --file output.yaml
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse the flags
		recoID, err := cmd.Flags().GetString("id")
		if err != nil {
			return err
		}
		if recoID == "" && len(args) == 1 {
			recoID = args[0]
		}
		_, err = uuid.Parse(recoID)
		if err != nil {
			return fmt.Errorf("failed to decode input id %s into a UUID, err: %v", recoID, err)
		}
		kubeconfig, err := ResolveKubeConfig(cmd)
		if err != nil {
			return err
		}
		endpoint, err := cmd.Flags().GetString("clickhouse-endpoint")
		if err != nil {
			return err
		}
		if endpoint != "" {
			_, err := url.ParseRequestURI(endpoint)
			if err != nil {
				return fmt.Errorf("failed to decode input endpoint %s into a url, err: %v", endpoint, err)
			}
		}
		useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
		if err != nil {
			return err
		}
		filePath, err := cmd.Flags().GetString("file")
		if err != nil {
			return err
		}

		// Verify Clickhouse is running
		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig: %v", err)
		}
		if err := CheckClickHousePod(clientset); err != nil {
			return err
		}

		recoResult, err := getPolicyRecommendationResult(clientset, kubeconfig, endpoint, useClusterIP, filePath, recoID)
		if err != nil {
			return err
		} else {
			if recoResult != "" {
				fmt.Print(recoResult)
			}
		}
		return nil
	},
}

func getPolicyRecommendationResult(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool, filePath string, recoID string) (recoResult string, err error) {
	if endpoint == "" {
		service := "clickhouse-clickhouse"
		if useClusterIP {
			serviceIP, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return "", fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		} else {
			listenAddress := "localhost"
			listenPort := 9000
			_, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return "", fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
			}
			// Forward the ClickHouse service port
			pf, err := StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
			if err != nil {
				return "", fmt.Errorf("error when forwarding port: %v", err)
			}
			defer pf.Stop()
			endpoint = fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)
		}
	}

	// Connect to ClickHouse and get the result
	username, password, err := getClickHouseSecret(clientset)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err := connectClickHouse(clientset, url)
	if err != nil {
		return "", fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	recoResult, err = getResultFromClickHouse(connect, recoID)
	if err != nil {
		return "", fmt.Errorf("error when getting result from ClickHouse, %v", err)
	}
	if filePath != "" {
		if err := ioutil.WriteFile(filePath, []byte(recoResult), 0600); err != nil {
			return "", fmt.Errorf("error when writing recommendation result to file: %v", err)
		}
	} else {
		return recoResult, nil
	}
	return "", nil
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
			connErr = fmt.Errorf("failed to ping ClickHouse: %v", err)
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

func getResultFromClickHouse(connect *sql.DB, id string) (string, error) {
	var recoResult string
	query := "SELECT yamls FROM recommendations WHERE id = (?);"
	err := connect.QueryRow(query, id).Scan(&recoResult)
	if err != nil {
		return recoResult, fmt.Errorf("failed to get recommendation result with id %s: %v", id, err)
	}
	return recoResult, nil
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationRetrieveCmd)
	policyRecommendationRetrieveCmd.Flags().StringP(
		"id",
		"i",
		"",
		"ID of the policy recommendation Spark job.",
	)
	policyRecommendationRetrieveCmd.Flags().String(
		"clickhouse-endpoint",
		"",
		"The ClickHouse service endpoint.",
	)
	policyRecommendationRetrieveCmd.Flags().Bool(
		"use-cluster-ip",
		false,
		`Enable this option will use Service ClusterIP instead of port forwarding when connecting to the ClickHouse service.
It can only be used when running theia in cluster.`,
	)
	policyRecommendationRetrieveCmd.Flags().StringP(
		"file",
		"f",
		"",
		"The file path where you want to save the result.",
	)
}
