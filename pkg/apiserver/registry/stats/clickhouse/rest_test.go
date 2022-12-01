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

package clickhouse

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	stats "antrea.io/theia/pkg/apis/stats/v1alpha1"
)

type fakeQuerier struct{}

func TestREST_Get(t *testing.T) {
	tests := []struct {
		name         string
		queryName    string
		expectErr    error
		expectResult *stats.ClickHouseStats
	}{
		{
			name:      "Get diskInfo",
			queryName: "diskInfo",
			expectErr: nil,
			expectResult: &stats.ClickHouseStats{
				DiskInfos: []stats.DiskInfo{{
					Shard: "Shard_test",
				}},
			},
		},
		{
			name:      "Get tableInfo",
			queryName: "tableInfo",
			expectErr: nil,
			expectResult: &stats.ClickHouseStats{
				TableInfos: []stats.TableInfo{{
					Shard: "Shard_test",
				}},
			},
		},
		{
			name:      "Get insertRate",
			queryName: "insertRate",
			expectErr: nil,
			expectResult: &stats.ClickHouseStats{
				InsertRates: []stats.InsertRate{{
					Shard: "Shard_test",
				}},
			},
		},
		{
			name:      "Get stackTraces",
			queryName: "stackTrace",
			expectErr: nil,
			expectResult: &stats.ClickHouseStats{
				StackTraces: []stats.StackTrace{{
					Shard: "Shard_test",
				}},
			},
		},
		{
			name:         "not found",
			queryName:    "notFound",
			expectErr:    fmt.Errorf("cannot recognize the statua name: notFound"),
			expectResult: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			result, err := r.Get(context.TODO(), tt.queryName, &v1.GetOptions{})
			if tt.expectErr == nil {
				assert.NoError(t, err)
				status, ok := result.(*stats.ClickHouseStats)
				assert.True(t, ok)
				assert.EqualValues(t, tt.expectResult, status)

			} else {
				assert.Error(t, err)
				assert.Equal(t, err, tt.expectErr)
			}
		})
	}
}

func (c *fakeQuerier) GetDiskInfo(namespace string, status *stats.ClickHouseStats) error {
	status.DiskInfos = []stats.DiskInfo{{
		Shard: "Shard_test",
	}}
	return nil
}
func (c *fakeQuerier) GetTableInfo(namespace string, status *stats.ClickHouseStats) error {
	status.TableInfos = []stats.TableInfo{{
		Shard: "Shard_test",
	}}
	return nil
}
func (c *fakeQuerier) GetInsertRate(namespace string, status *stats.ClickHouseStats) error {
	status.InsertRates = []stats.InsertRate{{
		Shard: "Shard_test",
	}}
	return nil
}
func (c *fakeQuerier) GetStackTrace(namespace string, status *stats.ClickHouseStats) error {
	status.StackTraces = []stats.StackTrace{{
		Shard: "Shard_test",
	}}
	return nil
}
