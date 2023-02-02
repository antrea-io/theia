// Copyright 2022 Antrea Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"antrea.io/theia/snowflake/pkg/udfs"
	"antrea.io/theia/snowflake/pkg/utils/timestamps"
)

const (
	staticPolicyRecommendationFunctionName     = "static_policy_recommendation"
	preprocessingFunctionName                  = "preprocessing"
	policyRecommendationFunctionName           = "policy_recommendation"
	defaultPolicyRecommendationFunctionVersion = "v0.1.1"
	defaultPolicyRecommendationWaitTimeout     = "10m"
	// Limit the number of rows per partition to avoid hitting the 5 minutes end_partition() timeout.
	partitionSizeLimit = 50000
)

func buildPolicyRecommendationUdfQuery(
	jobType string,
	limit uint,
	isolationMethod int,
	start string,
	end string,
	startTs string,
	endTs string,
	nsAllowList string,
	labelIgnoreList string,
	clusterUUID string,
	databaseName string,
	functionVersion string,
) (string, error) {
	now := time.Now()
	recommendationID := uuid.New().String()
	functionName := udfs.GetFunctionName(staticPolicyRecommendationFunctionName, functionVersion)
	var queryBuilder strings.Builder
	fmt.Fprintf(&queryBuilder, `SELECT r.job_type, r.recommendation_id, r.time_created, r.yamls FROM
	TABLE(%s(
	  '%s',
	  '%s',
	  %d,
	  '%s'
	) over (partition by 1)) as r;
`, functionName, jobType, recommendationID, isolationMethod, nsAllowList)

	queryBuilder.WriteString(`WITH filtered_flows AS (
SELECT
  sourcePodNamespace,
  sourcePodLabels,
  destinationIP,
  destinationPodNamespace,
  destinationPodLabels,
  destinationServicePortName,
  destinationTransportPort,
  protocolIdentifier,
  flowType
FROM
  flows
WHERE
  ingressNetworkPolicyName IS NULL
AND
  egressNetworkPolicyName IS NULL
`)

	var startTime string
	if startTs != "" {
		startTime = startTs
	} else if start != "" {
		var err error
		startTime, err = timestamps.ParseTimestamp(start, now)
		if err != nil {
			return "", err
		}
	}
	if startTime != "" {
		fmt.Fprintf(&queryBuilder, `AND
  flowStartSeconds >= '%s'
`, startTime)
	}

	var endTime string
	if endTs != "" {
		endTime = endTs
	} else if end != "" {
		var err error
		endTime, err = timestamps.ParseTimestamp(end, now)
		if err != nil {
			return "", err
		}
	}
	if endTime != "" {
		fmt.Fprintf(&queryBuilder, `AND
  flowEndSeconds < '%s'
`, endTime)
	}

	if clusterUUID != "" {
		_, err := uuid.Parse(clusterUUID)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&queryBuilder, `AND
  clusterUUID = '%s'
`, clusterUUID)
	} else {
		logger.Info("No clusterUUID input, all flows will be considered during policy recommendation.")
	}

	queryBuilder.WriteString(`GROUP BY
sourcePodNamespace,
sourcePodLabels,
destinationIP,
destinationPodNamespace,
destinationPodLabels,
destinationServicePortName,
destinationTransportPort,
protocolIdentifier,
flowType
`)

	if limit > 0 {
		fmt.Fprintf(&queryBuilder, `
LIMIT %d`, limit)
	} else {
		// limit the number unique flow records to 500k to avoid udf timeout
		queryBuilder.WriteString(`
LIMIT 500000`)
	}

	// Choose the destinationIP as the partition field for the preprocessing
	// UDTF because flow rows could be divided into the most subsets
	functionName = udfs.GetFunctionName(preprocessingFunctionName, functionVersion)
	fmt.Fprintf(&queryBuilder, `), processed_flows AS (SELECT r.applied_to, r.ingress, r.egress FROM filtered_flows AS f,
TABLE(%s(
	'%s',
	%d,
	'%s',
	'%s',
  f.sourcePodNamespace,
  f.sourcePodLabels,
  f.destinationIP,
  f.destinationPodNamespace,
  f.destinationPodLabels,
  f.destinationServicePortName,
  f.destinationTransportPort,
  f.protocolIdentifier,
  f.flowType
) over (partition by f.destinationIP)) as r
`, functionName, jobType, isolationMethod, nsAllowList, labelIgnoreList)

	// Scan the row number for each applied_to group and divide the partitions
	// larger than partitionSizeLimit.
	fmt.Fprintf(&queryBuilder, `), pf_with_index AS (
SELECT 
  pf.applied_to, 
  pf.ingress, 
  pf.egress, 
  floor((Row_number() over (partition by pf.applied_to order by egress))/%d) as row_index 
FROM processed_flows as pf
`, partitionSizeLimit)

	// Choose the applied_to as the partition field for the policyRecommendation
	// UDTF because each network policy is recommended based on all ingress and
	// egress traffic related to an applied_to group.
	functionName = udfs.GetFunctionName(policyRecommendationFunctionName, functionVersion)
	fmt.Fprintf(&queryBuilder, `) SELECT r.job_type, r.recommendation_id, r.time_created, r.yamls FROM pf_with_index,
TABLE(%s(
  '%s',
  '%s',
  %d,
  '%s',
  pf_with_index.applied_to,
  pf_with_index.ingress,
  pf_with_index.egress
) over (partition by pf_with_index.applied_to, pf_with_index.row_index)) as r
`, functionName, jobType, recommendationID, isolationMethod, nsAllowList)

	return queryBuilder.String(), nil
}

