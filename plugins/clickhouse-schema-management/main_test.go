// Copyright 2022 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database"
	dStub "github.com/golang-migrate/migrate/database/stub"
	"github.com/golang-migrate/migrate/source"
	sStub "github.com/golang-migrate/migrate/source/stub"
	"github.com/stretchr/testify/assert"
)

// fakeDirEntry implements os.DirEntry interface
type fakeDirEntry struct {
	name  string
	isDir bool
}

func (f fakeDirEntry) Name() string {
	return f.name
}
func (f fakeDirEntry) IsDir() bool {
	return f.isDir
}
func (f fakeDirEntry) Type() fs.FileMode {
	return 0
}
func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	return nil, nil
}

type migrationSequence []*migrate.Migration

func (m *migrationSequence) bodySequence() []string {
	r := make([]string, 0)
	for _, v := range *m {
		if v.Body != nil {
			body, err := io.ReadAll(v.Body)
			if err != nil {
				panic(err)
			}
			v.Body = io.NopCloser(bytes.NewReader(body))

			r = append(r, string(body[:]))
		} else {
			r = append(r, "<empty>")
		}
	}
	return r
}

// mr is a convenience func to create a new *Migration from the raw database query
func mr(value string) *migrate.Migration {
	return &migrate.Migration{
		Body: io.NopCloser(strings.NewReader(value)),
	}
}

var (
	sourceInstance   source.Driver
	databaseInstance database.Driver
	fakeExecCommand  = func(name string, arg ...string) *exec.Cmd {
		cmdArr := []string{name}
		cmdArr = append(cmdArr, arg...)
		return exec.Command("echo", cmdArr...)
	}

	fakeReadDir = func(name string) ([]os.DirEntry, error) {
		fileUpEntry1 := fakeDirEntry{name: "000001_0-1-0.up.sql", isDir: false}
		fileDownEntry1 := fakeDirEntry{name: "000001_0-1-0.down.sql", isDir: false}
		fileUpEntry2 := fakeDirEntry{name: "000002_0-3-0.up.sql", isDir: false}
		fileDownEntry2 := fakeDirEntry{name: "000002_0-3-0.down.sql", isDir: false}
		fileUpEntry3 := fakeDirEntry{name: "000003_0-5-0.up.sql", isDir: false}
		fileDownEntry3 := fakeDirEntry{name: "000003_0-5-0.down.sql", isDir: false}
		folderEntry := fakeDirEntry{name: "folder", isDir: true}
		return []os.DirEntry{fileUpEntry1, fileDownEntry1, fileUpEntry2, fileDownEntry2, fileUpEntry3, fileDownEntry3, folderEntry}, nil
	}

	fakeGetEnv = func(key string) string {
		switch key {
		case "MIGRATE_USERNAME":
			return "username"
		case "MIGRATE_PASSWORD":
			return "password"
		case "DB_URL":
			return "localhost:9000"
		case "THEIA_VERSION":
			return "0.4.0"
		default:
			return ""
		}
	}

	fakeMkdirAll = func(path string, perm fs.FileMode) error {
		return nil
	}

	fakeNewMigrate = func(sourceURL, databaseURL string) (*migrate.Migrate, error) {
		sourceStubMigrations := source.NewMigrations()
		sourceStubMigrations.Append(&source.Migration{Version: 1, Direction: source.Up, Identifier: "CREATE 1"})
		sourceStubMigrations.Append(&source.Migration{Version: 1, Direction: source.Down, Identifier: "DROP 1"})
		sourceStubMigrations.Append(&source.Migration{Version: 2, Direction: source.Up, Identifier: "CREATE 2"})
		sourceStubMigrations.Append(&source.Migration{Version: 2, Direction: source.Down, Identifier: "DROP 2"})
		sourceStubMigrations.Append(&source.Migration{Version: 3, Direction: source.Up, Identifier: "CREATE 3"})
		sourceStubMigrations.Append(&source.Migration{Version: 3, Direction: source.Down, Identifier: "DROP 3"})
		sourceInstance.(*sStub.Stub).Migrations = sourceStubMigrations

		return migrate.NewWithInstance("stub://", sourceInstance, "stub://", databaseInstance)
	}
)

func TestSchemaManagement(t *testing.T) {
	execCommand = fakeExecCommand
	readDir = fakeReadDir
	getEnv = fakeGetEnv
	mkdirAll = fakeMkdirAll
	newMigrate = fakeNewMigrate
	checkMigrations(t)
	checkErrorMsg(t)
}

