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

package anomalydetector

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/querier"
)

const defaultNameSpace = "flow-visibility"

// REST implements rest.Storage for anomalydetector.
type REST struct {
	ThroughputAnomalyDetectorQuerier querier.ThroughputAnomalyDetectorQuerier
}

var (
	_ rest.Creater = &REST{}
)

// NewREST returns a REST object that will work against API services.
func NewREST(tadq querier.ThroughputAnomalyDetectorQuerier) *REST {
	return &REST{ThroughputAnomalyDetectorQuerier: tadq}
}

func (r *REST) New() runtime.Object {
	return &v1alpha1.ThroughputAnomalyDetector{}
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(v1alpha1.Resource("anomalydetectors")).ConvertToTable(ctx, obj, tableOptions)
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	newTAD, ok := obj.(*v1alpha1.ThroughputAnomalyDetector)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a NetworkPolicyRecommendation object: %T", obj))
	}
	existTAD, _ := r.ThroughputAnomalyDetectorQuerier.GetThroughputAnomalyDetection(defaultNameSpace, newTAD.Name)
	if existTAD != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("ThroughputAnomalyDetection job exists, name: %s", newTAD.Name))
	}
	job := new(crdv1alpha1.ThroughputAnomalyDetector)
	job.Name = newTAD.Name
	job.Spec.JobType = newTAD.Type
	job.Spec.StartInterval = newTAD.StartInterval
	job.Spec.EndInterval = newTAD.EndInterval
	job.Spec.ExecutorInstances = newTAD.ExecutorInstances
	job.Spec.DriverCoreRequest = newTAD.DriverCoreRequest
	job.Spec.DriverMemory = newTAD.DriverMemory
	job.Spec.ExecutorCoreRequest = newTAD.ExecutorCoreRequest
	job.Spec.ExecutorMemory = newTAD.ExecutorMemory
	_, err := r.ThroughputAnomalyDetectorQuerier.CreateThroughputAnomalyDetection(defaultNameSpace, job)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("error when creating ThroughputAnomalyDetection CR: %v", err))
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}
