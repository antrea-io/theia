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
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	anomalydetector "antrea.io/theia/pkg/apis/anomalydetector/v1alpha1"
	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
)

type fakeQuerier struct{}

func TestREST_Get(t *testing.T) {

	tests := []struct {
		name         string
		tadName      string
		expectErr    error
		expectResult *anomalydetector.ThroughputAnomalyDetector
	}{
		{
			name:         "Not Found case",
			tadName:      "non-existent-tad",
			expectErr:    errors.NewNotFound(anomalydetector.Resource("throughputanomalydetectors"), "non-existent-tad"),
			expectResult: nil,
		},
		{
			name:      "Successful Get case",
			tadName:   "tad-2",
			expectErr: nil,
			expectResult: &anomalydetector.ThroughputAnomalyDetector{
				Type: "TAD",
				Status: anomalydetector.ThroughputAnomalyDetectorStatus{
					State: crdv1alpha1.ThroughputAnomalyDetectorStateCompleted,
				},
				Stats: []anomalydetector.ThroughputAnomalyDetectorStats{{
					Id:                       "mock_Id",
					SourceIP:                 "mock_SourceIP",
					SourceTransportPort:      "mock_SourceTransportPort",
					DestinationIP:            "mock_DestinationIP",
					DestinationTransportPort: "mock_DestinationTransportPort",
					FlowStartSeconds:         "mock_FlowStartSeconds",
					FlowEndSeconds:           "mock_FlowEndSeconds",
					Throughput:               "mock_Throughput",
					AlgoCalc:                 "mock_AlgoCalc",
					Anomaly:                  "mock_Anomaly",
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()
			resultRows := sqlmock.NewRows([]string{
				"Id", "SourceIP", "SourceTransportPort", "DestinationIP", "DestinationTransportPort", "FlowStartSeconds", "FlowEndSeconds", "Throughput", "AlgoCalc", "Anomaly"}).
				AddRow("mock_Id", "mock_SourceIP", "mock_SourceTransportPort", "mock_DestinationIP", "mock_DestinationTransportPort", "mock_FlowStartSeconds", "mock_FlowEndSeconds", "mock_Throughput", "mock_AlgoCalc", "mock_Anomaly")
			mock.ExpectQuery("SELECT id, sourceIP, sourceTransportPort, destinationIP, destinationTransportPort, flowStartSeconds, flowEndSeconds, throughput, algoCalc, anomaly FROM tadetector WHERE id = (?);").WillReturnRows(resultRows)

			setupClickHouseConnection = func(client kubernetes.Interface) (connect *sql.DB, err error) {
				return db, nil
			}
			r := NewREST(&fakeQuerier{})
			tad, err := r.Get(context.TODO(), tt.tadName, &v1.GetOptions{})
			assert.Equal(t, err, tt.expectErr)
			if tad != nil {
				assert.Equal(t, tt.expectResult, tad.(*anomalydetector.ThroughputAnomalyDetector))
			} else {
				assert.Nil(t, tt.expectResult)
			}
		})
	}
}

func TestREST_Delete(t *testing.T) {
	tests := []struct {
		name      string
		tadName   string
		expectErr error
	}{
		{
			name:      "Job doesn't exist case",
			tadName:   "non-existent-tad",
			expectErr: errors.NewBadRequest(fmt.Sprintf("ThroughputAnomalyDetector job doesn't exist, name: %s", "non-existent-tad")),
		},
		{
			name:      "Successful Delete case",
			tadName:   "tad-2",
			expectErr: nil,
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			_, _, err := r.Delete(context.TODO(), tt.tadName, nil, &v1.DeleteOptions{})
			assert.Equal(t, err, tt.expectErr)
		})
	}
}

func TestREST_Create(t *testing.T) {
	tests := []struct {
		name         string
		obj          runtime.Object
		expectErr    error
		expectResult runtime.Object
	}{
		{
			name:         "Wrong object case",
			obj:          &crdv1alpha1.ThroughputAnomalyDetector{},
			expectErr:    errors.NewBadRequest(fmt.Sprintf("not a ThroughputAnomalyDetector object: %T", &crdv1alpha1.ThroughputAnomalyDetector{})),
			expectResult: nil,
		},
		{
			name: "Job already exists case",
			obj: &anomalydetector.ThroughputAnomalyDetector{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{Name: "existent-tad"},
			},
			expectErr:    errors.NewBadRequest(fmt.Sprintf("ThroughputAnomalyDetection job exists, name: %s", "existent-tad")),
			expectResult: nil,
		},
		{
			name: "Successful Create case",
			obj: &anomalydetector.ThroughputAnomalyDetector{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{Name: "non-existent-tad"},
			},
			expectErr:    nil,
			expectResult: &v1.Status{Status: v1.StatusSuccess},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			result, err := r.Create(context.TODO(), tt.obj, nil, &v1.CreateOptions{})
			assert.Equal(t, err, tt.expectErr)
			assert.Equal(t, tt.expectResult, result)
		})
	}
}

func TestREST_List(t *testing.T) {
	tests := []struct {
		name         string
		expectResult []anomalydetector.ThroughputAnomalyDetector
	}{
		{
			name: "Successful List case",
			expectResult: []anomalydetector.ThroughputAnomalyDetector{
				{ObjectMeta: v1.ObjectMeta{Name: "tad-1"}},
				{ObjectMeta: v1.ObjectMeta{Name: "tad-2"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			itemList, err := r.List(context.TODO(), &internalversion.ListOptions{})
			assert.NoError(t, err)
			tadList, ok := itemList.(*anomalydetector.ThroughputAnomalyDetectorList)
			assert.True(t, ok)
			assert.ElementsMatch(t, tt.expectResult, tadList.Items)
		})
	}
}

func (c *fakeQuerier) GetThroughputAnomalyDetector(namespace, name string) (*crdv1alpha1.ThroughputAnomalyDetector, error) {
	if name == "non-existent-tad" {
		return nil, fmt.Errorf("not found")
	}
	return &crdv1alpha1.ThroughputAnomalyDetector{
		Spec: crdv1alpha1.ThroughputAnomalyDetectorSpec{
			JobType: "TAD",
		},
		Status: crdv1alpha1.ThroughputAnomalyDetectorStatus{
			State: crdv1alpha1.ThroughputAnomalyDetectorStateCompleted,
		},
	}, nil
}

func (c *fakeQuerier) CreateThroughputAnomalyDetector(namespace string, throughputAnomalyDetection *crdv1alpha1.ThroughputAnomalyDetector) (*crdv1alpha1.ThroughputAnomalyDetector, error) {
	return nil, nil
}

func (c *fakeQuerier) DeleteThroughputAnomalyDetector(namespace, name string) error {
	return nil
}

func (c *fakeQuerier) ListThroughputAnomalyDetector(namespace string) ([]*crdv1alpha1.ThroughputAnomalyDetector, error) {
	return []*crdv1alpha1.ThroughputAnomalyDetector{
		{ObjectMeta: v1.ObjectMeta{Name: "tad-1"}},
		{ObjectMeta: v1.ObjectMeta{Name: "tad-2"}},
	}, nil
}
