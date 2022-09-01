/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"antrea.io/theia/snowflake/pkg/infra"
)

// offboardCmd represents the offboard command
var offboardCmd = &cobra.Command{
	Use:   "offboard",
	Short: "Delete cloud resources in Snowflake and AWS",
	Long: `Delete all the resources created by the "onboard" command. Note
that all resources will be deleted, including the Snowflake database storing the
Antrea flows. If needed, you can copy the database contents yourself before
running this command. This command has the same prerequisites as the "onboard"
command when it comes to AWS and Snowflake credentials. It is important to call
this command with the same parameters as when "onboard" was called:
"--bucket-name" and, if applicable, "region", "--bucket-prefix",
"--bucket-region", and "--stack-name".

For example:
"theia-sf offboard --bucket-name <YOUR BUCKET NAME>"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		stackName, _ := cmd.Flags().GetString("stack-name")
		bucketName, _ := cmd.Flags().GetString("bucket-name")
		bucketPrefix, _ := cmd.Flags().GetString("bucket-prefix")
		bucketRegion, _ := cmd.Flags().GetString("bucket-region")
		keyID, _ := cmd.Flags().GetString("key-id")
		keyRegion, _ := cmd.Flags().GetString("key-region")
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
		mgr := infra.NewManager(logger, stackName, stateBackendURL, secretsProviderURL, region, "", workdir, verbose)
		if err := mgr.Offboard(ctx); err != nil {
			return err
		}
		fmt.Println("SUCCESS!")
		fmt.Println("To re-create infrastructure, run 'theia-sf onboard' again")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(offboardCmd)

	offboardCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region where bucket should be created")
	offboardCmd.Flags().String("stack-name", "default", "Name of the infrastructure stack: useful to deploy multiple instances in the same Snowflake account or with the same bucket (e.g., one for dev and one for prod)")
	offboardCmd.Flags().String("bucket-name", "", "bucket to store infra state")
	offboardCmd.MarkFlagRequired("bucket-name")
	offboardCmd.Flags().String("bucket-prefix", "antrea-flows-infra", "prefix to use to store infra state")
	offboardCmd.Flags().String("bucket-region", "", "region where infra bucket is defined; if omitted, we will try to get the region from AWS")
	offboardCmd.Flags().String("key-id", GetEnv("THEIA_SF_KMS_KEY_ID", ""), "Kms Key ID")
	offboardCmd.Flags().String("key-region", "", "Kms Key region")
	offboardCmd.Flags().String("workdir", "", "use provided local workdir (by default a temporary one will be created")
}
