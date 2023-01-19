// Copyright 2023 Antrea Authors
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
	"database/sql"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	"antrea.io/theia/pkg/querier"
	"antrea.io/theia/pkg/util/clickhouse"
)

const (
	defaultNameSpace     = "flow-visibility"
	tadQuery         int = iota
)

// REST implements rest.Storage for anomalydetector.
type REST struct {
	ThroughputAnomalyDetectorQuerier querier.ThroughputAnomalyDetectorQuerier
	clickhouseConnect                *sql.DB
}

var (
	_ rest.Creater         = &REST{}
	_ rest.Getter          = &REST{}
	_ rest.Lister          = &REST{}
	_ rest.Creater         = &REST{}
	_ rest.GracefulDeleter = &REST{}

	setupClickHouseConnection = clickhouse.SetupConnection
)

var queryMap = map[int]string{
	tadQuery: `
	SELECT 
		id,
		sourceIP,
		sourceTransportPort,
		destinationIP,
		destinationTransportPort,
		flowStartSeconds,
		flowEndSeconds,
		throughput,
		algoCalc,
		anomaly
	FROM tadetector WHERE id = (?);`,
}

// NewREST returns a REST object that will work against API services.
func NewREST(tadq querier.ThroughputAnomalyDetectorQuerier) *REST {
	return &REST{ThroughputAnomalyDetectorQuerier: tadq}
}

func (r *REST) New() runtime.Object {
	return &v1alpha1.ThroughputAnomalyDetector{}
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	tad, err := r.ThroughputAnomalyDetectorQuerier.GetThroughputAnomalyDetector(defaultNameSpace, name)
	if err != nil {
		return nil, errors.NewNotFound(v1alpha1.Resource("throughputanomalydetectors"), name)
	}
	newTAD := new(v1alpha1.ThroughputAnomalyDetector)
	r.copyThroughputAnomalyDetector(newTAD, tad)
	// Try to retrieve result from ClickHouse in case TAD is completed
	if tad.Status.State == crdv1alpha1.ThroughputAnomalyDetectorStateCompleted {
		err := r.getTADetectorResult(tad.Status.SparkApplication, newTAD)
		if err != nil {
			newTAD.Status.ErrorMsg = fmt.Sprintf("Failed to get the result for completed Throughput Anomaly Detector with id %s, error: %v", tad.Status.SparkApplication, err)
		}
	}
	return newTAD, nil
}

// copyThroughputAnomalyDetector is used to copy ThroughputAnomalyDetector from crd to anomalydetector
func (r *REST) copyThroughputAnomalyDetector(tad *v1alpha1.ThroughputAnomalyDetector, crd *crdv1alpha1.ThroughputAnomalyDetector) error {
	tad.Name = crd.Name
	tad.Type = crd.Spec.JobType
	tad.StartInterval = crd.Spec.StartInterval
	tad.EndInterval = crd.Spec.EndInterval
	tad.ExecutorInstances = crd.Spec.ExecutorInstances
	tad.DriverCoreRequest = crd.Spec.DriverCoreRequest
	tad.DriverMemory = crd.Spec.DriverMemory
	tad.ExecutorCoreRequest = crd.Spec.ExecutorCoreRequest
	tad.ExecutorMemory = crd.Spec.ExecutorMemory
	tad.Status.State = crd.Status.State
	tad.Status.SparkApplication = crd.Status.SparkApplication
	tad.Status.CompletedStages = crd.Status.CompletedStages
	tad.Status.TotalStages = crd.Status.TotalStages
	tad.Status.ErrorMsg = crd.Status.ErrorMsg
	tad.Status.StartTime = crd.Status.StartTime
	tad.Status.EndTime = crd.Status.EndTime
	return nil
}

func (r *REST) NewList() runtime.Object {
	return &v1alpha1.ThroughputAnomalyDetectorList{}
}

func (r *REST) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	tadList, err := r.ThroughputAnomalyDetectorQuerier.ListThroughputAnomalyDetector(defaultNameSpace)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("error when getting ThroughputAnomalyDetectorsList: %v", err))
	}
	items := make([]v1alpha1.ThroughputAnomalyDetector, 0, len(tadList))
	for _, tad := range tadList {
		newTAD := new(v1alpha1.ThroughputAnomalyDetector)
		r.copyThroughputAnomalyDetector(newTAD, tad)
		// Try to retrieve result from ClickHouse in case TAD is completed
		if tad.Status.State == crdv1alpha1.ThroughputAnomalyDetectorStateCompleted {
			err := r.getTADetectorResult(tad.Status.SparkApplication, newTAD)
			if err != nil {
				newTAD.Status.ErrorMsg += fmt.Sprintf("Failed to get the result for Throughput Anomaly Detector with id %s, error: %v", tad.Status.SparkApplication, err)
			}
		}
		items = append(items, *newTAD)
	}
	list := &v1alpha1.ThroughputAnomalyDetectorList{Items: items}
	return list, nil
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return rest.NewDefaultTableConvertor(v1alpha1.Resource("throughputanomalydetectors")).ConvertToTable(ctx, obj, tableOptions)
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	newTAD, ok := obj.(*v1alpha1.ThroughputAnomalyDetector)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a ThroughputAnomalyDetector object: %T", obj))
	}
	existTAD, _ := r.ThroughputAnomalyDetectorQuerier.GetThroughputAnomalyDetector(defaultNameSpace, newTAD.Name)
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
	_, err := r.ThroughputAnomalyDetectorQuerier.CreateThroughputAnomalyDetector(defaultNameSpace, job)
	if err != nil {
		return nil, errors.NewBadRequest(fmt.Sprintf("error when creating ThroughputAnomalyDetection job: %+v, err: %v", job, err))
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}

func (r *REST) getTADetectorResult(id string, tad *v1alpha1.ThroughputAnomalyDetector) error {
	var err error
	if r.clickhouseConnect == nil {
		r.clickhouseConnect, err = setupClickHouseConnection(nil)
		if err != nil {
			return err
		}
	}
	rows, err := r.clickhouseConnect.Query(queryMap[tadQuery], id)
	if err != nil {
		return fmt.Errorf("failed to get Throughput Anomaly Detector results with id %s: %v", id, err)
	}
	defer rows.Close()
	for rows.Next() {
		res := v1alpha1.ThroughputAnomalyDetectorStats{}
		err := rows.Scan(&res.Id, &res.SourceIP, &res.SourceTransportPort, &res.DestinationIP, &res.DestinationTransportPort, &res.FlowStartSeconds, &res.FlowEndSeconds, &res.Throughput, &res.AlgoCalc, &res.Anomaly)
		if err != nil {
			return fmt.Errorf("failed to scan Throughput Anomaly Detector results: %v", err)
		}
		tad.Stats = append(tad.Stats, res)
	}
	return nil
}

func (r *REST) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	_, err := r.ThroughputAnomalyDetectorQuerier.GetThroughputAnomalyDetector(defaultNameSpace, name)
	if err != nil {
		return nil, false, errors.NewBadRequest(fmt.Sprintf("ThroughputAnomalyDetector job doesn't exist, name: %s", name))
	}
	err = r.ThroughputAnomalyDetectorQuerier.DeleteThroughputAnomalyDetector(defaultNameSpace, name)
	if err != nil {
		return nil, false, err
	}
	return &metav1.Status{Status: metav1.StatusSuccess}, false, nil
}
