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

package clickhouse

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testNamespace = "flow-visibility"
)

func TestSetupConnection(t *testing.T) {
	testCases := []struct {
		name             string
		setup            func() (*sql.DB, sqlmock.Sqlmock)
		cleanup          func()
		expectedErrorMsg string
	}{
		{
			name: "Read address from environment",
			setup: func() (*sql.DB, sqlmock.Sqlmock) {
				os.Setenv(usernameKey, "username")
				os.Setenv(passwordKey, "password")
				os.Setenv(urlKey, "tcp://localhost:9000")
				db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual), sqlmock.MonitorPingsOption(true))
				if err != nil {
					t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
				}
				openSql = func(driverName, dataSourceName string) (*sql.DB, error) {
					assert.Equal(t, driverName, "clickhouse")
					assert.Equal(t, dataSourceName, "tcp://localhost:9000?debug=false&username=username&password=password")
					return db, nil
				}
				mock.ExpectPing()
				return db, mock
			},
			cleanup: func() {
				os.Unsetenv(usernameKey)
				os.Unsetenv(passwordKey)
				os.Unsetenv(urlKey)
			},
		},
		{
			name: "Get address from K8s client",
			setup: func() (*sql.DB, sqlmock.Sqlmock) {
				fakeClientset := fake.NewSimpleClientset()
				db, mock := CreateFakeClickHouse(t, fakeClientset, testNamespace)
				createK8sClient = func() (client kubernetes.Interface, err error) {
					return fakeClientset, nil
				}
				return db, mock
			},
		},
		{
			name: "Service not exists",
			setup: func() (*sql.DB, sqlmock.Sqlmock) {
				fakeClientset := fake.NewSimpleClientset()
				createK8sClient = func() (client kubernetes.Interface, err error) {
					return fakeClientset, nil
				}
				return nil, nil
			},
			expectedErrorMsg: fmt.Sprintf("failed to get ClickHouse URL: error when getting the ClickHouse Service address: error when finding the Service %s: services \"%s\" not found", ServiceName, ServiceName),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := tc.setup()
			_, err := SetupConnection(nil)
			if tc.expectedErrorMsg != "" {
				assert.ErrorContains(t, err, tc.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
			}
			if mock != nil {
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Errorf("there were unfulfilled expectations: %s", err)
				}
			}
			if tc.cleanup != nil {
				tc.cleanup()
			}
			if db != nil {
				db.Close()
			}
		})
	}

}
