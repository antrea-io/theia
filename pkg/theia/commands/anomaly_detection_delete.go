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
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/util"
)

// anomalyDetectionDeleteCmd represents the anomaly detection delete command
var anomalyDetectionDeleteCmd = &cobra.Command{
	Use:     "delete",
	Short:   "Delete a anomaly detection job",
	Long:    `Delete a anomaly detection job by Name.`,
	Aliases: []string{"del"},
	Args:    cobra.RangeArgs(0, 1),
	Example: `
Delete the anomaly detection job with Name tad-e998433e-accb-4888-9fc8-06563f073e86
$ theia throughput-anomaly-detection delete tad-e998433e-accb-4888-9fc8-06563f073e86
`,
	RunE: anomalyDetectionDelete,
}

func anomalyDetectionDelete(cmd *cobra.Command, args []string) error {
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
	err = theiaClient.Delete().
		AbsPath("/apis/anomalydetector.theia.antrea.io/v1alpha1/").
		Resource("throughputanomalydetectors").
		Name(tadName).
		Do(context.TODO()).
		Error()
	if err != nil {
		return fmt.Errorf("error when deleting anomaly detection job: %v", err)
	}
	fmt.Printf("Successfully deleted anomaly detection job with name: %s\n", tadName)
	return nil
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(anomalyDetectionDeleteCmd)
	anomalyDetectionDeleteCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the anomaly detection job.",
	)
}
