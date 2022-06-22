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

		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
		}

		idMap, err := getPolicyRecommendationIdMap(clientset, kubeconfig, endpoint, useClusterIP)
		if err != nil {
			return fmt.Errorf("err when getting policy recommendation ID map, %v", err)
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
	connect, portForward, err := setupClickHouseConnection(clientset, kubeconfig, endpoint, useClusterIP)
	if portForward != nil {
		defer portForward.Stop()
	}
	if err != nil {
		return err
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
}
