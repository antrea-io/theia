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
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go"
	"github.com/golang-migrate/migrate"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	_ "github.com/golang-migrate/migrate/database/clickhouse"
	_ "github.com/golang-migrate/migrate/source/file"
)

const (
	migratorTmpPath        = "/docker-entrypoint-initdb.d/migrators"
	migratorPersistentPath = "/var/lib/clickhouse/migrators"
)

var (
	versionMap    = make(map[string]int)
	clickHouseURL string
	execCommand   = exec.Command
	readDir       = os.ReadDir
	getEnv        = os.Getenv
	mkdirAll      = os.MkdirAll
	openSql       = sql.Open
	newMigrate    = migrate.New
)

func main() {
	clickhouseMigrate, err := initMigration()
	if err != nil {
		klog.ErrorS(err, "Error when initializing migration")
	}
	defer clickhouseMigrate.Close()
	if err := startMigration(clickhouseMigrate); err != nil {
		klog.ErrorS(err, "Error when migrating")
	}
}

func initMigration() (*migrate.Migrate, error) {
	// Copy migrators from tmp path to persistent path for the downgrading usage in the future
	if err := copyMigrators(); err != nil {
		return nil, fmt.Errorf("error when copying migrators: %v", err)
	}
	// versionMap is used to map Theia version string to golang-migrate version number
	if err := initializeVersionMap(); err != nil {
		return nil, fmt.Errorf("error when generating version number map: %v", err)
	}
	userName := getEnv("MIGRATE_USERNAME")
	password := getEnv("MIGRATE_PASSWORD")
	databaseURL := getEnv("DB_URL")
	if len(userName) == 0 || len(password) == 0 || len(databaseURL) == 0 {
		return nil, fmt.Errorf("unable to load environment variables, MIGRATE_USERNAME, MIGRATE_PASSWORD and DB_URL must be defined")
	}
	clickHouseURL = fmt.Sprintf("%s?username=%s&password=%s", databaseURL, userName, password)
	migrateDatabaseURL := fmt.Sprintf("clickhouse://%s&x-multi-statement=true", clickHouseURL)
	migrateSourceURL := fmt.Sprintf("file://%s", migratorPersistentPath)
	clickhouseMigrate, err := newMigrate(migrateSourceURL, migrateDatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("error when creating a Migrate instance for ClickHouse: %v", err)
	}
	return clickhouseMigrate, nil
}

// Get Theia version and data version number, migrate if they are different
func startMigration(clickhouseMigrate *migrate.Migrate) error {
	theiaVersionNumber, err := getTheiaVersionNumber()
	if err != nil {
		return fmt.Errorf("error when getting Theia version: %v", err)
	}
	dataVersionNumber, err := getDataVersionNumber(*clickhouseMigrate)
	if err != nil {
		return fmt.Errorf("error when getting the data version: %v", err)
	}
	if theiaVersionNumber == dataVersionNumber {
		klog.InfoS("Data schema version is the same as Theia version. Migration skipped.")
	} else if dataVersionNumber == -1 {
		klog.InfoS("No existing data schema. Migration skipped.")
	} else {
		klog.InfoS("Migrate data schema", "from", dataVersionNumber, "to", theiaVersionNumber)
		err = clickhouseMigrate.Steps(theiaVersionNumber - dataVersionNumber)
		if err != nil {
			return fmt.Errorf("error when applying migrations: %v", err)
		}
	}
	// Set the data schema version to Theia version anyway, as we expect initial
	// data will be created even if the migration is skipped.
	err = clickhouseMigrate.Force(theiaVersionNumber)
	if err != nil {
		return fmt.Errorf("error when setting version: %v", err)
	}
	return nil
}

func copyMigrators() error {
	if err := mkdirAll(migratorPersistentPath, os.ModeDir); err != nil {
		return fmt.Errorf("error when creating folder: %s, error: %v", migratorPersistentPath, err)
	}
	// Not sanitize migratorTmpPath and migratorPersistentPath as they are constant.
	sourcePath := fmt.Sprintf("%s/.", migratorTmpPath)
	cmd := execCommand("cp", "-r", sourcePath, migratorPersistentPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error when copying files, error: %v", err)
	}
	return nil
}

// Use the file names to map the Theia version string to golang-migrate version number
func initializeVersionMap() error {
	files, err := readDir(migratorPersistentPath)
	if err != nil {
		return fmt.Errorf("unable to get files in folder migrators: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		// fileName example: 000001_0-1-0.down.sql
		fileNameArr := strings.Split(strings.Split(file.Name(), ".")[0], "_")
		versionNumber, err := strconv.ParseInt(fileNameArr[0], 10, 64)
		if err != nil {
			return fmt.Errorf("error when parsing the version number: %v", err)
		}
		version := strings.Replace(fileNameArr[1], "-", ".", -1)
		// File <golang-migrate-version>_<theia-version>.up.sql is expected to be
		// applied when upgrading from <theia-version>.
		// golang-migrate applies this file when upgrading from <golang-migrate-version> - 1.
		versionMap[version] = int(versionNumber - 1)
	}
	return nil
}

// Get Theia version based on the environment variable
func getTheiaVersionNumber() (int, error) {
	theiaVersion := getEnv("THEIA_VERSION")
	if len(theiaVersion) == 0 {
		return 0, fmt.Errorf("unable to load environment variables, THEIA_VERSION must be defined")
	}
	theiaVersionNumber, err := getVersionNumber(theiaVersion)
	if err != nil {
		return theiaVersionNumber, fmt.Errorf("error when getting theia version number for %s: %v", theiaVersion, err)
	}
	return theiaVersionNumber, nil
}

// From v0.3, get data version based on version table
// For v0.1 and v0.2, determine version based on tables in database
func getDataVersionNumber(clickhouseMigrate migrate.Migrate) (int, error) {
	var version int
	// Get data version for version before v0.3
	versionStr, err := getDataVersionBasedOnTables()
	if err != nil {
		return 0, fmt.Errorf("error when getting data version based on tables: %v", err)
	}
	if versionStr != "" {
		version, err = getVersionNumber(versionStr)
		if err != nil {
			return version, fmt.Errorf("error when getting version number for %s: %v", versionStr, err)
		}
		if version > 0 {
			// Set data version for golang-migrate tool
			err = clickhouseMigrate.Force(version)
			if err != nil {
				return 0, fmt.Errorf("error when collarating data version: %v", err)
			}
		}
		return version, nil
	}
	// Get data version for version after v0.3
	uintVersion, _, err := clickhouseMigrate.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return version, fmt.Errorf("error when getting migration version: %v", err)
	}
	// No data schema created before
	if err == migrate.ErrNilVersion {
		return -1, nil
	}
	version = int(uintVersion)
	return version, nil
}

