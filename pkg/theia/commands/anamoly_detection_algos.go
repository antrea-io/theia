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
	"regexp"
	"time"

	"antrea.io/theia/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	"antrea.io/theia/pkg/theia/commands/config"
)

// throughputAnomalyDetectionEMWACmd represents the policy-recommendation delete command
var throughputAnomalyDetectionAlgoCmd = &cobra.Command{
	Use:   "algo",
	Short: "throughput Anomaly detection using Algo (Default: EMWA)",
	Long:  `throughput Anomaly detection using Algorythm, currently supported Algorythms are EMWA, ARIMA and DBSCAN`,
	Example: `Run the spefic Algorythm for throughput Anomaly Detection
	$ theia throughput-anomaly-detection --type ARIMA --start-time '2022-01-01 00:00:00' --end-time '2022-01-31 23:59:59'
	Run thoughput anomaly detection of type ARIMA starting from '2022-01-01 00:00:00' untill '2022-01-31 23:59:59'`,
	RunE: throughputAnomalyDetectionAlgo,
}

var (
	algoName string
)

func throughputAnomalyDetectionAlgo(cmd *cobra.Command, args []string) error {
	throughputAnomalyDetection := anomalydetector.ThroughputAnomalyDetector{}
	algoName, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}
	if algoName == "" && len(args) == 1 {
		algoName = args[0]
	}
	err = util.ParseADAlgorythmName(algoName)
	if err != nil {
		return err
	}
	throughputAnomalyDetection.Type = algoName

	startTime, err := cmd.Flags().GetString("start-time")
	if err != nil {
		return err
	}
	var startTimeObj time.Time
	if startTime != "" {
		startTimeObj, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return fmt.Errorf(`parsing start-time: %v, start-time should be in 
'YYYY-MM-DD hh:mm:ss' format, for example: 2006-01-02 15:04:05`, err)
		}
		throughputAnomalyDetection.StartInterval = metav1.NewTime(startTimeObj)
	}

	endTime, err := cmd.Flags().GetString("end-time")
	if err != nil {
		return err
	}
	if endTime != "" {
		endTimeObj, err := time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return fmt.Errorf(`parsing end-time: %v, end-time should be in 
'YYYY-MM-DD hh:mm:ss' format, for example: 2006-01-02 15:04:05`, err)
		}
		endAfterStart := endTimeObj.After(startTimeObj)
		if !endAfterStart {
			return fmt.Errorf("end-time should be after start-time")
		}
		throughputAnomalyDetection.EndInterval = metav1.NewTime(endTimeObj)
	}

	executorInstances, err := cmd.Flags().GetInt32("executor-instances")
	if err != nil {
		return err
	}
	if executorInstances < 0 {
		return fmt.Errorf("executor-instances should be an integer >= 0")
	}
	throughputAnomalyDetection.ExecutorInstances = int(executorInstances)

	driverCoreRequest, err := cmd.Flags().GetString("driver-core-request")
	if err != nil {
		return err
	}
	matchResult, err := regexp.MatchString(config.K8sQuantitiesReg, driverCoreRequest)
	if err != nil || !matchResult {
		return fmt.Errorf("driver-core-request should conform to the Kubernetes resource quantity convention")
	}
	throughputAnomalyDetection.DriverCoreRequest = driverCoreRequest

	driverMemory, err := cmd.Flags().GetString("driver-memory")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, driverMemory)
	if err != nil || !matchResult {
		return fmt.Errorf("driver-memory should conform to the Kubernetes resource quantity convention")
	}
	throughputAnomalyDetection.DriverMemory = driverMemory

	executorCoreRequest, err := cmd.Flags().GetString("executor-core-request")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, executorCoreRequest)
	if err != nil || !matchResult {
		return fmt.Errorf("executor-core-request should conform to the Kubernetes resource quantity convention")
	}
	throughputAnomalyDetection.ExecutorCoreRequest = executorCoreRequest

	executorMemory, err := cmd.Flags().GetString("executor-memory")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, executorMemory)
	if err != nil || !matchResult {
		return fmt.Errorf("executor-memory should conform to the Kubernetes resource quantity convention")
	}
	throughputAnomalyDetection.ExecutorMemory = executorMemory

	throughputAnomalyDetection.Namespace = config.FlowVisibilityNS

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
	err = theiaClient.Post().
		AbsPath("/apis/anomalydetector.theia.antrea.io/v1alpha1/").
		Resource("anomalydetectors").
		Body(&throughputAnomalyDetection).
		Do(context.TODO()).
		Error()
	if err != nil {
		return fmt.Errorf("error when deleting Throughput Anomaly Detection job: %v", err)
	}
	fmt.Printf("Successfully started Throughput Anomaly Detection job with name: %s\n", algoName)
	return nil
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(throughputAnomalyDetectionAlgoCmd)
	throughputAnomalyDetectionAlgoCmd.Flags().StringVarP(&algoName, "type", "t", "", "type of algorythm to use")

	err := throughputAnomalyDetectionAlgoCmd.MarkFlagRequired("type")

	if err != nil {
		fmt.Printf("Algo type not specified: %v", err)
	}
	throughputAnomalyDetectionAlgoCmd.Flags().StringP(
		"start-time",
		"s",
		"",
		`The start time of the flow records considered for the policy recommendation.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the start time of flow records by default.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().StringP(
		"end-time",
		"e",
		"",
		`The end time of the flow records considered for the policy recommendation.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the end time of flow records by default.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().Int32(
		"executor-instances",
		1,
		"Specify the number of executors for the Spark application. Example values include 1, 2, 8, etc.",
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"driver-core-request",
		"200m",
		`Specify the CPU request for the driver Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 0.1, 500m, 1.5, 5, etc.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"driver-memory",
		"512M",
		`Specify the memory request for the driver Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 512M, 1G, 8G, etc.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"executor-core-request",
		"200m",
		`Specify the CPU request for the executor Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 0.1, 500m, 1.5, 5, etc.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"executor-memory",
		"512M",
		`Specify the memory request for the executor Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 512M, 1G, 8G, etc.`,
	)

}
