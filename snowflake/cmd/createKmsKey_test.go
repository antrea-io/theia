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

	kmsclienttesting "antrea.io/theia/snowflake/pkg/aws/client/kms/testing"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestCreateKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLogger := logger
	defer func() {
		logger = origLogger
	}()
	logger = testr.New(t)

	mockKmsClient := kmsclienttesting.NewMockInterface(ctrl)
	description := "This key was created by theia-sf; it is used to encrypt infrastructure state"
	input := kms.CreateKeyInput{
		Description: &description,
	}
	keyId := "randomKeyId"
	output := kms.CreateKeyOutput{
		KeyMetadata: &types.KeyMetadata{
			KeyId: &keyId,
		},
	}
	mockKmsClient.EXPECT().CreateKey(context.TODO(), &input).Return(&output, nil)
	outputKeyId, err := createKey(context.TODO(), mockKmsClient)
	assert.NoError(t, err)
	assert.Equal(t, keyId, outputKeyId)
}
