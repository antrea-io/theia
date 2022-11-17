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
	"context"
	"fmt"

	"antrea.io/antrea/pkg/apis/stats"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"antrea.io/theia/pkg/apis/stats/v1alpha1"
	"antrea.io/theia/pkg/querier"
)

const defaultNameSpace = "flow-visibility"

// REST implements rest.Storage for clickhouse.
type REST struct {
	clickHouseStatusQuerier querier.ClickHouseStatQuerier
}

var (
	_ rest.Getter = &REST{}
)

// NewREST returns a REST object that will work against API services.
func NewREST(chq querier.ClickHouseStatQuerier) *REST {
	return &REST{clickHouseStatusQuerier: chq}
}

func (r *REST) New() runtime.Object {
	return &v1alpha1.ClickHouseStats{}
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	var stats [][]string
	var err error
	switch name {
	case "diskInfo":
		stats, err = r.clickHouseStatusQuerier.GetDiskInfo(defaultNameSpace)
	case "tableInfo":
		stats, err = r.clickHouseStatusQuerier.GetTableInfo(defaultNameSpace)
	case "insertRate":
		stats, err = r.clickHouseStatusQuerier.GetInsertRate(defaultNameSpace)
	case "stackTraces":
		stats, err = r.clickHouseStatusQuerier.GetStackTraces(defaultNameSpace)
	default:
		return nil, fmt.Errorf("cannot recognize the statua name: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("error when sending query to ClickHouse: %s", err)
	}
	return &v1alpha1.ClickHouseStats{Stat: stats}, nil
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(stats.Resource("clickhouse")).ConvertToTable(ctx, obj, tableOptions)
}
