package infra

import (
	"fmt"
)

func S3StateBackendURL(bucket, prefix, region string) string {
	return fmt.Sprintf("s3://%s/%s?region=%s", bucket, prefix, region)
}
