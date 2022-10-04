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
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
)

type WarehouseSizeType string

type ScalingPolicyType string

const (
	ScalingPolicyStandard ScalingPolicyType = "STANDARD"
	ScalingPolicyEconomy  ScalingPolicyType = "ECONOMY"
)

type WarehouseConfig struct {
	Size               *WarehouseSizeType
	MinClusterCount    *int32
	MaxClusterCount    *int32
	ScalingPolicy      *ScalingPolicyType
	AutoSuspend        *int32
	InitiallySuspended *bool
}

type Client interface {
	CreateWarehouse(ctx context.Context, name string, config WarehouseConfig) error
	UseWarehouse(ctx context.Context, name string) error
	DropWarehouse(ctx context.Context, name string) error
}

type client struct {
	db     *sql.DB
	logger logr.Logger
}

func NewClient(db *sql.DB, logger logr.Logger) *client {
	return &client{
		db:     db,
		logger: logger,
	}
}

func (c *client) CreateWarehouse(ctx context.Context, name string, config WarehouseConfig) error {
	query := fmt.Sprintf("CREATE WAREHOUSE %s", name)
	properties := make([]string, 0)
	if config.Size != nil {
		properties = append(properties, fmt.Sprintf("WAREHOUSE_SIZE = %s", *config.Size))
	}
	if config.MinClusterCount != nil {
		properties = append(properties, fmt.Sprintf("MIN_CLUSTER_COUNT = %d", *config.MinClusterCount))
	}
	if config.MaxClusterCount != nil {
		properties = append(properties, fmt.Sprintf("MAX_CLUSTER_COUNT = %d", *config.MaxClusterCount))
	}
	if config.ScalingPolicy != nil {
		properties = append(properties, fmt.Sprintf("SCALING_POLICY = %s", *config.ScalingPolicy))
	}
	if config.AutoSuspend != nil {
		properties = append(properties, fmt.Sprintf("AUTO_SUSPEND = %d", *config.AutoSuspend))
	}
	if config.InitiallySuspended != nil {
		properties = append(properties, fmt.Sprintf("INITIALLY_SUSPENDED = %t", *config.InitiallySuspended))
	}
	if len(properties) > 0 {
		query += " WITH " + strings.Join(properties, " ")
	}
	c.logger.V(2).Info("Snowflake query", "query", query)
	_, err := c.db.ExecContext(ctx, query)
	return err
}

func (c *client) UseWarehouse(ctx context.Context, name string) error {
	query := fmt.Sprintf("USE WAREHOUSE %s", name)
	c.logger.V(2).Info("Snowflake query", "query", query)
	_, err := c.db.ExecContext(ctx, query)
	return err
}

func (c *client) DropWarehouse(ctx context.Context, name string) error {
	query := fmt.Sprintf("DROP WAREHOUSE IF EXISTS %s", name)
	c.logger.V(2).Info("Snowflake query", "query", query)
	_, err := c.db.ExecContext(ctx, query)
	return err
}
