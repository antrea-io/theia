package cmd

import (
	"context"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	s3client "antrea.io/theia/snowflake/pkg/aws/client/s3"
)

func GetEnv(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func GetBucketRegion(ctx context.Context, bucket string, regionHint string) (string, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(regionHint))
	if err != nil {
		return "", fmt.Errorf("unable to load AWS SDK config: %w", err)

	}
	s3Client := s3client.GetClient(awsCfg)
	bucketRegion, err := s3client.GetBucketRegion(ctx, s3Client, bucket)
	if err != nil {
		return "", fmt.Errorf("unable to determine region for infra bucket '%s', make sure the bucket exists and consider providing the region explicitly: %w", bucket, err)
	}
	return bucketRegion, err
}
