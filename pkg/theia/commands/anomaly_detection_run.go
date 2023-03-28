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
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	anomalydetector "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/commands/config"
	"antrea.io/theia/pkg/util"
)

// throughputAnomalyDetectionEWMACmd represents the anomaly detection delete command
var throughputAnomalyDetectionAlgoCmd = &cobra.Command{
	Use:   "run",
	Short: "throughput anomaly detection using Algo",
	Long:  `throughput anomaly detection using algorithms, currently supported algorithms are EWMA, ARIMA and DBSCAN`,
	Example: `Run the specific algorithm for throughput anomaly detection
	$ theia throughput-anomaly-detection run --algo ARIMA --start-time 2022-01-01T00:00:00 --end-time 2022-01-31T23:59:59
	Run throughput anomaly detection algorithm of type ARIMA and limit on flow records from '2022-01-01 00:00:00' to '2022-01-31 23:59:59'
	Please note, algo is a mandatory argument'`,
	RunE: throughputAnomalyDetectionAlgo,
}

func throughputAnomalyDetectionAlgo(cmd *cobra.Command, args []string) error {
	throughputAnomalyDetection := anomalydetector.ThroughputAnomalyDetector{}
	algoName, err := cmd.Flags().GetString("algo")
	if err != nil {
		return err
	}
	err = util.ParseADAlgorithmName(algoName)
	if err != nil {
		return err
	}
	throughputAnomalyDetection.Type = algoName

	startTime, err := cmd.Flags().GetString("start-time")
	if err != nil {
		return err
	}
	startTime = strings.Replace(startTime, "T", " ", 1)
	var startTimeObj time.Time
	if startTime != "" {
		startTimeObj, err = time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			return fmt.Errorf(`parsing start-time: %v, start-time should be in 
'YYYY-MM-DDThh:mm:ss' format, for example: 2006-01-02T15:04:05`, err)
		}
		throughputAnomalyDetection.StartInterval = metav1.NewTime(startTimeObj)
	}

	endTime, err := cmd.Flags().GetString("end-time")
	if err != nil {
		return err
	}
	endTime = strings.Replace(endTime, "T", " ", 1)
	if endTime != "" {
		endTimeObj, err := time.Parse("2006-01-02 15:04:05", endTime)
		if err != nil {
			return fmt.Errorf(`parsing end-time: %v, end-time should be in 
'YYYY-MM-DDThh:mm:ss' format, for example: 2006-01-02T15:04:05`, err)
		}
		endAfterStart := endTimeObj.After(startTimeObj)
		if !endAfterStart {
			return fmt.Errorf("end-time should be after start-time")
		}
		throughputAnomalyDetection.EndInterval = metav1.NewTime(endTimeObj)
	}

	nsIgnoreList, err := cmd.Flags().GetString("ns-ignore-list")
	if err != nil {
		return err
	}
	if nsIgnoreList != "" {
		var parsednsIgnoreList []string
		err := json.Unmarshal([]byte(nsIgnoreList), &parsednsIgnoreList)
		if err != nil {
			return fmt.Errorf(`parsing ns-ignore-list: %v, ns-ignore-list should 
be a list of namespace string, for example: '["kube-system","flow-aggregator","flow-visibility"]'`, err)
		}
		throughputAnomalyDetection.NSIgnoreList = parsednsIgnoreList
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

	aggregatedFlow, err := cmd.Flags().GetString("agg-flow")
	if err != nil {
		return err
	}
	if aggregatedFlow != "" {
		switch aggregatedFlow {
		case "pod":
			podLabel, err := cmd.Flags().GetString("pod-label")
			if err != nil {
				return err
			}
			if podLabel != "" {
				throughputAnomalyDetection.PodLabel = podLabel
			}
			podName, err := cmd.Flags().GetString("pod-name")
			if err != nil {
				return err
			}
			if podName != "" {
				throughputAnomalyDetection.PodName = podName
			}
			podNameSpace, err := cmd.Flags().GetString("pod-namespace")
			if err != nil {
				return err
			}
			if podNameSpace != "" {
				if podName == "" && podLabel == "" {
					return fmt.Errorf("'pod-namespace' argument can not be used alone, should be specified along pod-label or pod-name")
				} else {
					throughputAnomalyDetection.PodNameSpace = podNameSpace
				}
			}
			throughputAnomalyDetection.AggregatedFlow = aggregatedFlow
		case "external":
			externalIp, err := cmd.Flags().GetString("external-ip")
			if err != nil {
				return err
			}
			if externalIp != "" {
				throughputAnomalyDetection.ExternalIP = externalIp
			}
			throughputAnomalyDetection.AggregatedFlow = aggregatedFlow
		case "svc":
			servicePortName, err := cmd.Flags().GetString("svc-port-name")
			if err != nil {
				return err
			}
			if servicePortName != "" {
				throughputAnomalyDetection.ServicePortName = servicePortName
			}
			throughputAnomalyDetection.AggregatedFlow = aggregatedFlow
		default:
			return fmt.Errorf("throughput anomaly detector aggregated flow type should be 'pod' or 'external' or 'svc'")
		}
	}

	tadID := uuid.New().String()
	throughputAnomalyDetection.Name = "tad-" + tadID
	throughputAnomalyDetection.Namespace = config.FlowVisibilityNS

	useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
	if err != nil {
		return err
	}
	theiaClient, pf, err := SetupTheiaClientAndConnection(cmd, useClusterIP)
	if err != nil {
		return fmt.Errorf("throughput anomaly detection couldn't setup Theia manager client, %v", err)
	}
	if pf != nil {
		defer pf.Stop()
	}
	err = theiaClient.Post().
		AbsPath("/apis/intelligence.theia.antrea.io/v1alpha1/").
		Resource("throughputanomalydetectors").
		Body(&throughputAnomalyDetection).
		Do(context.TODO()).
		Error()
	if err != nil {
		return fmt.Errorf("failed to Post Throughput Anomaly Detection job: %v", err)
	}
	fmt.Printf("Successfully started Throughput Anomaly Detection job with name: %s\n", throughputAnomalyDetection.Name)
	return nil
}

