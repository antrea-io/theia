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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/util"
)

// throughputAnomalyDetectionRetrieveCmd represents the throughput-anomaly-detection retrieve command
var throughputAnomalyDetectionRetrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Short: "Get the result of an anomaly detection job",
	Long: `Get the result of an anomaly detection job by name.
It will return the anomalies detected in the network flow`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Get the anomaly detection result with job name tad-e998433e-accb-4888-9fc8-06563f073e86
$ theia throughput-anomaly-detection retrieve --name tad-e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia throughput-anomaly-detection retrieve tad-e998433e-accb-4888-9fc8-06563f073e86
Use Service ClusterIP when getting the result
$ theia throughput-anomaly-detection retrieve tad-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
Save the anomaly detection result to file
$ theia throughput-anomaly-detection retrieve tad-e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip --file output.yaml
`,
	RunE: throughputAnomalyDetectionRetrieve,
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(throughputAnomalyDetectionRetrieveCmd)
	throughputAnomalyDetectionRetrieveCmd.Flags().StringP(
		"name",
		"",
		"",
		"Name of the anomaly detection job.",
	)
	throughputAnomalyDetectionRetrieveCmd.Flags().StringP(
		"file",
		"f",
		"",
		"The file path where you want to save the result.",
	)
}

func throughputAnomalyDetectionRetrieve(cmd *cobra.Command, args []string) error {
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
	tad, err := GetThroughputAnomalyDetectorByID(theiaClient, tadName)
	if err != nil {
		return fmt.Errorf("error when getting anomaly detection job by job name: %v", err)
	}
	for _, stat := range tad.Stats {
		if stat.Anomaly == "NO ANOMALY DETECTED" {
			fmt.Printf("No Anomaly found in id: %v\n", stat.Id)
			return nil
		}
	}
	data, _ := json.MarshalIndent(tad.Stats, "", " ")
	if filePath != "" {
		if err := os.WriteFile(filePath, data, 0600); err != nil {
			return fmt.Errorf("error when writing anomaly detection result to file: %v", err)
		}
		return nil
	} else {
		var result [][]string
		switch tad.Stats[0].AggType {
		case "e2e":
			result = append(result, []string{"id", "sourceIP", "sourceTransportPort", "destinationIP", "destinationTransportPort", "flowStartSeconds", "flowEndSeconds", "throughput", "aggType", "algoType", "algoCalc", "anomaly"})
			for _, p := range tad.Stats {
				result = append(result, []string{p.Id, p.SourceIP, p.SourceTransportPort, p.DestinationIP, p.DestinationTransportPort, p.FlowStartSeconds, p.FlowEndSeconds, p.Throughput, p.AggType, p.AlgoType, p.AlgoCalc, p.Anomaly})
			}
		case "pod":
			if tad.Stats[0].PodName != "" {
				result = append(result, []string{"id", "podNamespace", "podName", "direction", "flowEndSeconds", "throughput", "aggType", "algoType", "algoCalc", "anomaly"})
				for _, p := range tad.Stats {
					result = append(result, []string{p.Id, p.PodNamespace, p.PodName, p.Direction, p.FlowEndSeconds, p.Throughput, p.AggType, p.AlgoType, p.AlgoCalc, p.Anomaly})
				}
			} else {
				result = append(result, []string{"id", "podNamespace", "podLabels", "direction", "flowEndSeconds", "throughput", "aggType", "algoType", "algoCalc", "anomaly"})
				for _, p := range tad.Stats {
					result = append(result, []string{p.Id, p.PodNamespace, p.PodLabels, p.Direction, p.FlowEndSeconds, p.Throughput, p.AggType, p.AlgoType, p.AlgoCalc, p.Anomaly})
				}
			}
		case "external":
			result = append(result, []string{"id", "destinationIP", "flowEndSeconds", "throughput", "aggType", "algoType", "algoCalc", "anomaly"})
			for _, p := range tad.Stats {
				result = append(result, []string{p.Id, p.DestinationIP, p.FlowEndSeconds, p.Throughput, p.AggType, p.AlgoType, p.AlgoCalc, p.Anomaly})
			}
		case "svc":
			result = append(result, []string{"id", "destinationServicePortName", "flowEndSeconds", "throughput", "aggType", "algoType", "algoCalc", "anomaly"})
			for _, p := range tad.Stats {
				result = append(result, []string{p.Id, p.DestinationServicePortName, p.FlowEndSeconds, p.Throughput, p.AggType, p.AlgoType, p.AlgoCalc, p.Anomaly})
			}
		}
		TableOutput(result)
	}
	return nil
}
