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

// createKmsKeyCmd represents the create-kms-key command
var createKmsKeyCmd = &cobra.Command{
	Use:   "create-kms-key",
	Short: "Create an AWS KMS key",
	Long: `This command creates a new KMS key in your AWS account. This key
can be used to encrypt infrastructure state in the backend (S3 bucket). If you
already have a KMS key that you want to use, you won't need this command.

Before calling this command, ensure that AWS credentials are available:
https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials

To create a KMS key and obtain its ID:
"theia-sf create-kms-key"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)

		}
		kmsClient := kmsclient.GetClient(awsCfg)
		keyID, err := createKey(ctx, kmsClient)
		if err != nil {
			return err
		}
		fmt.Printf("Key ID: %s\n", keyID)
		return nil
	},
}

func createKey(ctx context.Context, kmsClient kmsclient.Interface) (string, error) {
	logger.Info("Creating key")
	description := "This key was created by theia-sf; it is used to encrypt infrastructure state"
	output, err := kmsClient.CreateKey(ctx, &kms.CreateKeyInput{
		Description: &description,
		// we use default parameters for everything else
	})
	if err != nil {
		return "", fmt.Errorf("error when creating key: %w", err)
	}
	logger.Info("Created key")
	return *output.KeyMetadata.KeyId, nil
}

func init() {
	rootCmd.AddCommand(createKmsKeyCmd)

	createKmsKeyCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region where key should be created")
}
