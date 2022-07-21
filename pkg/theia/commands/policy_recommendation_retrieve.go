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
	"database/sql"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
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
		err = ParseRecommendationID(recoID)
		if err != nil {
			return err
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
			err = ParseEndpoint(endpoint)
			if err != nil {
				return err
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
	connect, portForward, err := setupClickHouseConnection(clientset, kubeconfig, endpoint, useClusterIP)
	if portForward != nil {
		defer portForward.Stop()
	}
	if err != nil {
		return "", err
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
	policyRecommendationRetrieveCmd.Flags().StringP(
		"file",
		"f",
		"",
		"The file path where you want to save the result.",
	)
}
