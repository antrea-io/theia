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
)

type NPRecommendationQuerier interface {
	GetNetworkPolicyRecommendation(namespace, name string) (*v1alpha1.NetworkPolicyRecommendation, error)
	ListNetworkPolicyRecommendation(namespace string) ([]*v1alpha1.NetworkPolicyRecommendation, error)
	DeleteNetworkPolicyRecommendation(namespace, name string) error
	CreateNetworkPolicyRecommendation(namespace string, networkPolicyRecommendation *v1alpha1.NetworkPolicyRecommendation) (*v1alpha1.NetworkPolicyRecommendation, error)
}

type ClickHouseStatusQuerier interface {
	GetDiskInfo(namespace string) ([][]string, error)
	GetTableInfo(namespace string) ([][]string, error)
	GetInsertRate(namespace string) ([][]string, error)
	GetStackTraces(namespace string) ([][]string, error)
}
