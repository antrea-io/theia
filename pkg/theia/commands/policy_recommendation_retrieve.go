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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/util"
)

// policyRecommendationRetrieveCmd represents the policy-recommendation retrieve command
var policyRecommendationRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Short: "Get the recommendation result of a policy recommendation job",
	Long: `Get the recommendation result of a policy recommendation job by name.
It will return the recommended NetworkPolicies described in yaml.`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Get the recommendation result with job name pr-e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation retrieve --name pr-e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia policy-recommendation retrieve pr-e998433e-accb-4888-9fc8-06563f073e86
Use Service ClusterIP when getting the result
$ theia policy-recommendation retrieve pr-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
Save the recommendation result to file
$ theia policy-recommendation retrieve pr-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip --file output.yaml
`,
	RunE: policyRecommendationRetrieve,
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationRetrieveCmd)
	policyRecommendationRetrieveCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the policy recommendation job.",
	)
	policyRecommendationRetrieveCmd.Flags().StringP(
		"file",
		"f",
		"",
		"The file path where you want to save the result.",
	)
}

func policyRecommendationRetrieve(cmd *cobra.Command, args []string) error {
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
	filePath, err := cmd.Flags().GetString("file")
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
	npr, err := getPolicyRecommendationByName(theiaClient, prName)
	if err != nil {
		return fmt.Errorf("error when getting policy recommendation job by job name: %v", err)
	}
	if filePath != "" {
		if err := os.WriteFile(filePath, []byte(npr.Status.RecommendationOutcome), 0600); err != nil {
			return fmt.Errorf("error when writing recommendation result to file: %v", err)
		}
		return nil
	}
	if npr.Status.RecommendationOutcome != "" {
		fmt.Print(npr.Status.RecommendationOutcome)
	}
	return nil
}
