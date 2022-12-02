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

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/s3"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/sns"
	"github.com/pulumi/pulumi-aws/sdk/v5/go/aws/sqs"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi-snowflake/sdk/go/snowflake"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func declareSnowflakeIngestion(randomString *random.RandomString, bucket *s3.BucketV2, accountID string) func(ctx *pulumi.Context) (*snowflake.StorageIntegration, *iam.Role, error) {
	declareFunc := func(ctx *pulumi.Context) (*snowflake.StorageIntegration, *iam.Role, error) {
		storageIntegrationName := randomString.Result.ApplyT(func(suffix string) string {
			return fmt.Sprintf("%s%s", storageIntegrationNamePrefix, strings.ToUpper(suffix))
		}).(pulumi.StringOutput)
		storageIntegration, err := snowflake.NewStorageIntegration(ctx, "antrea-sf-storage-integration", &snowflake.StorageIntegrationArgs{
			Name:                    storageIntegrationName,
			Type:                    pulumi.String("EXTERNAL_STAGE"),
			Enabled:                 pulumi.Bool(true),
			StorageAllowedLocations: pulumi.StringArray([]pulumi.StringInput{pulumi.Sprintf("s3://%s/%s/", bucket.ID(), s3BucketFlowsFolder)}),
			StorageProvider:         pulumi.String("S3"),
			StorageAwsRoleArn:       pulumi.Sprintf("arn:aws:iam::%s:role/%s%s", accountID, storageIAMRoleNamePrefix, randomString.ID()),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, nil, err
		}
		ctx.Export("storageIntegrationName", storageIntegration.Name)

		storagePolicyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				iam.GetPolicyDocumentStatementArgs{
					Sid:       pulumi.String("1"),
					Effect:    pulumi.String("Allow"),
					Actions:   pulumi.ToStringArray([]string{"s3:GetObject", "s3:GetObjectVersion"}),
					Resources: pulumi.StringArray([]pulumi.StringInput{pulumi.Sprintf("arn:aws:s3:::%s/%s/*", bucket.ID(), s3BucketFlowsFolder)}),
				},
				iam.GetPolicyDocumentStatementArgs{
					Sid:       pulumi.String("2"),
					Effect:    pulumi.String("Allow"),
					Actions:   pulumi.ToStringArray([]string{"s3:ListBucket"}),
					Resources: pulumi.StringArray([]pulumi.StringInput{pulumi.Sprintf("arn:aws:s3:::%s", bucket.ID())}),
					Conditions: iam.GetPolicyDocumentStatementConditionArray{
						iam.GetPolicyDocumentStatementConditionArgs{
							Test:     pulumi.String("StringLike"),
							Variable: pulumi.String("s3:prefix"),
							Values:   pulumi.StringArray([]pulumi.StringInput{pulumi.Sprintf("%s/*", s3BucketFlowsFolder)}),
						},
					},
				},
				iam.GetPolicyDocumentStatementArgs{
					Sid:       pulumi.String("3"),
					Effect:    pulumi.String("Allow"),
					Actions:   pulumi.ToStringArray([]string{"s3:GetBucketLocation"}),
					Resources: pulumi.StringArray([]pulumi.StringInput{pulumi.Sprintf("arn:aws:s3:::%s", bucket.ID())}),
				},
			},
		})

		// For some reason pulumi reports a diff on every refresh, when in fact the resource
		// does not need to be updated and is not actually updated during the "up" stage.
		// See https://github.com/pulumi/pulumi-aws/issues/2024
		storageIAMPolicy, err := iam.NewPolicy(ctx, "antrea-sf-storage-iam-policy", &iam.PolicyArgs{
			Name:   pulumi.Sprintf("%s%s", storageIAMPolicyNamePrefix, randomString.ID()),
			Policy: storagePolicyDocument.Json(),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, nil, err
		}

		storageAssumeRolePolicyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				iam.GetPolicyDocumentStatementArgs{
					Sid:     pulumi.String("1"),
					Actions: pulumi.ToStringArray([]string{"sts:AssumeRole"}),
					Principals: iam.GetPolicyDocumentStatementPrincipalArray{
						iam.GetPolicyDocumentStatementPrincipalArgs{
							Type:        pulumi.String("AWS"),
							Identifiers: pulumi.StringArray([]pulumi.StringInput{storageIntegration.StorageAwsIamUserArn}),
						},
					},
					Conditions: iam.GetPolicyDocumentStatementConditionArray{
						iam.GetPolicyDocumentStatementConditionArgs{
							Test:     pulumi.String("StringEquals"),
							Variable: pulumi.String("sts:ExternalId"),
							Values:   pulumi.StringArray([]pulumi.StringInput{storageIntegration.StorageAwsExternalId}),
						},
					},
				},
			},
		})

		storageIAMRole, err := iam.NewRole(ctx, "antrea-sf-storage-iam-policy", &iam.RoleArgs{
			Name:             pulumi.Sprintf("%s%s", storageIAMRoleNamePrefix, randomString.ID()),
			AssumeRolePolicy: storageAssumeRolePolicyDocument.Json(),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, nil, err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "antrea-sf-storage-iam-role-policy-attachment", &iam.RolePolicyAttachmentArgs{
			Role:      storageIAMRole.ID(),
			PolicyArn: storageIAMPolicy.Arn,
		})
		if err != nil {
			return nil, nil, err
		}

		return storageIntegration, storageIAMRole, nil
	}
	return declareFunc
}

