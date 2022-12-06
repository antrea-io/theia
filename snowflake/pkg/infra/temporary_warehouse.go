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

package infra

import (
	"context"
	"fmt"
	"strings"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/go-logr/logr"

	sf "antrea.io/theia/snowflake/pkg/snowflake"
)

type temporaryWarehouse struct {
	sfClient      sf.Client
	logger        logr.Logger
	warehouseName string
}

func NewTemporaryWarehouse(sfClient sf.Client, logger logr.Logger) *temporaryWarehouse {
	return &temporaryWarehouse{
		sfClient:      sfClient,
		logger:        logger,
		warehouseName: strings.ToUpper(petname.Generate(3, "_")),
	}
}

func (w *temporaryWarehouse) Name() string {
	return w.warehouseName
}

func (w *temporaryWarehouse) Create(ctx context.Context) error {
	warehouseSize := sf.WarehouseSizeType("XSMALL")
	autoSuspend := int32(60) // minimum value
	intiallySuspended := true
	w.logger.Info("Creating Snowflake warehouse", "name", w.warehouseName, "size", warehouseSize)
	if err := w.sfClient.CreateWarehouse(ctx, w.warehouseName, sf.WarehouseConfig{
		Size:               &warehouseSize,
		AutoSuspend:        &autoSuspend,
		InitiallySuspended: &intiallySuspended,
	}); err != nil {
		return fmt.Errorf("error when creating Snowflake warehouse: %w", err)
	}
	w.logger.Info("Created Snowflake warehouse", "name", w.warehouseName)
	return nil
}

func (w *temporaryWarehouse) Delete(ctx context.Context) error {
	w.logger.Info("Deleting Snowflake warehouse", "name", w.warehouseName)
	if err := w.sfClient.DropWarehouse(ctx, w.warehouseName); err != nil {
		return fmt.Errorf("error when deleting Snowflake warehouse: %w", err)
	}
	w.logger.Info("Deleted Snowflake warehouse", "name", w.warehouseName)
	return nil
}
