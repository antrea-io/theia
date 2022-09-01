package kms

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

func GetClient(cfg aws.Config) Interface {
	return kms.NewFromConfig(cfg)
}
