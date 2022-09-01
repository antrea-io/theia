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

	schemaName           = "THEIA"
	flowRetentionDays    = 30
	flowDeletionTaskName = "DELETE_STALE_FLOWS"
	udfStageName         = "UDFS"
	ingestionStageName   = "FLOWSTAGE"
	autoIngestPipeName   = "FLOWPIPE"

	// do not change!!!
	flowsTableName = "FLOWS"

	migrationsDir = "migrations"
)
