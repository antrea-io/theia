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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClickHouseStats struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DiskInfos   []DiskInfo   `json:"diskInfos,omitempty"`
	TableInfos  []TableInfo  `json:"tableInfos,omitempty"`
	InsertRates []InsertRate `json:"insertRates,omitempty"`
	StackTraces []StackTrace `json:"stackTraces,omitempty"`
	ErrorMsg    []string     `json:"errorMsg,omitempty"`
}

type DiskInfo struct {
	Shard          string `json:"shard,omitempty"`
	Database       string `json:"name,omitempty"`
	Path           string `json:"path,omitempty"`
	FreeSpace      string `json:"freeSpace,omitempty"`
	TotalSpace     string `json:"totalSpace,omitempty"`
	UsedPercentage string `json:"usedPercentage,omitempty"`
}

type TableInfo struct {
	Shard      string `json:"shard,omitempty"`
	Database   string `json:"database,omitempty"`
	TableName  string `json:"tableName,omitempty"`
	TotalRows  string `json:"totalRows,omitempty"`
	TotalBytes string `json:"totalBytes,omitempty"`
	TotalCols  string `json:"totalCols,omitempty"`
}

type InsertRate struct {
	Shard       string `json:"shard,omitempty"`
	RowsPerSec  string `json:"rowsPerSec,omitempty"`
	BytesPerSec string `json:"bytesPerSec,omitempty"`
}

type StackTrace struct {
	Shard          string `json:"shard,omitempty"`
	TraceFunctions string `json:"traceFunctions,omitempty"`
	Count          string `json:"count,omitempty"`
}
