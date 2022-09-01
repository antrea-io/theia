package s3

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func GetClient(cfg aws.Config) Interface {
	return s3.NewFromConfig(cfg)
}
