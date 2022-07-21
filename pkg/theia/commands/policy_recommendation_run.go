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
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const (
	flowVisibilityNS        = "flow-visibility"
	k8sQuantitiesReg        = "^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	sparkImage              = "projects.registry.vmware.com/antrea/theia-policy-recommendation:latest"
	sparkImagePullPolicy    = "IfNotPresent"
	sparkAppFile            = "local:///opt/spark/work-dir/policy_recommendation_job.py"
	sparkServiceAccount     = "policy-recommendation-spark"
	sparkVersion            = "3.1.1"
	statusCheckPollInterval = 5 * time.Second
	statusCheckPollTimeout  = 60 * time.Minute
)

type SparkResourceArgs struct {
	executorInstances   int32
	driverCoreRequest   string
	driverMemory        string
	executorCoreRequest string
	executorMemory      string
}

// policyRecommendationRunCmd represents the policy recommendation run command
var policyRecommendationRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a new policy recommendation Spark job",
	Long: `Run a new policy recommendation Spark job. 
Must finish the deployment of Theia first`,
	Example: `Run a policy recommendation Spark job with default configuration
$ theia policy-recommendation run
Run an initial policy recommendation Spark job with policy type anp-deny-applied and limit on last 10k flow records
$ theia policy-recommendation run --type initial --policy-type anp-deny-applied --limit 10000
Run an initial policy recommendation Spark job with policy type anp-deny-applied and limit on flow records from 2022-01-01 00:00:00 to 2022-01-31 23:59:59.
$ theia policy-recommendation run --type initial --policy-type anp-deny-applied --start-time '2022-01-01 00:00:00' --end-time '2022-01-31 23:59:59'
Run a policy recommendation Spark job with default configuration but doesn't recommend toServices ANPs
$ theia policy-recommendation run --to-services=false
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var recoJobArgs []string
		sparkResourceArgs := SparkResourceArgs{}

		recoType, err := cmd.Flags().GetString("type")
		if err != nil {
			return err
		}
		if recoType != "initial" && recoType != "subsequent" {
			return fmt.Errorf("recommendation type should be 'initial' or 'subsequent'")
		}
		recoJobArgs = append(recoJobArgs, "--type", recoType)

		limit, err := cmd.Flags().GetInt("limit")
		if err != nil {
			return err
		}
		if limit < 0 {
			return fmt.Errorf("limit should be an integer >= 0")
		}
		recoJobArgs = append(recoJobArgs, "--limit", strconv.Itoa(limit))

		policyType, err := cmd.Flags().GetString("policy-type")
		if err != nil {
			return err
		}
		var policyTypeArg int
		if policyType == "anp-deny-applied" {
			policyTypeArg = 1
		} else if policyType == "anp-deny-all" {
			policyTypeArg = 2
		} else if policyType == "k8s-np" {
			policyTypeArg = 3
		} else {
			return fmt.Errorf(`type of generated NetworkPolicy should be
anp-deny-applied or anp-deny-all or k8s-np`)
		}
		recoJobArgs = append(recoJobArgs, "--option", strconv.Itoa(policyTypeArg))

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
			recoJobArgs = append(recoJobArgs, "--start_time", startTime)
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
			recoJobArgs = append(recoJobArgs, "--end_time", endTime)
		}

		nsAllowList, err := cmd.Flags().GetString("ns-allow-list")
		if err != nil {
			return err
		}
		if nsAllowList != "" {
			var parsedNsAllowList []string
			err := json.Unmarshal([]byte(nsAllowList), &parsedNsAllowList)
			if err != nil {
				return fmt.Errorf(`parsing ns-allow-list: %v, ns-allow-list should 
