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
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/theia/commands/config"
)

// policyRecommendationRunCmd represents the policy recommendation run command
var policyRecommendationRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a new policy recommendation job",
	Long: `Run a new policy recommendation job. 
Must finish the deployment of Theia first`,
	Example: `Run a policy recommendation job with default configuration
$ theia policy-recommendation run
Run an initial policy recommendation job with policy type anp-deny-applied and limit on last 10k flow records
$ theia policy-recommendation run --type initial --policy-type anp-deny-applied --limit 10000
Run an initial policy recommendation job with policy type anp-deny-applied and limit on flow records from 2022-01-01 00:00:00 to 2022-01-31 23:59:59.
$ theia policy-recommendation run --type initial --policy-type anp-deny-applied --start-time '2022-01-01 00:00:00' --end-time '2022-01-31 23:59:59'
Run a policy recommendation job with default configuration but doesn't recommend toServices ANPs
$ theia policy-recommendation run --to-services=false
`,
	RunE: policyRecommendationRun,
}

func policyRecommendationRun(cmd *cobra.Command, args []string) error {
	networkPolicyRecommendation := intelligence.NetworkPolicyRecommendation{}
	recoType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}
	if recoType != "initial" && recoType != "subsequent" {
		return fmt.Errorf("recommendation type should be 'initial' or 'subsequent'")
	}
	networkPolicyRecommendation.Type = recoType

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	if limit < 0 {
		return fmt.Errorf("limit should be an integer >= 0")
	}
	networkPolicyRecommendation.Limit = limit

	policyType, err := cmd.Flags().GetString("policy-type")
	if err != nil {
		return err
	}
	if policyType != "anp-deny-applied" && policyType != "anp-deny-all" && policyType != "k8s-np" {
		return fmt.Errorf(`type of generated NetworkPolicy should be
anp-deny-applied or anp-deny-all or k8s-np`)
	}
	networkPolicyRecommendation.PolicyType = policyType

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
		networkPolicyRecommendation.StartInterval = metav1.NewTime(startTimeObj)
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
		networkPolicyRecommendation.EndInterval = metav1.NewTime(endTimeObj)
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
		networkPolicyRecommendation.NSAllowList = parsedNsAllowList
	}

	excludeLabels, err := cmd.Flags().GetBool("exclude-labels")
	if err != nil {
		return err
	}
	networkPolicyRecommendation.ExcludeLabels = excludeLabels

	toServices, err := cmd.Flags().GetBool("to-services")
	if err != nil {
		return err
	}
	networkPolicyRecommendation.ToServices = toServices

	executorInstances, err := cmd.Flags().GetInt32("executor-instances")
	if err != nil {
		return err
	}
	if executorInstances < 0 {
		return fmt.Errorf("executor-instances should be an integer >= 0")
	}
	networkPolicyRecommendation.ExecutorInstances = int(executorInstances)

	driverCoreRequest, err := cmd.Flags().GetString("driver-core-request")
	if err != nil {
		return err
	}
	matchResult, err := regexp.MatchString(config.K8sQuantitiesReg, driverCoreRequest)
	if err != nil || !matchResult {
		return fmt.Errorf("driver-core-request should conform to the Kubernetes resource quantity convention")
	}
	networkPolicyRecommendation.DriverCoreRequest = driverCoreRequest

	driverMemory, err := cmd.Flags().GetString("driver-memory")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, driverMemory)
	if err != nil || !matchResult {
		return fmt.Errorf("driver-memory should conform to the Kubernetes resource quantity convention")
	}
	networkPolicyRecommendation.DriverMemory = driverMemory

	executorCoreRequest, err := cmd.Flags().GetString("executor-core-request")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, executorCoreRequest)
	if err != nil || !matchResult {
		return fmt.Errorf("executor-core-request should conform to the Kubernetes resource quantity convention")
	}
	networkPolicyRecommendation.ExecutorCoreRequest = executorCoreRequest

	executorMemory, err := cmd.Flags().GetString("executor-memory")
	if err != nil {
		return err
	}
	matchResult, err = regexp.MatchString(config.K8sQuantitiesReg, executorMemory)
	if err != nil || !matchResult {
		return fmt.Errorf("executor-memory should conform to the Kubernetes resource quantity convention")
	}
	networkPolicyRecommendation.ExecutorMemory = executorMemory

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

	waitFlag, err := cmd.Flags().GetBool("wait")
	if err != nil {
		return err
	}

	recoID := uuid.New().String()
	networkPolicyRecommendation.Name = "pr-" + recoID
	networkPolicyRecommendation.Namespace = config.FlowVisibilityNS

	err = theiaClient.Post().
		AbsPath("/apis/intelligence.theia.antrea.io/v1alpha1/").
		Resource("networkpolicyrecommendations").
		Body(&networkPolicyRecommendation).
		Do(context.TODO()).Error()
	if err != nil {
		return fmt.Errorf("failed to post policy recommendation job: %v", err)
	}
	if waitFlag {
		var npr intelligence.NetworkPolicyRecommendation
		err = wait.Poll(config.StatusCheckPollInterval, config.StatusCheckPollTimeout, func() (bool, error) {
			npr, err = getPolicyRecommendationByName(theiaClient, networkPolicyRecommendation.Name)
			if err != nil {
				return false, fmt.Errorf("error when getting policy recommendation job by job name: %v", err)
			}
			state := npr.Status.State
			if state == crdv1alpha1.NPRecommendationStateCompleted {
				return true, nil
			}
			if state == crdv1alpha1.NPRecommendationStateFailed {
				return false, fmt.Errorf("policy recommendation job failed, Error Message: %s", npr.Status.ErrorMsg)
			} else {
				return false, nil
			}
		})
		if err != nil {
			if strings.Contains(err.Error(), "timed out") {
				return fmt.Errorf(`policy recommendation job with name %s wait timeout of 60 minutes expired.
				Job is still running. Please check completion status for job via CLI later`, networkPolicyRecommendation.Name)
			}
			return err
		}
		if npr.Status.RecommendationOutcome != "" {
			fmt.Print(npr.Status.RecommendationOutcome)
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
	} else {
		fmt.Printf("Successfully created policy recommendation job with name %s\n", networkPolicyRecommendation.Name)
	}
	return nil
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
