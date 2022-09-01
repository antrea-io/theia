/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"antrea.io/theia/snowflake/pkg/infra"
)

// onboardCmd represents the onboard command
var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Create or update cloud resources in Snowflake and AWS",
	Long: `Create or update cloud resources in Snowflake and AWS. You need
to bring your own Snowflake and AWS accounts.

1. ensure that AWS credentials are available:
   https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials
2. export your Snowflake credentials as environment variables:
   SNOWFLAKE_ACCOUNT, SNOWFLAKE_USER, SNOWFLAKE_PASSWORD
3. choose an AWS S3 bucket which will be used to store infrastructure state;
   you can create one with "theia-sf create-bucket" if needed
4. choose an AWS KMS key which will be used to encrypt infrastructure state;
   you can create one with "theia-sf create-kms-key" if needed
5. provision infrastructure with:
   "theia-sf onboard --bucket-name <YOUR BUCKET NAME> --key-id <YOUR KMS KEY ID>"

You can run the "onboard" command multiple times as it is idempotent. When
upgrading this application to a more recent version, it is safe to run "onboard"
again with the same parameters.

You can delete all the created cloud resources at any time with:
"theia-sf offboard --bucket-name <YOUR BUCKET NAME>"

The "onboard" command requires a Snowflake warehouse to run database
migration. By default, it will create a temporary one. You can also bring your
own by using the "--warehouse-name" parameter.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		stackName, _ := cmd.Flags().GetString("stack-name")
		bucketName, _ := cmd.Flags().GetString("bucket-name")
		bucketPrefix, _ := cmd.Flags().GetString("bucket-prefix")
		bucketRegion, _ := cmd.Flags().GetString("bucket-region")
		keyID, _ := cmd.Flags().GetString("key-id")
		keyRegion, _ := cmd.Flags().GetString("key-region")
		warehouseName, _ := cmd.Flags().GetString("warehouse-name")
		workdir, _ := cmd.Flags().GetString("workdir")
		verbose := verbosity >= 2
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
		defer cancel()
		if bucketRegion == "" {
			var err error
			bucketRegion, err = GetBucketRegion(ctx, bucketName, region)
			if err != nil {
				return err
			}
		}
		stateBackendURL := infra.S3StateBackendURL(bucketName, bucketPrefix, bucketRegion)
		var secretsProviderURL string
		if keyID != "" {
			if keyRegion == "" {
				keyRegion = region
			}
			secretsProviderURL = infra.KmsSecretsProviderURL(keyID, keyRegion)
		}
		mgr := infra.NewManager(logger, stackName, stateBackendURL, secretsProviderURL, region, warehouseName, workdir, verbose)
		result, err := mgr.Onboard(ctx)
		if err != nil {
			return err
		}
		showResults(result)
		fmt.Println("SUCCESS!")
		fmt.Println("To update infrastructure, run 'theia-sf onboard' again")
		fmt.Println("To destroy all infrastructure, run 'theia-sf offboard'")
		return nil
	},
}

func showResults(result *infra.Result) {
	table := tablewriter.NewWriter(os.Stdout)
	data := [][]string{
		[]string{"Region", result.Region},
		[]string{"Bucket Name", result.BucketName},
		[]string{"Bucket Flows Folder", result.BucketFlowsFolder},
		[]string{"Snowflake Database Name", result.DatabaseName},
		[]string{"Snowflake Schema Name", result.SchemaName},
		[]string{"Snowflake Flows Table Name", result.FlowsTableName},
		[]string{"SNS Topic ARN", result.SNSTopicARN},
		[]string{"SQS Queue ARN", result.SQSQueueARN},
	}
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.AppendBulk(data)
	table.Render()
}

func init() {
	rootCmd.AddCommand(onboardCmd)

	onboardCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region where AWS resources will be provisioned")
	onboardCmd.Flags().String("stack-name", "default", "name of the infrastructure stack: useful to deploy multiple instances in the same Snowflake account or with the same bucket (e.g., one for dev and one for prod)")
	onboardCmd.Flags().String("bucket-name", "", "bucket to store infra state")
	onboardCmd.MarkFlagRequired("bucket-name")
	onboardCmd.Flags().String("bucket-prefix", "antrea-flows-infra", "prefix to use to store infra state")
	onboardCmd.Flags().String("bucket-region", "", "region where infra bucket is defined; if omitted, we will try to get the region from AWS")
	onboardCmd.Flags().String("key-id", GetEnv("THEIA_SF_KMS_KEY_ID", ""), "Kms key ID")
	onboardCmd.Flags().String("key-region", "", "Kms Key region")
	onboardCmd.Flags().String("workdir", "", "use provided local workdir (by default a temporary one will be created")
	onboardCmd.Flags().String("warehouse-name", "", "Snowflake Virtual Warehouse to use for onboarding queries, by default we will use a temporary one")
}
