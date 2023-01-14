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
	"database/sql"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
	"antrea.io/theia/pkg/querier"
	"antrea.io/theia/pkg/util/clickhouse"
)

// REST implements rest.Storage for NetworkPolicyRecommendation.
type REST struct {
	npRecommendationQuerier querier.NPRecommendationQuerier
	clickhouseConnect       *sql.DB
}

var (
	_ rest.Scoper          = &REST{}
	_ rest.Getter          = &REST{}
	_ rest.Lister          = &REST{}
	_ rest.Creater         = &REST{}
	_ rest.GracefulDeleter = &REST{}

	setupClickHouseConnection = clickhouse.SetupConnection
)

const (
	defaultNameSpace = "flow-visibility"
)

// NewREST returns a REST object that will work against API services.
func NewREST(nprq querier.NPRecommendationQuerier) *REST {
	return &REST{npRecommendationQuerier: nprq}
}

func (r *REST) New() runtime.Object {
	return &intelligence.NetworkPolicyRecommendation{}
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	npReco, err := r.npRecommendationQuerier.GetNetworkPolicyRecommendation(defaultNameSpace, name)
	if err != nil {
		return nil, errors.NewNotFound(intelligence.Resource("networkpolicyrecommendations"), name)
	}
	intelliNPR := new(intelligence.NetworkPolicyRecommendation)
	r.copyNetworkPolicyRecommendation(intelliNPR, npReco)
	// Try to retrieve result from ClickHouse in case NPR is completed
	if npReco.Status.State == crdv1alpha1.NPRecommendationStateCompleted {
		result, err := r.getRecommendationResult(npReco.Status.SparkApplication)
		if err != nil {
			intelliNPR.Status.ErrorMsg = fmt.Sprintf("Failed to get the result for completed NetworkPolicy Recommedation with id %s, error: %v", npReco.Status.SparkApplication, err)
		} else {
			intelliNPR.Status.RecommendationOutcome = result
		}
	}
	return intelliNPR, nil
}

func (r *REST) NewList() runtime.Object {
	return &intelligence.NetworkPolicyRecommendationList{}
}

func (r *REST) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	npRecoList, err := r.npRecommendationQuerier.ListNetworkPolicyRecommendation(defaultNameSpace)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("error when getting NetworkPolicyRecommendationsList: %v", err))
	}
	items := make([]intelligence.NetworkPolicyRecommendation, 0, len(npRecoList))
	for _, npReco := range npRecoList {
		intelliNPR := new(intelligence.NetworkPolicyRecommendation)
		r.copyNetworkPolicyRecommendation(intelliNPR, npReco)
		// Try to retrieve result from ClickHouse in case NPR is completed
		if npReco.Status.State == crdv1alpha1.NPRecommendationStateCompleted {
			result, err := r.getRecommendationResult(npReco.Status.SparkApplication)
			if err != nil {
				intelliNPR.Status.ErrorMsg += fmt.Sprintf("Failed to get the result for completed NetworkPolicy Recommedation with id %s, error: %v", npReco.Status.SparkApplication, err)
			} else {
				intelliNPR.Status.RecommendationOutcome = result
			}
		}
		items = append(items, *intelliNPR)
	}
	list := &intelligence.NetworkPolicyRecommendationList{Items: items}
	return list, nil
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(intelligence.Resource("networkpolicyrecommendations")).ConvertToTable(ctx, obj, tableOptions)
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	npReco, ok := obj.(*intelligence.NetworkPolicyRecommendation)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a NetworkPolicyRecommendation object: %T", obj))
	}
	existNPReco, _ := r.npRecommendationQuerier.GetNetworkPolicyRecommendation(defaultNameSpace, npReco.Name)
	if existNPReco != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("networkPolicyRecommendation job exists, name: %s", npReco.Name))
	}
	job := new(crdv1alpha1.NetworkPolicyRecommendation)
	job.Name = npReco.Name
	job.Spec.JobType = npReco.Type
	job.Spec.Limit = npReco.Limit
	job.Spec.PolicyType = npReco.PolicyType
	job.Spec.StartInterval = npReco.StartInterval
	job.Spec.EndInterval = npReco.EndInterval
	job.Spec.NSAllowList = npReco.NSAllowList
	job.Spec.ExcludeLabels = npReco.ExcludeLabels
	job.Spec.ToServices = npReco.ToServices
	job.Spec.ExecutorInstances = npReco.ExecutorInstances
	job.Spec.DriverCoreRequest = npReco.DriverCoreRequest
	job.Spec.DriverMemory = npReco.DriverMemory
	job.Spec.ExecutorCoreRequest = npReco.ExecutorCoreRequest
	job.Spec.ExecutorMemory = npReco.ExecutorMemory
	_, err := r.npRecommendationQuerier.CreateNetworkPolicyRecommendation(defaultNameSpace, job)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("error when creating NetworkPolicyRecommendation CR: %v", err))
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}

