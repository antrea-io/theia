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

package infra

const (
	projectName = "theia-infra"

	pulumiVersion                = "v3.39.3"
	pulumiAWSPluginVersion       = "v5.13.0"
	pulumiSnowflakePluginVersion = "v0.13.0"
	pulumiRandomPluginVersion    = "v4.8.2"
	pulumiCommandPluginVersion   = "v0.5.2"

	migrateSnowflakeVersion = "v0.2.0"

	flowRecordsRetentionDays = 7
	s3BucketNamePrefix       = "antrea-flows-"
	s3BucketFlowsFolder      = "flows"
	snsTopicNamePrefix       = "antrea-flows-"
	sqsQueueNamePrefix       = "antrea-flows-"
	// how long will Snowpipe error notifications be saved in SQS queue
	sqsMessageRetentionSeconds = 7 * 24 * 3600

	storageIAMRoleNamePrefix     = "antrea-sf-storage-iam-role-"
	storageIAMPolicyNamePrefix   = "antrea-sf-storage-iam-policy-"
	storageIntegrationNamePrefix = "ANTREA_FLOWS_STORAGE_INTEGRATION_"

	notificationIAMRoleNamePrefix     = "antrea-sf-notification-iam-role-"
	notificationIAMPolicyNamePrefix   = "antrea-sf-notification-iam-policy"
	notificationIntegrationNamePrefix = "ANTREA_FLOWS_NOTIFICATION_INTEGRATION_"

	databaseNamePrefix = "ANTREA_"

	SchemaName           = "THEIA"
	flowRetentionDays    = 30
	flowDeletionTaskName = "DELETE_STALE_FLOWS"
	udfStageName         = "UDFS"
	ingestionStageName   = "FLOWSTAGE"
	autoIngestPipeName   = "FLOWPIPE"

	// do not change!!!
	flowsTableName = "FLOWS"

	migrationsDir = "migrations"
	udfsDir       = "udfs"

	udfVersionPlaceholder        = "%VERSION%"
	udfCreateFunctionSQLFilename = "create_function.sql"
	k8sPythonClientUrl           = "https://downloads.antrea.io/artifacts/snowflake-udf/k8s-client-python-v24.2.0.zip"
	k8sPythonClientFileName      = "kubernetes.zip"
)