func getVersionNumber(version string) (int, error) {
	versionNumber, ok := versionMap[version]
	if !ok {
		var err error
		// In case data schema does not change between version A and B (assuming B is the later version)
		// we will not have file xxx_A.up.sql and xxx_A.down.sql
		// In migration, we can treat version A the same as version C,
		// which is the first version after A having data schema changes comparing its next version.
		versionNumber, err = roundUpVersion(version)
		if err != nil {
			return versionNumber, fmt.Errorf("error when rounding up version: %v", err)
		}
	}
	return versionNumber, nil
}

func roundUpVersion(version string) (int, error) {
	var versionNumber int
	for key, value := range versionMap {
		less, err := versionLessThan(key, version)
		if err != nil {
			return versionNumber, fmt.Errorf("error when comparing version %s and %s: %v", key, version, err)
		}
		if less && (versionNumber < value+1) {
			versionNumber = value + 1
		}
	}
	return versionNumber, nil
}

// Return true if version a is earlier than version b.
func versionLessThan(a, b string) (bool, error) {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as); i++ {
		xi, err := strconv.Atoi(as[i])
		if err != nil {
			return false, fmt.Errorf("error when parsing version %s: %v", a, err)
		}
		yi, err := strconv.Atoi(bs[i])
		if err != nil {
			return false, fmt.Errorf("error when parsing version %s: %v", b, err)
		}
		if xi == yi {
			continue
		}
		return (xi < yi), nil
	}
	return false, nil
}

// go-migrate is introduced in Theia v0.3. It cannot distinguish versions before v0.3 automatically.
// If table flows_local is created, the data version is v0.2.
// If only table flows_local does not exist while table flows is created, the data version is v0.1.
// If non of these two tables are found, it means no data schema has been created before.
func getDataVersionBasedOnTables() (string, error) {
	// Query to ClickHouse time out if it fails for 10 seconds.
	queryTimeout := 10 * time.Second
	// Retry query to ClickHouse every second if it fails.
	queryRetryInterval := 1 * time.Second

	connect, err := connectClickHouse()
	if err != nil {
		return "", fmt.Errorf("error when connecting to ClickHouse: %v", err)
	}
	var tableName string
	var version string
	command := "SHOW TABLES"
	var containsFlowsLocal, containsFlows bool
	if err := wait.PollImmediate(queryRetryInterval, queryTimeout, func() (bool, error) {
		rows, err := connect.Query(command)
		if err != nil {
			return false, nil
		} else {
			for rows.Next() {
				if err := rows.Scan(&tableName); err != nil {
					return false, nil
				}
				if tableName == "migrate_version" {
					var versionFromOldTable string
					err = connect.QueryRow("SELECT * FROM migrate_version").Scan(&versionFromOldTable)
					if err != nil {
						return false, nil
					}
					if versionFromOldTable == "0.2.0" {
						version = versionFromOldTable
					}
				}
				if tableName == "flows" {
					containsFlows = true
				}
				if tableName == "flows_local" {
					containsFlowsLocal = true
				}
			}
			return true, nil
		}
	}); err != nil {
		return version, err
	}
	if containsFlows && !containsFlowsLocal {
		version = "0.1.0"
	}
	return version, nil
}

func connectClickHouse() (*sql.DB, error) {
	var connect *sql.DB
	var connErr error
	connRetryInterval := 1 * time.Second
	connTimeout := 10 * time.Second

	// Connect to ClickHouse in a loop
	if err := wait.PollImmediate(connRetryInterval, connTimeout, func() (bool, error) {
		// Open the database and ping it
		var err error
		url := fmt.Sprintf("tcp://%s", clickHouseURL)
		connect, err = openSql("clickhouse", url)
		if err != nil {
			connErr = fmt.Errorf("failed to open ClickHouse: %v", err)
			return false, nil
		}
		if err := connect.Ping(); err != nil {
			if exception, ok := err.(*clickhouse.Exception); ok {
				connErr = fmt.Errorf("failed to ping ClickHouse: %v", exception.Message)
			} else {
				connErr = fmt.Errorf("failed to ping ClickHouse: %v", err)
			}
			return false, nil
		} else {
			return true, nil
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to connect to ClickHouse after %s: %v", connTimeout, connErr)
	}
	return connect, nil
}
