package s3

import (
	"context"

	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
)

func GetBucketRegion(ctx context.Context, client Interface, bucket string) (string, error) {
	return s3manager.GetBucketRegion(ctx, client, bucket)
}
