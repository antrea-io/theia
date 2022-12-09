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

	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/util"
)

// policyRecommendationStatusCmd represents the policy-recommendation status command
var policyRecommendationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of a policy recommendation job",
	Long: `Check the current status of a policy recommendation job by name.
It will return the status of this policy recommendation job like SUBMITTED, RUNNING, COMPLETED, or FAILED.`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Check the current status of job with name pr-e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation status --name pr-e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia policy-recommendation status pr-e998433e-accb-4888-9fc8-06563f073e86
Use Service ClusterIP when checking the current status of job with name pr-e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation status pr-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
`,
	RunE: policyRecommendationStatus,
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationStatusCmd)
	policyRecommendationStatusCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the policy recommendation job.",
	)
}

func policyRecommendationStatus(cmd *cobra.Command, args []string) error {
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
	npr, err := getPolicyRecommendationByName(theiaClient, prName)
	if err != nil {
		return fmt.Errorf("error when getting policy recommendation job by using job name: %v", err)
	}
	state := npr.Status.State
	if state == "RUNNING" {
		completedStages := npr.Status.CompletedStages
		totalStages := npr.Status.TotalStages
		var stateProgress string
		if totalStages == 0 {
			stateProgress = ": 0/0 (0%) stages completed"
		} else {
			stateProgress = fmt.Sprintf(": %d/%d (%d%%) stages completed", completedStages, totalStages, completedStages*100/totalStages)
		}
		state += stateProgress
	}
	errorMessage := npr.Status.ErrorMsg
	fmt.Printf("Status of this policy recommendation job is %s\n", state)
	if errorMessage != "" {
		fmt.Printf("Error message: %s\n", errorMessage)
	}
	return nil
}
