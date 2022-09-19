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
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/spf13/cobra"

	kmsclient "antrea.io/theia/snowflake/pkg/aws/client/kms"
)

// deleteKmsKeyCmd represents the delete-kms-Key command
var deleteKmsKeyCmd = &cobra.Command{
	Use:   "delete-kms-key",
	Short: "Delete an AWS KMS key",
	Long: `This command deletes an existing KMS key in your AWS account. For
example:

"theia-sf delete-kms-key --key-id <YOUR KEY ID>"

Note that the key will not be deleted immediately, which is not supported by the
AWS API. Instead, the deletion will be scheduled after a 30-day waiting period.

Before deleting the key, ensure that you have deleted (with "theia-sf offboard")
all the infrastructure which depends on this key.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		keyID, _ := cmd.Flags().GetString("key-id")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)

		}
		kmsClient := kmsclient.GetClient(awsCfg)
		return scheduleKeyDeletion(ctx, kmsClient, keyID)
	},
}

func scheduleKeyDeletion(ctx context.Context, kmsClient kmsclient.Interface, keyID string) error {
	waitingPeriodDays := int32(30)
	logger.Info("Scheduling key deletion", "waiting days", waitingPeriodDays)
	output, err := kmsClient.ScheduleKeyDeletion(ctx, &kms.ScheduleKeyDeletionInput{
		KeyId:               &keyID,
		PendingWindowInDays: &waitingPeriodDays,
	})
	if err != nil {
		return fmt.Errorf("error when scheduling key deletion: %w", err)
	}
	logger.Info("Scheduled key deletion", "deletion date", output.DeletionDate.String())
	return nil
}

func init() {
	rootCmd.AddCommand(deleteKmsKeyCmd)

	deleteKmsKeyCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region to use for deleting the key")
	deleteKmsKeyCmd.Flags().String("key-id", "", "KMS key ID")
	deleteKmsKeyCmd.MarkFlagRequired("key-id")
}
