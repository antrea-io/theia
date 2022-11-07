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
	"testing"
	"time"

	kmsclienttesting "antrea.io/theia/snowflake/pkg/aws/client/kms/testing"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestScheduleKeyDeletion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLogger := logger
	defer func() {
		logger = origLogger
	}()
	logger = testr.New(t)

	mockKmsClient := kmsclienttesting.NewMockInterface(ctrl)
	keyId := ""
	waitingPeriodDays := int32(30)
	input := kms.ScheduleKeyDeletionInput{
		KeyId:               &keyId,
		PendingWindowInDays: &waitingPeriodDays,
	}
	output := kms.ScheduleKeyDeletionOutput{
		DeletionDate: &time.Time{},
	}
	mockKmsClient.EXPECT().ScheduleKeyDeletion(context.TODO(), &input).Return(&output, nil)
	err := scheduleKeyDeletion(context.TODO(), mockKmsClient, keyId)
	assert.NoError(t, err)
}