func checkMigrations(t *testing.T) {
	testcases := []struct {
		name                string
		ms                  migrationSequence
		setDataVersion      func()
		showTablesRows      *sqlmock.Rows
		oldVersionTablesRow *sqlmock.Rows
	}{
		{
			name:           "No existing data schema",
			ms:             migrationSequence{},
			setDataVersion: func() {},
			showTablesRows: sqlmock.NewRows([]string{"table"}),
		},
		{
			name:           "Upgrading from v0.1.0 to v0.4.0",
			ms:             migrationSequence{mr("CREATE 1"), mr("CREATE 2")},
			setDataVersion: func() {},
			showTablesRows: sqlmock.NewRows([]string{"table"}).AddRow("flows"),
		},
		{
			name:                "Upgrading from v0.2.0 to v0.4.0",
			ms:                  migrationSequence{mr("CREATE 2")},
			setDataVersion:      func() {},
			showTablesRows:      sqlmock.NewRows([]string{"table"}).AddRow("flows").AddRow("migrate_version").AddRow("flows_local"),
			oldVersionTablesRow: sqlmock.NewRows([]string{"version"}).AddRow("0.2.0"),
		},
		{
			name:           "Downgrading from v0.6.0 to v0.4.0",
			ms:             migrationSequence{mr("DROP 3")},
			showTablesRows: sqlmock.NewRows([]string{"table"}).AddRow("schema_migrations"),
			setDataVersion: func() {
				databaseInstance.SetVersion(3, false)
			},
		},
		{
			name:           "No migration",
			ms:             migrationSequence{},
			showTablesRows: sqlmock.NewRows([]string{"table"}).AddRow("flows").AddRow("schema_migrations").AddRow("flows_local"),
			setDataVersion: func() {
				databaseInstance.SetVersion(2, false)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			openSql = func(driverName, dataSourceName string) (*sql.DB, error) {
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
				if err != nil {
					return db, err
				}
				mock.ExpectPing()
				mock.ExpectQuery("SHOW TABLES").WillReturnRows(tc.showTablesRows)
				if tc.oldVersionTablesRow != nil {
					mock.ExpectQuery("SELECT * FROM migrate_version").WillReturnRows(tc.oldVersionTablesRow)
				}
				return db, err
			}
			var err error
			sourceInstance, err = source.Open("stub://")
			assert.NoError(t, err, "error when creating stub source migrate")
			databaseInstance, err = database.Open("stub://")
			assert.NoError(t, err, "error when creating stub database migrate")
			clickhouseMigrate, err := initMigration()
			assert.NoErrorf(t, err, "error when initializing migrator: %v", err)
			assert.Equalf(t, 0, versionMap["0.1.0"], "version 0.1.0 should map to version 0")
			assert.Equalf(t, 1, versionMap["0.3.0"], "version 0.3.0 should map to version 1")
			assert.Equalf(t, 2, versionMap["0.5.0"], "version 0.5.0 should map to version 2")
			tc.setDataVersion()
			err = startMigration(clickhouseMigrate)
			assert.NoError(t, err, "error when migrating: %v", err)
			bs := tc.ms.bodySequence()
			assert.True(t, databaseInstance.(*dStub.Stub).EqualSequence(bs), "error in migration sequence")
		})
	}
}

func checkErrorMsg(t *testing.T) {
	testcases := []struct {
		name                  string
		execCommand           func(name string, arg ...string) *exec.Cmd
		readDir               func(name string) ([]fs.DirEntry, error)
		getEnv                func(key string) string
		newMigrate            func(sourceURL string, databaseURL string) (*migrate.Migrate, error)
		initExpectedErrorMsg  string
		startExpectedErrorMsg string
	}{
		{
			name:                 "Environment variable not set",
			getEnv:               func(key string) string { return "" },
			initExpectedErrorMsg: "unable to load environment variables, MIGRATE_USERNAME, MIGRATE_PASSWORD and DB_URL must be defined",
		},
		{
			name: "Fail to create migration instance",
			newMigrate: func(sourceURL, databaseURL string) (*migrate.Migrate, error) {
				return nil, fmt.Errorf("")
			},
			initExpectedErrorMsg: "error when creating a Migrate instance for ClickHouse: ",
		},
		{
			name: "Fail to read directory",
			readDir: func(name string) ([]fs.DirEntry, error) {
				return nil, fmt.Errorf("")
			},
			initExpectedErrorMsg: "error when generating version number map: unable to get files in folder migrators: ",
		},
		{
			name: "Wrong migrator file name",
			readDir: func(name string) ([]fs.DirEntry, error) {
				file := fakeDirEntry{name: "0-1-0.up.sql", isDir: false}
				return []os.DirEntry{file}, nil
			},
			initExpectedErrorMsg: "error when generating version number map: error when parsing the version number: ",
		},
		{
			name: "Invalid theia version",
			getEnv: func(key string) string {
				if key == "THEIA_VERSION" {
					return "v0.1.0"
				} else {
					return fakeGetEnv(key)
				}
			},
			startExpectedErrorMsg: "error when getting Theia version: error when getting theia version number for v0.1.0: error when rounding up version: ",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			execCommand = fakeExecCommand
			readDir = fakeReadDir
			getEnv = fakeGetEnv
			newMigrate = fakeNewMigrate
			if tc.execCommand != nil {
				execCommand = tc.execCommand
			}
			if tc.readDir != nil {
				readDir = tc.readDir
			}
			if tc.getEnv != nil {
				getEnv = tc.getEnv
			}
			if tc.newMigrate != nil {
				newMigrate = tc.newMigrate
			}
			openSql = func(driverName, dataSourceName string) (*sql.DB, error) {
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
				if err != nil {
					return db, err
				}
				mock.ExpectPing()
				mock.ExpectQuery("SHOW TABLES")
				return db, err
			}
			var err error
			sourceInstance, err = source.Open("stub://")
			assert.NoError(t, err, "error when creating stub source migrate")
			databaseInstance, err = database.Open("stub://")
			assert.NoError(t, err, "error when creating stub database migrate")
			clickhouseMigrate, err := initMigration()
			if tc.initExpectedErrorMsg != "" {
				assert.ErrorContains(t, err, tc.initExpectedErrorMsg)
			} else {
				assert.Nil(t, err)
				err = startMigration(clickhouseMigrate)
				if tc.startExpectedErrorMsg != "" {
					assert.ErrorContains(t, err, tc.startExpectedErrorMsg)
				} else {
					assert.Nil(t, err)
				}
			}

		})
	}
}
