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

package udfs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-logr/logr"

	"antrea.io/theia/snowflake/pkg/infra"
	sf "antrea.io/theia/snowflake/pkg/snowflake"
)

func GetFunctionName(baseName string, version string) string {
	version = strings.ReplaceAll(version, ".", "_")
	version = strings.ReplaceAll(version, "-", "_")
	return fmt.Sprintf("%s_%s", baseName, version)
}

func RunUdf(ctx context.Context, logger logr.Logger, query string, databaseName string, warehouseName string) (*sql.Rows, error) {
	logger.Info("Running UDF")
	dsn, _, err := sf.GetDSN()
	if err != nil {
		return nil, fmt.Errorf("failed to create DSN: %w", err)
	}

	db, err := sql.Open("snowflake", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Snowflake: %w", err)
	}
	defer db.Close()

	sfClient := sf.NewClient(db, logger)

	if err := sfClient.UseDatabase(ctx, databaseName); err != nil {
		return nil, err
	}

	if err := sfClient.UseSchema(ctx, infra.SchemaName); err != nil {
		return nil, err
	}

	if warehouseName == "" {
		temporaryWarehouse := infra.NewTemporaryWarehouse(sfClient, logger)
		warehouseName = temporaryWarehouse.Name()
		if err := temporaryWarehouse.Create(ctx); err != nil {
			return nil, err
		}
		defer func() {
			if err := temporaryWarehouse.Delete(ctx); err != nil {
				logger.Error(err, "Failed to delete temporary warehouse, please do it manually", "name", warehouseName)
			}
		}()
	}

	if err := sfClient.UseWarehouse(ctx, warehouseName); err != nil {
		return nil, err
	}

	rows, err := sfClient.QueryMultiStatement(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error when running UDF: %w", err)
	}
	return rows, nil
}