func declareSnowflakeErrorNotifications(randomString *random.RandomString, snsTopic *sns.Topic, accountID string) func(ctx *pulumi.Context) (*snowflake.NotificationIntegration, *iam.Role, error) {
	declareFunc := func(ctx *pulumi.Context) (*snowflake.NotificationIntegration, *iam.Role, error) {
		notificationIntegrationName := randomString.Result.ApplyT(func(suffix string) string {
			return fmt.Sprintf("%s%s", notificationIntegrationNamePrefix, strings.ToUpper(suffix))
		}).(pulumi.StringOutput)
		notificationIntegration, err := snowflake.NewNotificationIntegration(ctx, "antrea-sf-notification-integration", &snowflake.NotificationIntegrationArgs{
			Name:                 notificationIntegrationName,
			AwsSnsRoleArn:        pulumi.Sprintf("arn:aws:iam::%s:role/%s%s", accountID, notificationIAMRoleNamePrefix, randomString.ID()),
			AwsSnsTopicArn:       snsTopic.ID(),
			Direction:            pulumi.String("OUTBOUND"),
			Enabled:              pulumi.Bool(true),
			NotificationProvider: pulumi.String("AWS_SNS"),
			Type:                 pulumi.String("QUEUE"),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, nil, err
		}
		ctx.Export("notificationIntegrationName", notificationIntegration.Name)

		notificationPolicyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				iam.GetPolicyDocumentStatementArgs{
					Sid:       pulumi.String("1"),
					Effect:    pulumi.String("Allow"),
					Actions:   pulumi.ToStringArray([]string{"sns:Publish"}),
					Resources: pulumi.StringArray([]pulumi.StringInput{snsTopic.ID()}),
				},
			},
		})

		notificationIAMPolicy, err := iam.NewPolicy(ctx, "antrea-sf-notification-iam-policy", &iam.PolicyArgs{
			Name:   pulumi.Sprintf("%s%s", notificationIAMPolicyNamePrefix, randomString.ID()),
			Policy: notificationPolicyDocument.Json(),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, nil, err
		}

		notificationAssumeRolePolicyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				iam.GetPolicyDocumentStatementArgs{
					Sid:     pulumi.String("1"),
					Actions: pulumi.ToStringArray([]string{"sts:AssumeRole"}),
					Principals: iam.GetPolicyDocumentStatementPrincipalArray{
						iam.GetPolicyDocumentStatementPrincipalArgs{
							Type:        pulumi.String("AWS"),
							Identifiers: pulumi.StringArray([]pulumi.StringInput{notificationIntegration.AwsSnsIamUserArn}),
						},
					},
					Conditions: iam.GetPolicyDocumentStatementConditionArray{
						iam.GetPolicyDocumentStatementConditionArgs{
							Test:     pulumi.String("StringEquals"),
							Variable: pulumi.String("sts:ExternalId"),
							Values:   pulumi.StringArray([]pulumi.StringInput{notificationIntegration.AwsSnsExternalId}),
						},
					},
				},
			},
		})

		notificationIAMRole, err := iam.NewRole(ctx, "antrea-sf-notification-iam-policy", &iam.RoleArgs{
			Name:             pulumi.Sprintf("%s%s", notificationIAMRoleNamePrefix, randomString.ID()),
			AssumeRolePolicy: notificationAssumeRolePolicyDocument.Json(),
		})
		if err != nil {
			return nil, nil, err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "antrea-sf-notification-iam-role-policy-attachment", &iam.RolePolicyAttachmentArgs{
			Role:      notificationIAMRole.ID(),
			PolicyArn: notificationIAMPolicy.Arn,
		})
		if err != nil {
			return nil, nil, err
		}

		return notificationIntegration, notificationIAMRole, nil
	}
	return declareFunc
}

