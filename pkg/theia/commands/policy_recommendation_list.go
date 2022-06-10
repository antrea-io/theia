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
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

type policyRecommendationRow struct {
	timeComplete time.Time
	id           string
}

// policyRecommendationListCmd represents the policy-recommendation list command
var policyRecommendationListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all policy recommendation Spark jobs",
	Long:    `List all policy recommendation Spark jobs with name, creation time and status.`,
	Aliases: []string{"ls"},
	Example: `
List all policy recommendation Spark jobs
$ theia policy-recommendation list
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		kubeconfig, err := ResolveKubeConfig(cmd)
		if err != nil {
			return err
		}
		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
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

		err = PolicyRecoPreCheck(clientset)
		if err != nil {
			return err
		}

		sparkApplicationList := &sparkv1.SparkApplicationList{}
		err = clientset.CoreV1().RESTClient().Get().
			AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
			Namespace(flowVisibilityNS).
			Resource("sparkapplications").
			Do(context.TODO()).Into(sparkApplicationList)
		if err != nil {
			return err
		}

		completedPolicyRecommendationList, err := getCompletedPolicyRecommendationList(clientset, kubeconfig, endpoint, useClusterIP)

		if err != nil {
			return err
		}

		sparkApplicationTable := [][]string{
			{"CreateTime", "CompleteTime", "ID", "Status"},
		}
		idMap := make(map[string]bool)
		for _, sparkApplication := range sparkApplicationList.Items {
			id := sparkApplication.ObjectMeta.Name[3:]
			idMap[id] = true
			sparkApplicationTable = append(sparkApplicationTable,
				[]string{
					FormatTimestamp(sparkApplication.ObjectMeta.CreationTimestamp.Time),
					FormatTimestamp(sparkApplication.Status.TerminationTime.Time),
					id,
					strings.TrimSpace(string(sparkApplication.Status.AppState.State)),
				})
		}

		for _, completedPolicyRecommendation := range completedPolicyRecommendationList {
			if _, ok := idMap[completedPolicyRecommendation.id]; !ok {
				idMap[completedPolicyRecommendation.id] = true
				sparkApplicationTable = append(sparkApplicationTable,
					[]string{
						"N/A",
						FormatTimestamp(completedPolicyRecommendation.timeComplete),
						completedPolicyRecommendation.id,
						"COMPLETED",
					})
			}
		}

		TableOutput(sparkApplicationTable)
		return nil
	},
}

func getCompletedPolicyRecommendationList(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool) (completedPolicyRecommendationList []policyRecommendationRow, err error) {
	if endpoint == "" {
		service := "clickhouse-clickhouse"
		if useClusterIP {
			serviceIP, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return completedPolicyRecommendationList, fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		} else {
			listenAddress := "localhost"
			listenPort := 9000
			_, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return completedPolicyRecommendationList, fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
			}
			// Forward the ClickHouse service port
			pf, err := StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
			if err != nil {
				return completedPolicyRecommendationList, fmt.Errorf("error when forwarding port: %v", err)
			}
			defer pf.Stop()
			endpoint = fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)
		}
	}

	// Connect to ClickHouse and get the result
	username, password, err := getClickHouseSecret(clientset)
	if err != nil {
		return completedPolicyRecommendationList, err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err := connectClickHouse(clientset, url)
	if err != nil {
		return completedPolicyRecommendationList, fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	query := "SELECT timeCreated, id FROM recommendations;"
	rows, err := connect.Query(query)
	if err != nil {
		return completedPolicyRecommendationList, fmt.Errorf("failed to get recommendation jobs: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var row policyRecommendationRow
		err := rows.Scan(&row.timeComplete, &row.id)
		if err != nil {
			return completedPolicyRecommendationList, fmt.Errorf("err when scaning recommendations row %v", err)
		}
		completedPolicyRecommendationList = append(completedPolicyRecommendationList, row)
	}
	return completedPolicyRecommendationList, nil
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationListCmd)
	policyRecommendationListCmd.Flags().String(
		"clickhouse-endpoint",
		"",
		"The ClickHouse service endpoint.",
	)
	policyRecommendationListCmd.Flags().Bool(
		"use-cluster-ip",
		false,
		`Enable this option will use Service ClusterIP instead of port forwarding when connecting to the ClickHouse service.
It can only be used when running theia in cluster.`,
	)
}