be a list of namespace string, for example: '["kube-system","flow-aggregator","flow-visibility"]'`, err)
			}
			recoJobArgs = append(recoJobArgs, "--ns_allow_list", nsAllowList)
		}

		excludeLabels, err := cmd.Flags().GetBool("exclude-labels")
		if err != nil {
			return err
		}
		recoJobArgs = append(recoJobArgs, "--rm_labels", strconv.FormatBool(excludeLabels))

		toServices, err := cmd.Flags().GetBool("to-services")
		if err != nil {
			return err
		}
		recoJobArgs = append(recoJobArgs, "--to_services", strconv.FormatBool(toServices))

		executorInstances, err := cmd.Flags().GetInt32("executor-instances")
		if err != nil {
			return err
		}
		if executorInstances < 0 {
			return fmt.Errorf("executor-instances should be an integer >= 0")
		}
		sparkResourceArgs.executorInstances = executorInstances

		driverCoreRequest, err := cmd.Flags().GetString("driver-core-request")
		if err != nil {
			return err
		}
		matchResult, err := regexp.MatchString(k8sQuantitiesReg, driverCoreRequest)
		if err != nil || !matchResult {
			return fmt.Errorf("driver-core-request should conform to the Kubernetes resource quantity convention")
		}
		sparkResourceArgs.driverCoreRequest = driverCoreRequest

		driverMemory, err := cmd.Flags().GetString("driver-memory")
		if err != nil {
			return err
		}
		matchResult, err = regexp.MatchString(k8sQuantitiesReg, driverMemory)
		if err != nil || !matchResult {
			return fmt.Errorf("driver-memory should conform to the Kubernetes resource quantity convention")
		}
		sparkResourceArgs.driverMemory = driverMemory

		executorCoreRequest, err := cmd.Flags().GetString("executor-core-request")
		if err != nil {
			return err
		}
		matchResult, err = regexp.MatchString(k8sQuantitiesReg, executorCoreRequest)
		if err != nil || !matchResult {
			return fmt.Errorf("executor-core-request should conform to the Kubernetes resource quantity convention")
		}
		sparkResourceArgs.executorCoreRequest = executorCoreRequest

		executorMemory, err := cmd.Flags().GetString("executor-memory")
		if err != nil {
			return err
		}
		matchResult, err = regexp.MatchString(k8sQuantitiesReg, executorMemory)
		if err != nil || !matchResult {
			return fmt.Errorf("executor-memory should conform to the Kubernetes resource quantity convention")
		}
		sparkResourceArgs.executorMemory = executorMemory

		kubeconfig, err := ResolveKubeConfig(cmd)
		if err != nil {
			return err
		}
		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
		}

		waitFlag, err := cmd.Flags().GetBool("wait")
		if err != nil {
			return err
		}

		err = PolicyRecoPreCheck(clientset)
		if err != nil {
			return err
		}

		recommendationID := uuid.New().String()
		recoJobArgs = append(recoJobArgs, "--id", recommendationID)
		recommendationApplication := &sparkv1.SparkApplication{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "sparkoperator.k8s.io/v1beta2",
				Kind:       "SparkApplication",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pr-" + recommendationID,
				Namespace: flowVisibilityNS,
			},
			Spec: sparkv1.SparkApplicationSpec{
				Type:                "Python",
				SparkVersion:        sparkVersion,
				Mode:                "cluster",
				Image:               ConstStrToPointer(sparkImage),
				ImagePullPolicy:     ConstStrToPointer(sparkImagePullPolicy),
				MainApplicationFile: ConstStrToPointer(sparkAppFile),
				Arguments:           recoJobArgs,
				Driver: sparkv1.DriverSpec{
					CoreRequest: &driverCoreRequest,
					SparkPodSpec: sparkv1.SparkPodSpec{
						Memory: &driverMemory,
						Labels: map[string]string{
							"version": sparkVersion,
						},
						EnvSecretKeyRefs: map[string]sparkv1.NameKey{
							"CH_USERNAME": {
								Name: "clickhouse-secret",
								Key:  "username",
							},
							"CH_PASSWORD": {
								Name: "clickhouse-secret",
								Key:  "password",
							},
						},
						ServiceAccount: ConstStrToPointer(sparkServiceAccount),
					},
				},
				Executor: sparkv1.ExecutorSpec{
					CoreRequest: &executorCoreRequest,
					SparkPodSpec: sparkv1.SparkPodSpec{
						Memory: &executorMemory,
						Labels: map[string]string{
							"version": sparkVersion,
						},
						EnvSecretKeyRefs: map[string]sparkv1.NameKey{
							"CH_USERNAME": {
								Name: "clickhouse-secret",
								Key:  "username",
							},
							"CH_PASSWORD": {
								Name: "clickhouse-secret",
								Key:  "password",
							},
						},
					},
					Instances: &sparkResourceArgs.executorInstances,
				},
			},
		}
		response := &sparkv1.SparkApplication{}
		err = clientset.CoreV1().RESTClient().
			Post().
			AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
			Namespace(flowVisibilityNS).
			Resource("sparkapplications").
			Body(recommendationApplication).
			Do(context.TODO()).
			Into(response)
		if err != nil {
			return err
		}
		if waitFlag {
			err = wait.Poll(statusCheckPollInterval, statusCheckPollTimeout, func() (bool, error) {
				state, err := getPolicyRecommendationStatus(clientset, recommendationID)
				if err != nil {
					return false, err
				}
				if state == "COMPLETED" {
					return true, nil
				}
				if state == "FAILED" || state == "SUBMISSION_FAILED" || state == "FAILING" || state == "INVALIDATING" {
					return false, fmt.Errorf("policy recommendation job failed, state: %s", state)
				} else {
					return false, nil
				}
			})
			if err != nil {
				if strings.Contains(err.Error(), "timed out") {
					return fmt.Errorf(`Spark job with ID %s wait timeout of 60 minutes expired.