func declareSnowflakeDatabase(
	warehouseName string,
	randomString *random.RandomString,
	bucket *s3.BucketV2,
	storageIntegration *snowflake.StorageIntegration,
	storageIAMRole *iam.Role,
	notificationIntegration *snowflake.NotificationIntegration,
	notificationIAMRole *iam.Role,
) func(ctx *pulumi.Context) (*snowflake.Pipe, error) {
	declareFunc := func(ctx *pulumi.Context) (*snowflake.Pipe, error) {
		databaseName := randomString.Result.ApplyT(func(suffix string) string {
			return fmt.Sprintf("%s%s", databaseNamePrefix, strings.ToUpper(suffix))
		}).(pulumi.StringOutput)
		db, err := snowflake.NewDatabase(ctx, "antrea-sf-db", &snowflake.DatabaseArgs{
			Name: databaseName,
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, err
		}
		ctx.Export("databaseName", db.Name)

		schema, err := snowflake.NewSchema(ctx, "antrea-sf-schema", &snowflake.SchemaArgs{
			Database: db.ID(),
			Name:     pulumi.String(SchemaName),
		}, pulumi.Parent(db), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, err
		}

		_, err = snowflake.NewStage(ctx, "antrea-sf-udf-stage", &snowflake.StageArgs{
			Database: db.ID(),
			Schema:   schema.Name,
			Name:     pulumi.String(udfStageName),
		}, pulumi.Parent(schema), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, err
		}

		// IAMRoles need to be added as dependencies, but integrations are probably redundant
		dependencies := []pulumi.Resource{storageIntegration, storageIAMRole, notificationIntegration, notificationIAMRole}
		ingestionStage, err := snowflake.NewStage(ctx, "antrea-sf-ingestion-stage", &snowflake.StageArgs{
			Database:           db.ID(),
			Schema:             schema.Name,
			Name:               pulumi.String(ingestionStageName),
			Url:                pulumi.Sprintf("s3://%s/%s/", bucket.ID(), s3BucketFlowsFolder),
			StorageIntegration: storageIntegration.ID(),
		}, pulumi.Parent(schema), pulumi.DependsOn(dependencies), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, err
		}

		// ideally this would be a dynamic provider: https://www.pulumi.com/docs/intro/concepts/resources/dynamic-providers/
		// however, dynamic providers are not supported at the moment for Golang
		// maybe this is a change that we can make at a future time if support is added
		dbMigrations, err := local.NewCommand(ctx, "antrea-sf-run-db-migrations", &local.CommandArgs{
			// we will ignore changes to create because the warehouse name can change
			Create: pulumi.Sprintf("./migrate-snowflake -source file://%s -database %s -schema %s -warehouse %s", migrationsDir, db.Name, schema.Name, warehouseName),
			Environment: pulumi.StringMap{
				"SNOWFLAKE_ACCOUNT":  pulumi.ToSecret(os.Getenv("SNOWFLAKE_ACCOUNT")).(pulumi.StringOutput),
				"SNOWFLAKE_USER":     pulumi.ToSecret(os.Getenv("SNOWFLAKE_USER")).(pulumi.StringOutput),
				"SNOWFLAKE_PASSWORD": pulumi.ToSecret(os.Getenv("SNOWFLAKE_PASSWORD")).(pulumi.StringOutput),
			},
			// migrations are idempotent so for now we always run them
			Triggers: pulumi.Array([]pulumi.Input{db.ID(), schema.ID(), pulumi.Int(rand.Int())}),
		}, pulumi.Parent(schema), pulumi.DeleteBeforeReplace(true), pulumi.ReplaceOnChanges([]string{"environment"}), pulumi.IgnoreChanges([]string{"create"}))
		if err != nil {
			return nil, err
		}

		// a bit of defensive programming: explicit dependency on ingestionStage may not be strictly required
		pipe, err := snowflake.NewPipe(ctx, "antrea-sf-auto-ingest-pipe", &snowflake.PipeArgs{
			Database:         db.ID(),
			Schema:           schema.Name,
			Name:             pulumi.String(autoIngestPipeName),
			AutoIngest:       pulumi.Bool(true),
			ErrorIntegration: notificationIntegration.ID(),
			// FQN required for table and stage, see https://github.com/pulumi/pulumi-snowflake/issues/129
			// 0x27 is the hex representation of single quote. We use it to enclose Pod labels string.
			CopyStatement: pulumi.Sprintf("COPY INTO %s.%s.%s FROM @%s.%s.%s FILE_FORMAT = (TYPE = CSV FIELD_OPTIONALLY_ENCLOSED_BY='0x27')", databaseName, SchemaName, flowsTableName, databaseName, SchemaName, ingestionStageName),
		}, pulumi.Parent(schema), pulumi.DependsOn([]pulumi.Resource{ingestionStage, dbMigrations}), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return nil, err
		}

		if flowRetentionDays > 0 {
			_, err := snowflake.NewTask(ctx, "antrea-sf-flow-deletion-task", &snowflake.TaskArgs{
				Database:                            db.ID(),
				Schema:                              schema.Name,
				Name:                                pulumi.String(flowDeletionTaskName),
				Schedule:                            pulumi.String("USING CRON 0 0 * * * UTC"),
				SqlStatement:                        pulumi.Sprintf("DELETE FROM %s WHERE DATEDIFF(day, timeInserted, CURRENT_TIMESTAMP) > %d", flowsTableName, flowRetentionDays),
				UserTaskManagedInitialWarehouseSize: pulumi.String("XSMALL"),
				Enabled:                             pulumi.Bool(true),
			}, pulumi.Parent(schema), pulumi.DependsOn([]pulumi.Resource{dbMigrations}), pulumi.DeleteBeforeReplace(true))
			if err != nil {
				return nil, err
			}
		}

		return pipe, nil
	}
	return declareFunc
}

func declareStack(warehouseName string) func(ctx *pulumi.Context) error {
	declareFunc := func(ctx *pulumi.Context) error {
		randomString, err := random.NewRandomString(ctx, "antrea-flows-random-pet-suffix", &random.RandomStringArgs{
			Length:  pulumi.Int(16),
			Lower:   pulumi.Bool(true),
			Upper:   pulumi.Bool(false),
			Number:  pulumi.Bool(true),
			Special: pulumi.Bool(false),
		})
		// when disabling auto-naming, it's safer to use DeleteBeforeReplace
		// see https://www.pulumi.com/docs/intro/concepts/resources/names/#autonaming
		bucket, err := s3.NewBucketV2(ctx, "antrea-flows-bucket", &s3.BucketV2Args{
			Bucket:       pulumi.Sprintf("%s%s", s3BucketNamePrefix, randomString.ID()),
			ForceDestroy: pulumi.Bool(true), // bucket will be deleted even if not empty
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		ctx.Export("bucketID", bucket.ID())

		_, err = s3.NewBucketLifecycleConfigurationV2(ctx, "antrea-flows-bucket-lifecycle-configuration", &s3.BucketLifecycleConfigurationV2Args{
			Bucket: bucket.ID(),
			Rules: s3.BucketLifecycleConfigurationV2RuleArray{
				&s3.BucketLifecycleConfigurationV2RuleArgs{
					Expiration: &s3.BucketLifecycleConfigurationV2RuleExpirationArgs{
						Days: pulumi.Int(flowRecordsRetentionDays),
					},
					Filter: &s3.BucketLifecycleConfigurationV2RuleFilterArgs{
						Prefix: pulumi.Sprintf("%s/", pulumi.String(s3BucketFlowsFolder)),
					},
					Id:     pulumi.String(s3BucketFlowsFolder),
					Status: pulumi.String("Enabled"),
				},
			},
		})
		if err != nil {
			return err
		}

		sqsQueue, err := sqs.NewQueue(ctx, "antrea-flows-sqs-queue", &sqs.QueueArgs{
			Name:                    pulumi.Sprintf("%s%s", sqsQueueNamePrefix, randomString.ID()),
			MessageRetentionSeconds: pulumi.Int(sqsMessageRetentionSeconds),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		ctx.Export("sqsQueueARN", sqsQueue.Arn)

		snsTopic, err := sns.NewTopic(ctx, "antrea-flows-sns-topic", &sns.TopicArgs{
			Name: pulumi.Sprintf("%s%s", snsTopicNamePrefix, randomString.ID()),
		}, pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}
		ctx.Export("snsTopicARN", snsTopic.Arn)

		sqsQueuePolicyDocument := iam.GetPolicyDocumentOutput(ctx, iam.GetPolicyDocumentOutputArgs{
			Statements: iam.GetPolicyDocumentStatementArray{
				iam.GetPolicyDocumentStatementArgs{
					Sid:    pulumi.String("1"),
					Effect: pulumi.String("Allow"),
					Principals: iam.GetPolicyDocumentStatementPrincipalArray{
						iam.GetPolicyDocumentStatementPrincipalArgs{
							Type:        pulumi.String("Service"),
							Identifiers: pulumi.ToStringArray([]string{"sns.amazonaws.com"}),
						},
					},
					Actions:   pulumi.ToStringArray([]string{"sqs:SendMessage"}),
					Resources: pulumi.StringArray([]pulumi.StringInput{sqsQueue.Arn}),
					Conditions: iam.GetPolicyDocumentStatementConditionArray{
						iam.GetPolicyDocumentStatementConditionArgs{
							Test:     pulumi.String("ArnEquals"),
							Variable: pulumi.String("aws:SourceArn"),
							Values:   pulumi.StringArray([]pulumi.StringInput{snsTopic.Arn}),
						},
					},
				},
			},
		})

		_, err = sqs.NewQueuePolicy(ctx, "antrea-flows-sqs-queue-policy", &sqs.QueuePolicyArgs{
			QueueUrl: sqsQueue.ID(),
			Policy:   sqsQueuePolicyDocument.Json(),
		})
		if err != nil {
			return err
		}

		_, err = sns.NewTopicSubscription(ctx, "antrea-flows-sns-subscription", &sns.TopicSubscriptionArgs{
			Endpoint: sqsQueue.Arn,
			Protocol: pulumi.String("sqs"),
			Topic:    snsTopic.Arn,
		})
		if err != nil {
			return err
		}

		current, err := aws.GetCallerIdentity(ctx, nil, nil)
		if err != nil {
			return err
		}

		storageIntegration, storageIAMRole, err := declareSnowflakeIngestion(randomString, bucket, current.AccountId)(ctx)
		if err != nil {
			return err
		}

		notificationIntegration, notificationIAMRole, err := declareSnowflakeErrorNotifications(randomString, snsTopic, current.AccountId)(ctx)
		if err != nil {
			return err
		}

		pipe, err := declareSnowflakeDatabase(warehouseName, randomString, bucket, storageIntegration, storageIAMRole, notificationIntegration, notificationIAMRole)(ctx)
		if err != nil {
			return err
		}

		_, err = s3.NewBucketNotification(ctx, "antrea-flows-bucket-notification", &s3.BucketNotificationArgs{
			Bucket: bucket.ID(),
			Queues: s3.BucketNotificationQueueArray{
				&s3.BucketNotificationQueueArgs{
					QueueArn: pipe.NotificationChannel,
					Events: pulumi.StringArray{
						pulumi.String("s3:ObjectCreated:*"),
					},
					FilterPrefix: pulumi.Sprintf("%s/", s3BucketFlowsFolder),
					Id:           pulumi.String("Auto-ingest Snowflake"),
				},
			},
		})
		if err != nil {
			return err
		}

		return nil
	}
	return declareFunc
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
