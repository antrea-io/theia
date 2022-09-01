/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/dustinkirkland/golang-petname"
	"github.com/spf13/cobra"

	s3client "antrea.io/theia/snowflake/pkg/aws/client/s3"
)

// createBucketCmd represents the create-bucket command
var createBucketCmd = &cobra.Command{
	Use:   "create-bucket",
	Short: "Create an AWS S3 bucket",
	Long: `This command creates a new S3 bucket in your AWS account. This is
useful if you don't already have a bucket available to store infrastructure
state (such a bucket is required for the "onboard" command).

Before calling this command, ensure that AWS credentials are available:
https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials

To create a bucket with a specific name:
"theia-sf create-bucket --name this-is-the-name-i-want"

To create a bucket with a random name in a specific non-default region:
"theia-sf create-bucket --region us-east-2"`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		bucketName, _ := cmd.Flags().GetString("name")
		bucketPrefix, _ := cmd.Flags().GetString("prefix")
		if bucketName == "" {
			suffix := petname.Generate(4, "-")
			bucketName = fmt.Sprintf("%s-%s", bucketPrefix, suffix)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)

		}
		s3Client := s3client.GetClient(awsCfg)
		if err := createBucket(ctx, s3Client, bucketName, region); err != nil {
			return err
		}
		fmt.Printf("Bucket name: %s\n", bucketName)
		return nil
	},
}

func createBucket(ctx context.Context, s3Client s3client.Interface, name string, region string) error {
	logger := logger.WithValues("bucket", name, "region", region)
	logger.Info("Checking if S3 bucket exists")
	_, err := s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: &name,
	})
	var notFoundError *s3types.NotFound
	if err != nil && !errors.As(err, &notFoundError) {
		return fmt.Errorf("error when looking for bucket '%s': %w", name, err)
	}
	if notFoundError == nil {
		logger.Info("S3 bucket already exists")
		return nil
	}
	logger.Info("Creating S3 bucket")
	if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &name,
		ACL:    s3types.BucketCannedACLPrivate,
		CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		},
	}); err != nil {
		return fmt.Errorf("error when creating bucket '%s': %w", name, err)
	}
	logger.Info("Created S3 bucket")
	return nil
}

func init() {
	rootCmd.AddCommand(createBucketCmd)

	createBucketCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region where bucket should be created")
	createBucketCmd.Flags().String("name", "", "name of bucket to create")
	createBucketCmd.Flags().String("prefix", "antrea", "prefix to use for bucket name (with auto-generated suffix)")
	createBucketCmd.MarkFlagsMutuallyExclusive("name", "prefix")
}
