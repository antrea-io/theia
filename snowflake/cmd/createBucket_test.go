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

	s3clienttesting "antrea.io/theia/snowflake/pkg/aws/client/s3/testing"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestCreateBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLogger := logger
	defer func() {
		logger = origLogger
	}()
	logger = testr.New(t)

	name := ""
	region := ""

	t.Run("Create existing bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		mockS3Client.EXPECT().HeadBucket(context.TODO(), &s3.HeadBucketInput{Bucket: &name}).Return(nil, nil)
		err := createBucket(context.TODO(), mockS3Client, name, region)
		assert.NoError(t, err)
	})

	t.Run("Create new bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		mockS3Client.EXPECT().HeadBucket(context.TODO(), &s3.HeadBucketInput{Bucket: &name}).Return(nil, &s3types.NotFound{})
		input := s3.CreateBucketInput{
			Bucket: &name,
			ACL:    s3types.BucketCannedACLPrivate,
			CreateBucketConfiguration: &s3types.CreateBucketConfiguration{
				LocationConstraint: s3types.BucketLocationConstraint(region),
			},
		}
		mockS3Client.EXPECT().CreateBucket(context.TODO(), &input).Return(nil, nil)
		err := createBucket(context.TODO(), mockS3Client, name, region)
		assert.NoError(t, err)
	})
}
