/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"

	s3client "antrea.io/theia/snowflake/pkg/aws/client/s3"
)

// deleteBucketCmd represents the delete-bucket command
var deleteBucketCmd = &cobra.Command{
	Use:   "delete-bucket",
	Short: "Delete an AWS S3 bucket",
	Long: `This command deletes an existing S3 bucket in your AWS
account. For example:

"theia-sf delete-bucket --name <YOUR BUCKET NAME>"

If the bucket is not empty, you will need to provide the "--force" flag, which
will cause all objects in the bucket to be deleted.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		bucketName, _ := cmd.Flags().GetString("name")
		force, _ := cmd.Flags().GetBool("force")
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)

		}
		s3Client := s3client.GetClient(awsCfg)
		if force {
			if err := deleteS3Objects(ctx, s3Client, bucketName); err != nil {
				return err
			}
		}
		return deleteBucket(ctx, s3Client, bucketName)
	},
}

func deleteBucket(ctx context.Context, s3Client s3client.Interface, name string) error {
	logger := logger.WithValues("bucket", name)
	logger.Info("Deleting S3 bucket")
	if _, err := s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: &name,
	}); err != nil {
		return fmt.Errorf("error when deleting bucket '%s': %w", name, err)
	}
	logger.Info("Deleted S3 bucket")
	return nil
}

func deleteS3Objects(ctx context.Context, s3Client s3client.Interface, bucketName string) error {
	logger := logger.WithValues("bucket", bucketName)
	prefix := ""
	var nextToken *string
	logger.Info("Deleting all objects in S3 bucket")
	for {
		output, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            &bucketName,
			ContinuationToken: nextToken,
			Prefix:            &prefix,
		})
		if err != nil {
			return fmt.Errorf("error when listing objects: %w", err)
		}
		keys := make([]s3types.ObjectIdentifier, 0, len(output.Contents))
		for _, obj := range output.Contents {
			keys = append(keys, s3types.ObjectIdentifier{
				Key: obj.Key,
			})
		}
		if len(keys) == 0 {
			break
		}
		logger.Info("Deleting objects", "count", len(keys))
		_, err = s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &bucketName,
			Delete: &s3types.Delete{
				Objects: keys,
			},
		})
		if err != nil {
			return fmt.Errorf("error when deleting objects: %w", err)
		}

		nextToken = output.ContinuationToken
		if nextToken == nil {
			break
		}
	}
	logger.Info("Deleted all objects in S3 bucket")
	return nil
}

func init() {
	rootCmd.AddCommand(deleteBucketCmd)

	deleteBucketCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region to use for deleting the bucket")
	deleteBucketCmd.Flags().String("name", "", "name of bucket to delete")
	deleteBucketCmd.MarkFlagRequired("name")
	deleteBucketCmd.Flags().Bool("force", false, "delete all objects if bucket is not empty")
}