// policyRecommendationCmd represents the policy-recommendation command
var policyRecommendationCmd = &cobra.Command{
	Use:   "policy-recommendation",
	Short: "Run the policy recommendation UDF in Snowflake",
	Long: `This command runs the policy recommendation UDF in Snowflake.
You need to bring your own Snowflake account and run the onboard command first.

Run policy recommendation with default configuration on database ANTREA_C9JR8KUKUIV4R72S:
"theia-sf policy-recommendation --database-name ANTREA_C9JR8KUKUIV4R72S"

The "policy-recommendation" command requires a Snowflake warehouse to run policy
recommendation UDFs in Snowflake. By default, it will create a temporary one.
You can also bring your own by using the "--warehouse-name" parameter.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jobType, _ := cmd.Flags().GetString("type")
		if jobType != "initial" {
			return fmt.Errorf("invalid --type argument")
		}
		limit, _ := cmd.Flags().GetUint("limit")

		policyType, _ := cmd.Flags().GetString("policy-type")
		var isolationMethod int
		if policyType == "anp-deny-applied" {
			isolationMethod = 1
		} else if policyType == "anp-deny-all" {
			isolationMethod = 2
		} else if policyType == "k8s-np" {
			isolationMethod = 3
		} else {
			return fmt.Errorf(`type of generated NetworkPolicy should be
anp-deny-applied or anp-deny-all or k8s-np`)
		}

		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		startTs, _ := cmd.Flags().GetString("start-ts")
		endTs, _ := cmd.Flags().GetString("end-ts")
		nsAllowList, _ := cmd.Flags().GetString("ns-allow")
		labelIgnoreList, _ := cmd.Flags().GetString("label-ignore")
		clusterUUID, _ := cmd.Flags().GetString("cluster-uuid")
		databaseName, _ := cmd.Flags().GetString("database-name")
		warehouseName, _ := cmd.Flags().GetString("warehouse-name")
		functionVersion, _ := cmd.Flags().GetString("udf-version")
		waitTimeout, _ := cmd.Flags().GetString("wait-timeout")
		waitDuration, err := time.ParseDuration(waitTimeout)
		if err != nil {
			return fmt.Errorf("invalid --wait-timeout argument, err when parsing it as a duration: %v", err)
		}
		query, err := buildPolicyRecommendationUdfQuery(jobType, limit, isolationMethod, start, end, startTs, endTs, nsAllowList, labelIgnoreList, clusterUUID, databaseName, functionVersion)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), waitDuration)
		defer cancel()
		rows, err := udfs.RunUdf(ctx, logger, query, databaseName, warehouseName)
		if err != nil {
			return fmt.Errorf("error when running policy recommendation UDF: %w", err)
		}
		defer rows.Close()

		var recommendationID string
		var timeCreated string
		var yamls string
		for cont := true; cont; cont = rows.NextResultSet() {
			for rows.Next() {
				if err := rows.Scan(&jobType, &recommendationID, &timeCreated, &yamls); err != nil {
					return fmt.Errorf("invalid row: %w", err)
				}
				fmt.Printf("%s---\n", yamls)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(policyRecommendationCmd)

	policyRecommendationCmd.Flags().String("type", "initial", "Type of recommendation job (initial|subsequent), we only support initial jobType for now")
	policyRecommendationCmd.Flags().Uint("limit", 0, "Limit on the number of flows to read, default is 0 (no limit)")
	policyRecommendationCmd.Flags().String("policy-type", "anp-deny-applied", `Types of recommended NetworkPolicy. Currently we have 3 options:
anp-deny-applied: Recommending allow ANP/ACNP policies, with default deny rules only on Pods which have an allow rule applied
anp-deny-all: Recommending allow ANP/ACNP policies, with default deny rules for whole cluster
k8s-np: Recommending allow K8s NetworkPolicies only`)
	policyRecommendationCmd.Flags().String("start", "", "Start time for flows, with reference to the current time (e.g., now-1h)")
	policyRecommendationCmd.Flags().String("end", "", "End time for flows, with reference to the current timr (e.g., now)")
	policyRecommendationCmd.Flags().String("start-ts", "", "Start time for flows, as a RFC3339 UTC timestamp (e.g., 2022-07-01T19:35:31Z)")
	policyRecommendationCmd.Flags().String("end-ts", "", "End time for flows, as a RFC3339 UTC timestamp (e.g., 2022-07-01T19:35:31Z)")
	policyRecommendationCmd.Flags().String("ns-allow", "kube-system,flow-aggregator,flow-visibility", "Namespaces with no restrictions")
	policyRecommendationCmd.Flags().String("label-ignore", "pod-template-hash,controller-revision-hash,pod-template-generation", "Pod labels to be ignored when recommending NetworkPolicy")
	policyRecommendationCmd.Flags().String("cluster-uuid", "", `UUID of the cluster for which policy recommendations will be generated
If no UUID is provided, all flows will be considered during policy recommendation`)
	policyRecommendationCmd.Flags().String("database-name", "", "Snowflake database name to run policy recommendation, it can be found in the output of the onboard command")
	policyRecommendationCmd.MarkFlagRequired("database-name")
	policyRecommendationCmd.Flags().String("warehouse-name", "", "Snowflake Virtual Warehouse to use for running policy recommendation, by default we will use a temporary one")
	policyRecommendationCmd.Flags().String("udf-version", defaultPolicyRecommendationFunctionVersion, "Version of the UDF function to use")
	policyRecommendationCmd.Flags().String("wait-timeout", defaultPolicyRecommendationWaitTimeout, "Wait timeout of the recommendation job (e.g., 5m, 100s)")

}
