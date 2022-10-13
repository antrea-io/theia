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
	"os"
	"path/filepath"
	"testing"

	"antrea.io/theia/snowflake/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMigrationsToDisk(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "antrea-pulumi-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	err = writeMigrationsToDisk(database.Migrations, database.MigrationsPath, filepath.Join(tempDir, migrationsDir))
	require.NoError(t, err)
	entries, err := database.Migrations.ReadDir(database.MigrationsPath)
	require.NoError(t, err)
	for _, entry := range entries {
		_, err := os.Stat(filepath.Join(tempDir, migrationsDir, entry.Name()))
		assert.NoErrorf(t, err, "Migration file %s not exist", entry.Name())
	}
}
