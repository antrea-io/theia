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
	"os"
	"strconv"
	"testing"

	sf "github.com/snowflakedb/gosnowflake"
	"github.com/stretchr/testify/assert"
)

func TestGetDSN(t *testing.T) {
	for _, tc := range []struct {
		name                string
		envMap              map[string]string
		expectedDSN         string
		expectedCFG         *sf.Config
		expectedError       error
		expectedFatalErrMsg string
	}{
		{
			name: "Successful case",
			envMap: map[string]string{
				"SNOWFLAKE_ACCOUNT":  "abc",
				"SNOWFLAKE_USER":     "test-user",
				"SNOWFLAKE_PASSWORD": "test-password",
			},
			expectedDSN: "test-user:test-password@abc.snowflakecomputing.com:443?ocspFailOpen=true&validateDefaultParameters=true",
			expectedCFG: &sf.Config{
				Account:  "abc",
				User:     "test-user",
				Password: "test-password",
				Host:     "abc.snowflakecomputing.com",
				Port:     443,
				Protocol: "https",
			},
			expectedError: nil,
		},
		{
			name: "Missing Snowflake account",
			envMap: map[string]string{
				"SNOWFLAKE_USER":     "test-user",
				"SNOWFLAKE_PASSWORD": "test-password",
			},
			expectedDSN:   "",
			expectedCFG:   nil,
			expectedError: MissingEnvVariableError{"SNOWFLAKE_ACCOUNT"},
		},
		{
			name: "Wrong Snowflake port format",
			envMap: map[string]string{
				"SNOWFLAKE_ACCOUNT":  "abc",
				"SNOWFLAKE_USER":     "test-user",
				"SNOWFLAKE_PASSWORD": "test-password",
				"SNOWFLAKE_PORT":     "1BB",
			},
			expectedDSN:   "",
			expectedCFG:   nil,
			expectedError: strconv.ErrSyntax,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envMap {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			dsn, cfg, err := GetDSN()
			if tc.expectedDSN != "" {
				assert.Equal(t, tc.expectedDSN, dsn)
				if tc.expectedCFG != nil {
					assert.Equal(t, tc.expectedCFG.Account, cfg.Account)
					assert.Equal(t, tc.expectedCFG.User, cfg.User)
					assert.Equal(t, tc.expectedCFG.Password, cfg.Password)
					assert.Equal(t, tc.expectedCFG.Host, cfg.Host)
					assert.Equal(t, tc.expectedCFG.Port, cfg.Port)
					assert.Equal(t, tc.expectedCFG.Protocol, cfg.Protocol)
				}
			}
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}