func (r *REST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	_, err := r.npRecommendationQuerier.GetNetworkPolicyRecommendation(defaultNameSpace, name)
	if err != nil {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("NetworkPolicyRecommendation job doesn't exist, name: %s", name))
	}
	err = r.npRecommendationQuerier.DeleteNetworkPolicyRecommendation(defaultNameSpace, name)
	if err != nil {
		return nil, false, err
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, false, nil
}

// copyNetworkPolicyRecommendation is used to copy NetworkPolicyRecommendation from crd to intelligence
func (r *REST) copyNetworkPolicyRecommendation(intelli *intelligence.NetworkPolicyRecommendation, crd *crdv1alpha1.NetworkPolicyRecommendation) error {
	intelli.Name = crd.Name
	intelli.Type = crd.Spec.JobType
	intelli.Limit = crd.Spec.Limit
	intelli.PolicyType = crd.Spec.PolicyType
	intelli.StartInterval = crd.Spec.StartInterval
	intelli.EndInterval = crd.Spec.EndInterval
	intelli.NSAllowList = crd.Spec.NSAllowList
	intelli.ExcludeLabels = crd.Spec.ExcludeLabels
	intelli.ToServices = crd.Spec.ToServices
	intelli.ExecutorInstances = crd.Spec.ExecutorInstances
	intelli.DriverCoreRequest = crd.Spec.DriverCoreRequest
	intelli.DriverMemory = crd.Spec.DriverMemory
	intelli.ExecutorCoreRequest = crd.Spec.ExecutorCoreRequest
	intelli.ExecutorMemory = crd.Spec.ExecutorMemory
	intelli.Status.State = crd.Status.State
	intelli.Status.SparkApplication = crd.Status.SparkApplication
	intelli.Status.CompletedStages = crd.Status.CompletedStages
	intelli.Status.TotalStages = crd.Status.TotalStages
	intelli.Status.ErrorMsg = crd.Status.ErrorMsg
	intelli.Status.StartTime = crd.Status.StartTime
	intelli.Status.EndTime = crd.Status.EndTime
	return nil
}

func (r *REST) getRecommendationResult(id string) (result string, err error) {
	if r.clickhouseConnect == nil {
		r.clickhouseConnect, err = setupClickHouseConnection(nil)
		if err != nil {
			return result, err
		}
	}
	query := "SELECT policy FROM recommendations WHERE id = (?);"
	rows, err := r.clickhouseConnect.Query(query, id)
	if err != nil {
		return result, fmt.Errorf("failed to get recommendation results with id %s: %v", id, err)
	}
	defer rows.Close()
	var policies []string
	for rows.Next() {
		var policyYaml string
		err := rows.Scan(&policyYaml)
		if err != nil {
			return result, fmt.Errorf("failed to scan recommendation results: %v", err)
		}
		policies = append(policies, policyYaml)
	}
	result = strings.Join(policies, "---\n")
	return result, nil
}
