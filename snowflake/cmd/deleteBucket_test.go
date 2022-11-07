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
	"fmt"
	"testing"

	s3clienttesting "antrea.io/theia/snowflake/pkg/aws/client/s3/testing"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/go-logr/logr/testr"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestDeleteS3Objects(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLogger := logger
	defer func() {
		logger = origLogger
	}()
	logger = testr.New(t)

	name := ""
	prefix := ""

	t.Run("Delete objects from non-empty bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		k := ""
		output := s3.ListObjectsV2Output{
			Contents: []s3types.Object{s3types.Object{Key: &k}},
		}
		mockS3Client.EXPECT().ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: &name,
			Prefix: &prefix,
		}).Return(&output, nil)
		keys := []s3types.ObjectIdentifier{s3types.ObjectIdentifier{
			Key: &k,
		}}
		deleteObjectsInput := s3.DeleteObjectsInput{
			Bucket: &name,
			Delete: &s3types.Delete{
				Objects: keys,
			},
		}
		mockS3Client.EXPECT().DeleteObjects(context.TODO(), &deleteObjectsInput).Times(1)
		err := deleteS3Objects(context.TODO(), mockS3Client, name)
		assert.NoError(t, err)
	})

	t.Run("Delete objects from empty bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		output := s3.ListObjectsV2Output{
			Contents: []s3types.Object{},
		}
		mockS3Client.EXPECT().ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: &name,
			Prefix: &prefix,
		}).Return(&output, nil)
		err := deleteS3Objects(context.TODO(), mockS3Client, name)
		assert.NoError(t, err)
	})
}

func TestDeleteBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	origLogger := logger
	defer func() {
		logger = origLogger
	}()
	logger = testr.New(t)

	name := ""

	t.Run("Delete existing bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		input := s3.DeleteBucketInput{
			Bucket: &name,
		}
		mockS3Client.EXPECT().DeleteBucket(context.TODO(), &input).Return(nil, nil)
		err := deleteBucket(context.TODO(), mockS3Client, name)
		assert.NoError(t, err)
	})

	t.Run("Delete non-existing bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		input := s3.DeleteBucketInput{
			Bucket: &name,
		}
		mockS3Client.EXPECT().DeleteBucket(context.TODO(), &input).Return(nil, &s3types.NoSuchBucket{})
		err := deleteBucket(context.TODO(), mockS3Client, name)
		assert.Error(t, err)
	})

	t.Run("Error when deleting bucket", func(t *testing.T) {
		mockS3Client := s3clienttesting.NewMockInterface(ctrl)
		input := s3.DeleteBucketInput{
			Bucket: &name,
		}
		mockS3Client.EXPECT().DeleteBucket(context.TODO(), &input).Return(nil, fmt.Errorf("random error"))
		err := deleteBucket(context.TODO(), mockS3Client, name)
		assert.Error(t, err)
	})
}
