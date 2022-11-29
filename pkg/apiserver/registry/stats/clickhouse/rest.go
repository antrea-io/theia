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
	var status v1alpha1.ClickHouseStats
	var err error
	switch name {
	case "diskInfo":
		err = r.clickHouseStatusQuerier.GetDiskInfo(defaultNameSpace, &status)
		if status.DiskInfos == nil {
			return nil, fmt.Errorf("no diskInfo data is returned by database")
		}
	case "tableInfo":
		err = r.clickHouseStatusQuerier.GetTableInfo(defaultNameSpace, &status)
		if status.TableInfos == nil {
			return nil, fmt.Errorf("no tableInfo data is returned by database")
		}
	case "insertRate":
		err = r.clickHouseStatusQuerier.GetInsertRate(defaultNameSpace, &status)
		if status.InsertRates == nil {
			return nil, fmt.Errorf("no insertRate data is returned by database")
		}
	case "stackTrace":
		err = r.clickHouseStatusQuerier.GetStackTrace(defaultNameSpace, &status)
		if status.StackTraces == nil {
			return nil, fmt.Errorf("no stackTrace data is returned by database")
		}
	default:
		return nil, fmt.Errorf("cannot recognize the statua name: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("error when sending query to ClickHouse: %s", err)
	}
	return &status, nil
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(stats.Resource("clickhouse")).ConvertToTable(ctx, obj, tableOptions)
}
