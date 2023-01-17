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
	dropDetectionFunctionName           = "drop_detection"
	defaultDropDetectionFunctionVersion = "v0.1.0"
	defaultDropDetectionWaitTimeout     = "5m"
)

func buildDropDetectionUdfQuery(
	jobType string,
	start string,
	end string,
	startTs string,
	endTs string,
	clusterUUID string,
	databaseName string,
	functionVersion string,
) (string, error) {
	now := time.Now()
	detectionID := uuid.New().String()

	var queryBuilder strings.Builder

	queryBuilder.WriteString(`WITH filtered_flows AS (
SELECT
	sourceIP,
	sourcePodName,
	sourcePodNamespace,
	destinationIP,
	destinationPodName,
	destinationPodNamespace,
	to_date(flowStartSeconds) as flowStartDate,
	ingressNetworkPolicyRuleAction,
	egressNetworkPolicyRuleAction,
	count(*) as flowNumber
FROM
	flows
WHERE
	ingressNetworkPolicyRuleAction = 2
	OR
	ingressNetworkPolicyRuleAction = 3
	OR
	egressNetworkPolicyRuleAction = 2
	OR
	egressNetworkPolicyRuleAction = 3
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
		logger.Info("No clusterUUID input, all flows will be considered during abnormal traffic drop detection.")
	}

	queryBuilder.WriteString(`GROUP BY
	sourceIP,
	sourcePodName,
	sourcePodNamespace,
	destinationIP,
	destinationPodName,
	destinationPodNamespace,
	flowStartDate,
	ingressNetworkPolicyRuleAction,
	egressNetworkPolicyRuleAction
), processed_flows AS (
SELECT
CASE WHEN f.ingressNetworkPolicyRuleAction = 2 OR f.ingressNetworkPolicyRuleAction = 3
	THEN
	(CASE WHEN f.destinationPodName IS NOT NULL
		THEN CONCAT(f.destinationPodNamespace, '/', f.destinationPodName)
		ELSE f.destinationIP
	END)
ELSE
	(CASE WHEN f.sourcePodName IS NOT NULL
		THEN CONCAT(f.sourcePodNamespace, '/', f.sourcePodName)
		ELSE f.sourceIP
	END)
END as endpoint,
CASE WHEN f.ingressNetworkPolicyRuleAction = 2 OR f.ingressNetworkPolicyRuleAction = 3
	THEN
	'ingress'
ELSE
	'egress'
END as direction,
f.flowStartDate as date,
f.flowNumber as dropNumber
FROM filtered_flows AS f
`)

	// Aggregate traffic drop number for each endpoint and direction
	queryBuilder.WriteString(`), aggregated_flows as (
SELECT 
	pf.endpoint,
	pf.direction,
	pf.date, 
	SUM(pf.dropNumber) as dropNumber
FROM processed_flows AS pf
GROUP BY
	pf.endpoint,
	pf.direction,
	pf.date
`)

	// Choose the endpoint + direction as the partition field for the dropDetection UDTF
	// because traffic drop detection is run on ingress and egress direction for each endpoint.
	functionName := udfs.GetFunctionName(dropDetectionFunctionName, functionVersion)
	fmt.Fprintf(&queryBuilder, `)
SELECT 
	r.job_type, 
	r.detection_id, 
	r.time_created, 
	r.endpoint,
	r.direction,
	r.avg_drop,
	r.stdev_drop,
	r.anomaly_drop_date,
	r.anomaly_drop_number FROM aggregated_flows AS af,
TABLE(%s(
	'%s',
	'%s',
	af.endpoint,
	af.direction,
	af.date, 
	af.dropNumber
) over (partition by af.endpoint, af.direction)) as r
`, functionName, jobType, detectionID)

	return queryBuilder.String(), nil
}

// dropDetectionCmd represents the drop-detection command
var dropDetectionCmd = &cobra.Command{
	Use:   "drop-detection",
	Short: "Run the abnormal traffic drop detection UDF in Snowflake",
	Long: `This command runs the abnormal traffic drop detection UDF in
Snowflake. You need to bring your own Snowflake account and run the onboard
command first.

Run abnormal traffic drop detection with default configuration on database ANTREA_C9JR8KUKUIV4R72S:
"theia-sf drop-detection --database-name ANTREA_C9JR8KUKUIV4R72S"

The "drop-detection" command requires a Snowflake warehouse to run the abnormal
traffic drop detection UDF in Snowflake. By default, it will create a temporary
one.
You can also bring your own by using the "--warehouse-name" parameter.
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jobType, _ := cmd.Flags().GetString("type")
		if jobType != "initial" {
			return fmt.Errorf("invalid --type argument")
		}

		start, _ := cmd.Flags().GetString("start")
		end, _ := cmd.Flags().GetString("end")
		startTs, _ := cmd.Flags().GetString("start-ts")
		endTs, _ := cmd.Flags().GetString("end-ts")
		clusterUUID, _ := cmd.Flags().GetString("cluster-uuid")
		databaseName, _ := cmd.Flags().GetString("database-name")
		warehouseName, _ := cmd.Flags().GetString("warehouse-name")
		functionVersion, _ := cmd.Flags().GetString("udf-version")
		waitTimeout, _ := cmd.Flags().GetString("wait-timeout")
		waitDuration, err := time.ParseDuration(waitTimeout)
		if err != nil {
			return fmt.Errorf("invalid --wait-timeout argument, err when parsing it as a duration: %v", err)
		}

		query, err := buildDropDetectionUdfQuery(jobType, start, end, startTs, endTs, clusterUUID, databaseName, functionVersion)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), waitDuration)
		defer cancel()
		rows, err := udfs.RunUdf(ctx, logger, query, databaseName, warehouseName)
		if err != nil {
			return fmt.Errorf("error when running traffic drop UDF: %w", err)
		}
		defer rows.Close()

		var detectionID string
		var timeCreated string
		var endpoint string
		var direction string
		var avgDrop float32
		var stdevDrop float32
		var anomalyDropDate string
		var anomalyDropNumber float32
		for rows.Next() {
			if err := rows.Scan(
				&jobType,
				&detectionID,
				&timeCreated,
				&endpoint,
				&direction,
				&avgDrop,
				&stdevDrop,
				&anomalyDropDate,
				&anomalyDropNumber,
			); err != nil {
				return fmt.Errorf("invalid row: %w", err)
			}
			fmt.Printf("endpoint: %s, direction: %s, avgDrop: %f, stdevDrop: %f, anomalyDropDate: %s, anomalyDropNumber: %f\n",
				endpoint,
				direction,
				avgDrop,
				stdevDrop,
				anomalyDropDate[:len(anomalyDropDate)-10],
				anomalyDropNumber)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dropDetectionCmd)

	dropDetectionCmd.Flags().String("type", "initial", "Type of abnormal traffic drop detection job (initial|periodical), we only support initial jobType for now")
	dropDetectionCmd.Flags().String("start", "", "Start time for flows, with reference to the current time (e.g., now-1h)")
	dropDetectionCmd.Flags().String("end", "", "End time for flows, with reference to the current timr (e.g., now)")
	dropDetectionCmd.Flags().String("start-ts", "", "Start time for flows, as a RFC3339 UTC timestamp (e.g., 2022-07-01T19:35:31Z)")
	dropDetectionCmd.Flags().String("end-ts", "", "End time for flows, as a RFC3339 UTC timestamp (e.g., 2022-07-01T19:35:31Z)")
	dropDetectionCmd.Flags().String("cluster-uuid", "", `UUID of the cluster for which abnormal traffic drop will be detected
If no UUID is provided, all flows will be considered during abnormal traffic drop detection`)
	dropDetectionCmd.Flags().String("database-name", "", "Snowflake database name to run abnormal traffic drop detection, it can be found in the output of the onboard command")
	dropDetectionCmd.MarkFlagRequired("database-name")
	dropDetectionCmd.Flags().String("warehouse-name", "", "Snowflake Virtual Warehouse to run abnormal traffic drop detection, by default we will use a temporary one")
	dropDetectionCmd.Flags().String("udf-version", defaultDropDetectionFunctionVersion, "Version of the UDF function to use")
	dropDetectionCmd.Flags().String("wait-timeout", defaultDropDetectionWaitTimeout, "Wait timeout of the abnormal traffic drop detection job (e.g., 5m, 100s)")
}
