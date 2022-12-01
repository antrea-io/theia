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

	"antrea.io/theia/pkg/util"
)

// policyRecommendationDeleteCmd represents the policy-recommendation delete command
var policyRecommendationDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a policy recommendation job",
	Long:    `Delete a policy recommendation job by Name.`,
	Aliases: []string{"del"},
	Args:    cobra.RangeArgs(0, 1),
	Example: `
Delete the network policy recommendation job with Name pr-e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation delete pr-e998433e-accb-4888-9fc8-06563f073e86
`,
	RunE: policyRecommendationDelete,
}

func policyRecommendationDelete(cmd *cobra.Command, args []string) error {
	prName, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	if prName == "" && len(args) == 1 {
		prName = args[0]
	}
	err = util.ParseRecommendationName(prName)
	if err != nil {
		return err
	}
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
	err = theiaClient.Delete().
		AbsPath("/apis/intelligence.theia.antrea.io/v1alpha1/").
		Resource("networkpolicyrecommendations").
		Name(prName).
		Do(context.TODO()).
		Error()
	if err != nil {
		return fmt.Errorf("error when deleting policy recommendation job: %v", err)
	}
	fmt.Printf("Successfully deleted policy recommendation job with name: %s\n", prName)
	return nil
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationDeleteCmd)
	policyRecommendationDeleteCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the policy recommendation job.",
	)
}