func init() {
	throughputanomalyDetectionCmd.AddCommand(throughputAnomalyDetectionAlgoCmd)
	throughputAnomalyDetectionAlgoCmd.Flags().StringP("algo", "a", "",
		`The algorithm used by throughput anomaly detection.
		Currently supported Algorithms are EWMA, ARIMA and DBSCAN.`)

	err := throughputAnomalyDetectionAlgoCmd.MarkFlagRequired("algo")
	if err != nil {
		fmt.Printf("Algo type not specified: %v", err)
	}

	throughputAnomalyDetectionAlgoCmd.Flags().StringP(
		"start-time",
		"s",
		"",
		`The start time of the flow records considered for the anomaly detection.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the start time of flow records by default.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().StringP(
		"end-time",
		"e",
		"",
		`The end time of the flow records considered for the anomaly detection.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the end time of flow records by default.`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().StringP(
		"ns-ignore-list",
		"n",
		"",
		`List of default drop Namespaces. Use this to ignore traffic from selected namespaces
If no Namespaces provided, Traffic from all namespaces present in flows table will be allowed by default.`,
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
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"agg-flow",
		"",
		`Specifies which aggregated flow to perform anomaly detection on, options are pod/svc/external`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"pod-label",
		"",
		`On choosing agg-flow as pod, user has option to specify labels for inbound/outbound throughput, default would be all labels`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"pod-name",
		"",
		`On choosing agg-flow as pod, user has option to specify podname for inbound/outbound throughput, if pod-labels specified, that will take priority`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"pod-namespace",
		"",
		`On choosing agg-flow as pod, user has option to specify podnamespace for inbound/outbound throughput, podnamespace argument should be combined with podlabels or podname`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"external-ip",
		"",
		`On choosing agg-flow as external, user has option to specify external-ip for inbound throughput, default would be all IPs`,
	)
	throughputAnomalyDetectionAlgoCmd.Flags().String(
		"svc-port-name",
		"",
		`On choosing agg-flow as svc, user has option to specify svc-port-name for inbound throughput, default would be all service port names`,
	)
}
