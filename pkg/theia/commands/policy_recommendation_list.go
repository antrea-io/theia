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

	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
)

// policyRecommendationListCmd represents the policy-recommendation list command
var policyRecommendationListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all policy recommendation jobs",
	Long:    `List all policy recommendation jobs with name, creation time, completion time and status.`,
	Aliases: []string{"ls"},
	Example: `
List all policy recommendation jobs
$ theia policy-recommendation list
`,
	RunE: policyRecommendationList,
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationListCmd)
}

func policyRecommendationList(cmd *cobra.Command, args []string) error {
	useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
	if err != nil {
		return err
	}
	theiaClient, pf, err := SetupTheiaClientAndConnection(cmd, useClusterIP)
	if err != nil {
		return fmt.Errorf("couldn't setup Theia manager client, %v", err)
	}
	if pf != nil {
		defer pf.Stop()
	}
	nprList := &intelligence.NetworkPolicyRecommendationList{}
	err = theiaClient.Get().
		AbsPath("/apis/intelligence.theia.antrea.io/v1alpha1/").
		Resource("networkpolicyrecommendations").
		Do(context.TODO()).Into(nprList)
	if err != nil {
		return fmt.Errorf("error when getting policy recommendation job list: %v", err)
	}

	sparkApplicationTable := [][]string{
		{"CreationTime", "CompletionTime", "Name", "Status"},
	}
	for _, npr := range nprList.Items {
		if npr.Status.SparkApplication == "" {
			continue
		}
		sparkApplicationTable = append(sparkApplicationTable,
			[]string{
				FormatTimestamp(npr.Status.StartTime.Time),
				FormatTimestamp(npr.Status.EndTime.Time),
				npr.Name,
				npr.Status.State,
			})
	}
	TableOutput(sparkApplicationTable)
	return nil
}
