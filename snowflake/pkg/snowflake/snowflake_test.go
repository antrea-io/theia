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

package snowflake

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const warehouseName = "ANTREA-QUERIES"

func TestCreateWarehouse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoErrorf(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	logger := testr.New(t)
	c := NewClient(db, logger)
	warehouseSize := WarehouseSizeType("XSMALL")
	minClusterCount := int32(1)
	maxClusterCount := int32(3)
	scalingPolicy := ScalingPolicyStandard
	autoSuspend := int32(60)
	intiallySuspended := true
	config := WarehouseConfig{
		Size:               &warehouseSize,
		MinClusterCount:    &minClusterCount,
		MaxClusterCount:    &maxClusterCount,
		ScalingPolicy:      &scalingPolicy,
		AutoSuspend:        &autoSuspend,
		InitiallySuspended: &intiallySuspended,
	}
	query := fmt.Sprintf("CREATE WAREHOUSE %s WITH WAREHOUSE_SIZE = %s MIN_CLUSTER_COUNT = %d MAX_CLUSTER_COUNT = %d SCALING_POLICY = %s AUTO_SUSPEND = %d INITIALLY_SUSPENDED = %t", warehouseName, *config.Size, *config.MinClusterCount, *config.MaxClusterCount, *config.ScalingPolicy, *config.AutoSuspend, *config.InitiallySuspended)

	for _, tc := range []struct {
		name          string
		prepareMock   func(mock sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name: "Successful case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
		{
			name: "Failed case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnError(fmt.Errorf("some error"))
			},
			expectedError: fmt.Errorf("some error"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.prepareMock(mock)
			execErr := c.CreateWarehouse(context.TODO(), warehouseName, config)
			assert.Equal(t, tc.expectedError, execErr)
			err := mock.ExpectationsWereMet()
			assert.NoErrorf(t, err, "there were unfulfilled expectations")
		})
	}
}

func TestUseWarehouse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoErrorf(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	logger := testr.New(t)
	c := NewClient(db, logger)
	query := fmt.Sprintf("USE WAREHOUSE %s", warehouseName)

	for _, tc := range []struct {
		name          string
		prepareMock   func(mock sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name: "Successful case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
		{
			name: "Failed case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnError(fmt.Errorf("some error"))
			},
			expectedError: fmt.Errorf("some error"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.prepareMock(mock)
			execErr := c.UseWarehouse(context.TODO(), warehouseName)
			assert.Equal(t, tc.expectedError, execErr)
			err := mock.ExpectationsWereMet()
			assert.NoErrorf(t, err, "there were unfulfilled expectations")
		})
	}
}

func TestDropWarehouse(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoErrorf(t, err, "an error was not expected when opening a stub database connection")
	defer db.Close()

	logger := testr.New(t)
	c := NewClient(db, logger)
	query := fmt.Sprintf("DROP WAREHOUSE IF EXISTS %s", warehouseName)

	for _, tc := range []struct {
		name          string
		prepareMock   func(mock sqlmock.Sqlmock)
		expectedError error
	}{
		{
			name: "Successful case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
		{
			name: "Failed case",
			prepareMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WillReturnError(fmt.Errorf("some error"))
			},
			expectedError: fmt.Errorf("some error"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.prepareMock(mock)
			execErr := c.DropWarehouse(context.TODO(), warehouseName)
			assert.Equal(t, tc.expectedError, execErr)
			err := mock.ExpectationsWereMet()
			assert.NoErrorf(t, err, "there were unfulfilled expectations")
		})
	}
}
