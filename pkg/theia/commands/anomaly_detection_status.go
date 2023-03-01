// Copyright 2023 Antrea Authors
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

// anomalyDetectionStatusCmd represents the throughput-anomaly-detection status command
var anomalyDetectionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of a anomaly detection job",
	Long: `Check the current status of a anomaly detection job by name.
It will return the status of this anomaly detection job like SUBMITTED, RUNNING, COMPLETED, or FAILED.`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Check the current status of job with name tad-e998433e-accb-4888-9fc8-06563f073e86
$ theia throughput-anomaly-detection status --name tad-e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia throughput-anomaly-detection status tad-e998433e-accb-4888-9fc8-06563f073e86
Use Service ClusterIP when checking the current status of job with name tad-e998433e-accb-4888-9fc8-06563f073e86
$ theia throughput-anomaly-detection status tad-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
`,
	RunE: anomalyDetectionStatus,
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(anomalyDetectionStatusCmd)
	anomalyDetectionStatusCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the anomaly detection job.",
	)
}

func anomalyDetectionStatus(cmd *cobra.Command, args []string) error {
	tadName, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	if tadName == "" && len(args) == 1 {
		tadName = args[0]
	}
	err = util.ParseADAlgorithmID(tadName)
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
	tad, err := GetThroughputAnomalyDetectorByID(theiaClient, tadName)
	if err != nil {
		return fmt.Errorf("error when getting anomaly detection job by using job name: %v", err)
	}
	state := tad.Status.State
	if state == "RUNNING" {
		completedStages := tad.Status.CompletedStages
		totalStages := tad.Status.TotalStages
		var stateProgress string
		if totalStages == 0 {
			stateProgress = ": 0/0 (0%) stages completed"
		} else {
			stateProgress = fmt.Sprintf(": %d/%d (%d%%) stages completed", completedStages, totalStages, completedStages*100/totalStages)
		}
		state += stateProgress
	}
	errorMessage := tad.Status.ErrorMsg
	fmt.Printf("Status of this anomaly detection job is %s\n", state)
	if errorMessage != "" {
		fmt.Printf("Error message: %s\n", errorMessage)
	}
	return nil
}
