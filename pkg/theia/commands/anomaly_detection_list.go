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

	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
)

// anomalyDetectionListCmd represents the anomaly-detection list command
var anomalyDetectionListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all anomaly detection jobs",
	Long:    `List all anomaly detection jobs with name, creation time, completion time and status.`,
	Aliases: []string{"ls"},
	Example: `
List all throughput-anomaly-detection jobs
$ theia throughput-anomaly-detection list
`,
	RunE: anomalyDetectionList,
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(anomalyDetectionListCmd)
}

func anomalyDetectionList(cmd *cobra.Command, args []string) error {
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
	tadList := &anomalydetector.ThroughputAnomalyDetectorList{}
	err = theiaClient.Get().
		AbsPath("/apis/anomalydetector.theia.antrea.io/v1alpha1/").
		Resource("throughputanomalydetectors").
		Do(context.TODO()).Into(tadList)
	if err != nil {
		return fmt.Errorf("error when getting anomaly detection job list: %v", err)
	}

	sparkApplicationTable := [][]string{
		{"CreationTime", "CompletionTime", "Name", "Status"},
	}
	for _, tad := range tadList.Items {
		if tad.Status.SparkApplication == "" {
			continue
		}
		sparkApplicationTable = append(sparkApplicationTable,
			[]string{
				FormatTimestamp(tad.Status.StartTime.Time),
				FormatTimestamp(tad.Status.EndTime.Time),
				tad.Name,
				tad.Status.State,
			})
	}
	TableOutput(sparkApplicationTable)
	return nil
}
