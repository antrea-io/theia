package sqs

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func GetClient(cfg aws.Config) Interface {
	return sqs.NewFromConfig(cfg)
}
