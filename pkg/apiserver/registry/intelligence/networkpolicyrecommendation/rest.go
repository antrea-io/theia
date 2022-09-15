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

package networkpolicyrecommendation

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/querier"
)

// REST implements rest.Storage for NetworkPolicyRecommendation.
type REST struct {
	npRecommendationQuerier querier.NPRecommendationQuerier
}

var (
	_ rest.Scoper = &REST{}
	_ rest.Getter = &REST{}
	_ rest.Lister = &REST{}
)

// NewREST returns a REST object that will work against API services.
func NewREST(nprq querier.NPRecommendationQuerier) *REST {
	return &REST{npRecommendationQuerier: nprq}
}

func (r *REST) New() runtime.Object {
	return &intelligence.NetworkPolicyRecommendation{}
}

func (r *REST) getNetworkPolicyRecommendation(name string) *intelligence.NetworkPolicyRecommendation {
	npReco, err := r.npRecommendationQuerier.GetNetworkPolicyRecommendation("flow-visibility", name)
	if err != nil {
		return nil
	}

	job := new(intelligence.NetworkPolicyRecommendation)
	job.Name = npReco.Name
	job.Type = npReco.Spec.JobType
	job.Limit = npReco.Spec.Limit
	job.PolicyType = npReco.Spec.PolicyType
	job.StartInterval = npReco.Spec.StartInterval
	job.EndInterval = npReco.Spec.EndInterval
	job.NSAllowList = npReco.Spec.NSAllowList
	job.ExcludeLabels = npReco.Spec.ExcludeLabels
	job.ToServices = npReco.Spec.ToServices
	job.ExecutorInstances = npReco.Spec.ExecutorInstances
	job.DriverCoreRequest = npReco.Spec.DriverCoreRequest
	job.DriverMemory = npReco.Spec.DriverMemory
	job.ExecutorCoreRequest = npReco.Spec.ExecutorCoreRequest
	job.ExecutorMemory = npReco.Spec.ExecutorMemory
	return job
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	job := r.getNetworkPolicyRecommendation(name)
	if job == nil {
		return nil, errors.NewNotFound(intelligence.Resource("networkpolicyrecommendations"), name)
	}
	return job, nil
}

func (r *REST) NewList() runtime.Object {
	return &intelligence.NetworkPolicyRecommendationList{}
}

func (r *REST) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	list := new(intelligence.NetworkPolicyRecommendationList)
	return list, nil
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(intelligence.Resource("networkpolicyrecommendations")).ConvertToTable(ctx, obj, tableOptions)
}