Job is still running. Please check completion status for job via CLI later.`, recommendationID)
				}
				return err
			}

			endpoint, err := cmd.Flags().GetString("clickhouse-endpoint")
			if err != nil {
				return err
			}
			if endpoint != "" {
				err = ParseEndpoint(endpoint)
				if err != nil {
					return err
				}
			}
			useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
			if err != nil {
				return err
			}
			filePath, err := cmd.Flags().GetString("file")
			if err != nil {
				return err
			}
			if err := CheckClickHousePod(clientset); err != nil {
				return err
			}
			recoResult, err := getPolicyRecommendationResult(clientset, kubeconfig, endpoint, useClusterIP, filePath, recommendationID)
			if err != nil {
				return err
			} else {
				if recoResult != "" {
					fmt.Print(recoResult)
				}
			}
			return nil
		} else {
			fmt.Printf("Successfully created policy recommendation job with ID %s\n", recommendationID)
		}
		return nil
	},
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationRunCmd)
	policyRecommendationRunCmd.Flags().StringP(
		"type",
		"t",
		"initial",
		"{initial|subsequent} Indicates this recommendation is an initial recommendion or a subsequent recommendation job.",
	)
	policyRecommendationRunCmd.Flags().IntP(
		"limit",
		"l",
		0,
		"The limit on the number of flow records read from the database. 0 means no limit.",
	)
	policyRecommendationRunCmd.Flags().StringP(
		"policy-type",
		"p",
		"anp-deny-applied",
		`Types of generated NetworkPolicy.
Currently we have 3 generated NetworkPolicy types:
anp-deny-applied: Recommending allow ANP/ACNP policies, with default deny rules only on Pods which have an allow rule applied.
anp-deny-all: Recommending allow ANP/ACNP policies, with default deny rules for whole cluster.
k8s-np: Recommending allow K8s NetworkPolicies.`,
	)
	policyRecommendationRunCmd.Flags().StringP(
		"start-time",
		"s",
		"",
		`The start time of the flow records considered for the policy recommendation.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the start time of flow records by default.`,
	)
	policyRecommendationRunCmd.Flags().StringP(
		"end-time",
		"e",
		"",
		`The end time of the flow records considered for the policy recommendation.
Format is YYYY-MM-DD hh:mm:ss in UTC timezone. No limit of the end time of flow records by default.`,
	)
	policyRecommendationRunCmd.Flags().StringP(
		"ns-allow-list",
		"n",
		"",
		`List of default allow Namespaces.
If no Namespaces provided, Traffic inside Antrea CNI related Namespaces: ['kube-system', 'flow-aggregator',
'flow-visibility'] will be allowed by default.`,
	)
	policyRecommendationRunCmd.Flags().Bool(
		"exclude-labels",
		true,
		`Enable this option will exclude automatically generated Pod labels including 'pod-template-hash',
'controller-revision-hash', 'pod-template-generation' during policy recommendation.`,
	)
	policyRecommendationRunCmd.Flags().Bool(
		"to-services",
		true,
		`Use the toServices feature in ANP and recommendation toServices rules for Pod-to-Service flows,
only works when option is anp-deny-applied or anp-deny-all.`,
	)
	policyRecommendationRunCmd.Flags().Int32(
		"executor-instances",
		1,
		"Specify the number of executors for the Spark application. Example values include 1, 2, 8, etc.",
	)
	policyRecommendationRunCmd.Flags().String(
		"driver-core-request",
		"200m",
		`Specify the CPU request for the driver Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 0.1, 500m, 1.5, 5, etc.`,
	)
	policyRecommendationRunCmd.Flags().String(
		"driver-memory",
		"512M",
		`Specify the memory request for the driver Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 512M, 1G, 8G, etc.`,
	)
	policyRecommendationRunCmd.Flags().String(
		"executor-core-request",
		"200m",
		`Specify the CPU request for the executor Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 0.1, 500m, 1.5, 5, etc.`,
	)
	policyRecommendationRunCmd.Flags().String(
		"executor-memory",
		"512M",
		`Specify the memory request for the executor Pod. Values conform to the Kubernetes resource quantity convention.
Example values include 512M, 1G, 8G, etc.`,
	)
	policyRecommendationRunCmd.Flags().Bool(
		"wait",
		false,
		"Enable this option will hold and wait the whole policy recommendation job finishes.",
	)
	policyRecommendationRunCmd.Flags().StringP(
		"file",
		"f",
		"",
		"The file path where you want to save the result. It can only be used when wait is enabled.",
	)
}
