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

package querier

import (
	"antrea.io/theia/pkg/apis/crd/v1alpha1"
	statsV1 "antrea.io/theia/pkg/apis/stats/v1alpha1"
)

type NPRecommendationQuerier interface {
	GetNetworkPolicyRecommendation(namespace, name string) (*v1alpha1.NetworkPolicyRecommendation, error)
	ListNetworkPolicyRecommendation(namespace string) ([]*v1alpha1.NetworkPolicyRecommendation, error)
	DeleteNetworkPolicyRecommendation(namespace, name string) error
	CreateNetworkPolicyRecommendation(namespace string, networkPolicyRecommendation *v1alpha1.NetworkPolicyRecommendation) (*v1alpha1.NetworkPolicyRecommendation, error)
}

type ClickHouseStatQuerier interface {
	GetDiskInfo(namespace string, stats *statsV1.ClickHouseStats) error
	GetTableInfo(namespace string, stats *statsV1.ClickHouseStats) error
	GetInsertRate(namespace string, stats *statsV1.ClickHouseStats) error
	GetStackTrace(namespace string, stats *statsV1.ClickHouseStats) error
}

type ThroughputAnomalyDetectorQuerier interface {
	GetThroughputAnomalyDetector(namespace, name string) (*v1alpha1.ThroughputAnomalyDetector, error)
	ListThroughputAnomalyDetector(namespace string) ([]*v1alpha1.ThroughputAnomalyDetector, error)
	DeleteThroughputAnomalyDetector(namespace, name string) error
	CreateThroughputAnomalyDetector(namespace string, anomalydetector *v1alpha1.ThroughputAnomalyDetector) (*v1alpha1.ThroughputAnomalyDetector, error)
}
