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
	"bytes"
	"context"
	"testing"

	sqsclienttesting "antrea.io/theia/snowflake/pkg/aws/client/sqs/testing"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestReceiveSQSMessage(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var b bytes.Buffer

	for _, tc := range []struct {
		name           string
		queueEmpty     bool
		deleteExpected bool
	}{
		{
			name:           "Receive message from empty queue",
			queueEmpty:     true,
			deleteExpected: true,
		},
		{
			name:           "Receive message from non-empty queue with deletion",
			queueEmpty:     false,
			deleteExpected: true,
		},
		{
			name:           "Receive message from non-empty queue without deletion",
			queueEmpty:     false,
			deleteExpected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b.Reset()
			mockSqsClient := sqsclienttesting.NewMockInterface(ctrl)
			name := ""
			messageBody := "randomMessageBody"
			getQueueUrlInput := sqs.GetQueueUrlInput{
				QueueName: &name,
			}
			url := ""
			getQueueUrlOutput := sqs.GetQueueUrlOutput{
				QueueUrl: &url,
			}
			mockSqsClient.EXPECT().GetQueueUrl(context.TODO(), &getQueueUrlInput).Return(&getQueueUrlOutput, nil)
			receiveMessageInput := sqs.ReceiveMessageInput{
				QueueUrl:            getQueueUrlOutput.QueueUrl,
				MaxNumberOfMessages: int32(1),
				WaitTimeSeconds:     int32(0),
			}
			if tc.queueEmpty {
				output := sqs.ReceiveMessageOutput{
					Messages: []types.Message{},
				}
				mockSqsClient.EXPECT().ReceiveMessage(context.TODO(), &receiveMessageInput).Return(&output, nil)
			} else {
				receiptHandle := "randomReceiptHandle"
				receiveMessageOutput := sqs.ReceiveMessageOutput{
					Messages: []types.Message{types.Message{Body: &messageBody, ReceiptHandle: &receiptHandle}},
				}
				mockSqsClient.EXPECT().ReceiveMessage(context.TODO(), &receiveMessageInput).Return(&receiveMessageOutput, nil)
				if tc.deleteExpected {
					deleteMessageInput := sqs.DeleteMessageInput{
						QueueUrl:      &url,
						ReceiptHandle: &receiptHandle,
					}
					mockSqsClient.EXPECT().DeleteMessage(context.TODO(), &deleteMessageInput).Return(nil, nil)
				}
			}
			receiveSQSMessage(context.TODO(), mockSqsClient, name, tc.deleteExpected, &b)
			if !tc.queueEmpty {
				assert.Contains(t, b.String(), messageBody)
			}
		})
	}
}
