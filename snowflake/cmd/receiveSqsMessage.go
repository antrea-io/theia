/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"time"

	awsarn "github.com/aws/aws-sdk-go-v2/aws/arn"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/spf13/cobra"

	sqsclient "antrea.io/theia/snowflake/pkg/aws/client/sqs"
)

// receiveSqsMessageCmd represents the receive-sqs-message command
var receiveSqsMessageCmd = &cobra.Command{
	Use:   "receive-sqs-message",
	Short: "Receive a message from an AWS SQS queue",
	Long: `This command can be used to receive a message from an SQS queue
in your AWS account. Snowflake data ingestion errors are sent to an SQS queue,
whose ARN is displayed when running the "onboard" command. While ingestion
errors are not expected when using the Antrea Flow Aggregator to export flows,
this command can prove useful if flow records don't get ingested into your
Snowflake account as expected.

To receive a message without deleting it (message will remain in the queue and
become available to consummers again after a short time interval):
"theia-sf receive-sqs-message --queue-arn <ARN>"

To receive a message and delete it from the queue:
"theia-sf receive-sqs-message --queue-arn <ARN> --delete"

Note that this command will not block: if no message is available in the queue,
it will return immediately.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		region, _ := cmd.Flags().GetString("region")
		sqsQueueARN, _ := cmd.Flags().GetString("queue-arn")
		delete, _ := cmd.Flags().GetBool("delete")
		arn, err := awsarn.Parse(sqsQueueARN)
		if err != nil {
			return fmt.Errorf("invalid ARN '%s': %w", sqsQueueARN, err)
		}
		if region != "" && arn.Region != region {
			return fmt.Errorf("region conflict between --region flag and ARN region")
		}
		region = arn.Region
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)

		}
		sqsClient := sqsclient.GetClient(awsCfg)
		return receiveSQSMessage(ctx, sqsClient, arn.Resource, delete)
	},
}

func receiveSQSMessage(ctx context.Context, sqsClient sqsclient.Interface, queueName string, delete bool) error {
	queueURL, err := func() (string, error) {
		output, err := sqsClient.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
			QueueName: &queueName,
		})
		if err != nil {
			return "", err
		}
		return *output.QueueUrl, nil
	}()
	if err != nil {
		return fmt.Errorf("error when retrieving SQS queue URL: %v", err)
	}
	output, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &queueURL,
		MaxNumberOfMessages: int32(1),
		WaitTimeSeconds:     int32(0),
	})
	if err != nil {
		return fmt.Errorf("error when receiving message from SQS queue: %v", err)
	}
	if len(output.Messages) == 0 {
		return nil
	}
	message := output.Messages[0]
	fmt.Println(*message.Body)
	if !delete {
		return nil
	}
	_, err = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: message.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("error when deleting message from SQS queue: %v", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(receiveSqsMessageCmd)

	receiveSqsMessageCmd.Flags().String("region", GetEnv("AWS_REGION", defaultRegion), "region of the SQS queue")
	receiveSqsMessageCmd.Flags().String("queue-arn", "", "ARN of the SQS queue")
	receiveSqsMessageCmd.MarkFlagRequired("queue-arn")
	receiveSqsMessageCmd.Flags().Bool("delete", false, "delete received message from SQS queue")
}
