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
		expectResult [][]string
	}{
		{
			name:         "Get diskInfo",
			queryName:    "diskInfo",
			expectErr:    nil,
			expectResult: [][]string{{"diskInfo_test"}},
		},
		{
			name:         "Get tableInfo",
			queryName:    "tableInfo",
			expectErr:    nil,
			expectResult: [][]string{{"tableInfo_test"}},
		},
		{
			name:         "Get insertRate",
			queryName:    "insertRate",
			expectErr:    nil,
			expectResult: [][]string{{"insertRate_test"}},
		},
		{
			name:         "Get stackTraces",
			queryName:    "stackTraces",
			expectErr:    nil,
			expectResult: [][]string{{"stackTraces_test"}},
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
				assert.ElementsMatch(t, tt.expectResult, status.Stat)

			} else {
				assert.Error(t, err)
				assert.Equal(t, err, tt.expectErr)
			}
		})
	}
}

func (c *fakeQuerier) GetDiskInfo(namespace string) ([][]string, error) {
	return [][]string{{"diskInfo_test"}}, nil
}
func (c *fakeQuerier) GetTableInfo(namespace string) ([][]string, error) {
	return [][]string{{"tableInfo_test"}}, nil
}
func (c *fakeQuerier) GetInsertRate(namespace string) ([][]string, error) {
	return [][]string{{"insertRate_test"}}, nil
}
func (c *fakeQuerier) GetStackTraces(namespace string) ([][]string, error) {
	return [][]string{{"stackTraces_test"}}, nil
}
