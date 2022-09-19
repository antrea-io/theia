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
