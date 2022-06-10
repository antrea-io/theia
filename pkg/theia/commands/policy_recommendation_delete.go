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

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

// policyRecommendationDeleteCmd represents the policy-recommendation delete command
var policyRecommendationDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a policy recommendation Spark job",
	Long:    `Delete a policy recommendation Spark job by ID.`,
	Aliases: []string{"del"},
	Args:    cobra.RangeArgs(0, 1),
	Example: `
Delete the policy recommendation job with ID e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation delete e998433e-accb-4888-9fc8-06563f073e86
`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
		}

		idMap, err := getPolicyRecommendationIdMap(clientset, kubeconfig, endpoint, useClusterIP)
		if err != nil {
			return fmt.Errorf("err when get policy recommendation ID map, %v", err)
		}

		if _, ok := idMap[recoID]; !ok {
			return fmt.Errorf("could not find the policy recommendation job with given ID")
		}

		clientset.CoreV1().RESTClient().Delete().
			AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
			Namespace(flowVisibilityNS).
			Resource("sparkapplications").
			Name("pr-" + recoID).
			Do(context.TODO())

		err = deletePolicyRecommendationResult(clientset, kubeconfig, endpoint, useClusterIP, recoID)
		if err != nil {
			return err
		}

		fmt.Printf("Successfully deleted policy recommendation job with ID %s\n", recoID)
		return nil
	},
}

func getPolicyRecommendationIdMap(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool) (idMap map[string]bool, err error) {
	idMap = make(map[string]bool)
	sparkApplicationList := &sparkv1.SparkApplicationList{}
	err = clientset.CoreV1().RESTClient().Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(flowVisibilityNS).
		Resource("sparkapplications").
		Do(context.TODO()).Into(sparkApplicationList)
	if err != nil {
		return idMap, err
	}
	for _, sparkApplication := range sparkApplicationList.Items {
		id := sparkApplication.ObjectMeta.Name[3:]
		idMap[id] = true
	}
	completedPolicyRecommendationList, err := getCompletedPolicyRecommendationList(clientset, kubeconfig, endpoint, useClusterIP)
	if err != nil {
		return idMap, err
	}
	for _, completedPolicyRecommendation := range completedPolicyRecommendationList {
		idMap[completedPolicyRecommendation.id] = true
	}
	return idMap, nil
}

func deletePolicyRecommendationResult(clientset kubernetes.Interface, kubeconfig string, endpoint string, useClusterIP bool, recoID string) (err error) {
	if endpoint == "" {
		service := "clickhouse-clickhouse"
		if useClusterIP {
			serviceIP, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return fmt.Errorf("error when getting the ClickHouse Service address: %v", err)
			}
			endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
		} else {
			listenAddress := "localhost"
			listenPort := 9000
			_, servicePort, err := GetServiceAddr(clientset, service)
			if err != nil {
				return fmt.Errorf("error when getting the ClickHouse Service port: %v", err)
			}
			// Forward the ClickHouse service port
			pf, err := StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
			if err != nil {
				return fmt.Errorf("error when forwarding port: %v", err)
			}
			defer pf.Stop()
			endpoint = fmt.Sprintf("tcp://%s:%d", listenAddress, listenPort)
		}
	}

	// Connect to ClickHouse and get the result
	username, password, err := getClickHouseSecret(clientset)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s?debug=false&username=%s&password=%s", endpoint, username, password)
	connect, err := connectClickHouse(clientset, url)
	if err != nil {
		return fmt.Errorf("error when connecting to ClickHouse, %v", err)
	}
	query := "ALTER TABLE recommendations DELETE WHERE id = (?);"
	_, err = connect.Exec(query, recoID)
	if err != nil {
		return fmt.Errorf("failed to delete recommendation result with id %s: %v", recoID, err)
	}
	return nil
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationDeleteCmd)
	policyRecommendationDeleteCmd.Flags().StringP(
		"id",
		"i",
		"",
		"ID of the policy recommendation Spark job.",
	)
	policyRecommendationDeleteCmd.Flags().String(
		"clickhouse-endpoint",
		"",
		"The ClickHouse service endpoint.",
	)
	policyRecommendationDeleteCmd.Flags().Bool(
		"use-cluster-ip",
		false,
		`Enable this option will use Service ClusterIP instead of port forwarding when connecting to the ClickHouse service.
It can only be used when running theia in cluster.`,
	)
}
