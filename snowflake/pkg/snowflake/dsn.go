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
	"fmt"
	"os"
	"strconv"

	sf "github.com/snowflakedb/gosnowflake"
)

type MissingEnvVariableError struct {
	envVarName string
}

func (e MissingEnvVariableError) Error() string {
	return fmt.Sprintf("env variable '%s' is not set", e.envVarName)
}

func SetWarehouse(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Warehouse = name
	}
}

func SetDatabase(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Database = name
	}
}

func SetSchema(name string) func(*sf.Config) {
	return func(cfg *sf.Config) {
		cfg.Schema = name
	}
}

// GetDSN constructs a DSN based on the test connection parameters
func GetDSN(options ...func(*sf.Config)) (string, *sf.Config, error) {
	env := func(k string, failOnMissing bool) (string, error) {
		if value := os.Getenv(k); value != "" {
			return value, nil
		}
		if failOnMissing {
			return "", MissingEnvVariableError{k}
		}
		return "", nil
	}

	account, err := env("SNOWFLAKE_ACCOUNT", true)
	if err != nil {
		return "", nil, err
	}
	user, err := env("SNOWFLAKE_USER", true)
	if err != nil {
		return "", nil, err
	}
	password, err := env("SNOWFLAKE_PASSWORD", true)
	if err != nil {
		return "", nil, err
	}
	host, _ := env("SNOWFLAKE_HOST", false)
	portStr, _ := env("SNOWFLAKE_PORT", false)
	protocol, _ := env("SNOWFLAKE_PROTOCOL", false)

	port := 443 // snowflake default port
	if len(portStr) > 0 {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", nil, err
		}
	}

	cfg := &sf.Config{
		Account:  account,
		User:     user,
		Password: password,
		Host:     host,
		Port:     port,
		Protocol: protocol,
	}

	for _, fn := range options {
		fn(cfg)
	}

	dsn, err := sf.DSN(cfg)
	return dsn, cfg, err
}
