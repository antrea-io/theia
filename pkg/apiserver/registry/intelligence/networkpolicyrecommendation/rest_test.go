// Copyright 2020 Antrea Authors
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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	intelligence "antrea.io/theia/pkg/apis/intelligence/v1alpha1"
)

type fakeQuerier struct{}

func TestREST_Get(t *testing.T) {
	tests := []struct {
		name         string
		nprName      string
		expectErr    error
		expectResult *intelligence.NetworkPolicyRecommendation
	}{
		{
			name:         "Not Found case",
			nprName:      "non-existent-npr",
			expectErr:    errors.NewNotFound(intelligence.Resource("networkpolicyrecommendations"), "non-existent-npr"),
			expectResult: nil,
		},
		{
			name:         "Successful Get case",
			nprName:      "npr-2",
			expectErr:    nil,
			expectResult: &intelligence.NetworkPolicyRecommendation{Type: "NPR", PolicyType: "Allow"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			npr, err := r.Get(context.TODO(), tt.nprName, &v1.GetOptions{})
			assert.Equal(t, err, tt.expectErr)
			if npr != nil {
				assert.Equal(t, tt.expectResult, npr.(*intelligence.NetworkPolicyRecommendation))
			} else {
				assert.Nil(t, tt.expectResult)
			}
		})
	}
}

func TestREST_Delete(t *testing.T) {
	tests := []struct {
		name      string
		nprName   string
		expectErr error
	}{
		{
			name:      "Job doesn't exist case",
			nprName:   "non-existent-npr",
			expectErr: errors.NewBadRequest(fmt.Sprintf("NetworkPolicyRecommendation job doesn't exist, name: %s", "non-existent-npr")),
		},
		{
			name:      "Successful Delete case",
			nprName:   "npr-2",
			expectErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			_, _, err := r.Delete(context.TODO(), tt.nprName, nil, &v1.DeleteOptions{})
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
			obj:          &crdv1alpha1.NetworkPolicyRecommendation{},
			expectErr:    errors.NewBadRequest(fmt.Sprintf("not a NetworkPolicyRecommendation object: %T", &crdv1alpha1.NetworkPolicyRecommendation{})),
			expectResult: nil,
		},
		{
			name: "Job already exists case",
			obj: &intelligence.NetworkPolicyRecommendation{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{Name: "existent-npr"},
			},
			expectErr:    errors.NewBadRequest(fmt.Sprintf("networkPolicyRecommendation job exists, name: %s", "existent-npr")),
			expectResult: nil,
		},
		{
			name: "Successful Create case",
			obj: &intelligence.NetworkPolicyRecommendation{
				TypeMeta:   v1.TypeMeta{},
				ObjectMeta: v1.ObjectMeta{Name: "non-existent-npr"},
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
		expectResult []intelligence.NetworkPolicyRecommendation
	}{
		{
			name: "Successful List case",
			expectResult: []intelligence.NetworkPolicyRecommendation{
				{ObjectMeta: v1.ObjectMeta{Name: "npr-1"}},
				{ObjectMeta: v1.ObjectMeta{Name: "npr-2"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewREST(&fakeQuerier{})
			itemList, err := r.List(context.TODO(), &internalversion.ListOptions{})
			assert.NoError(t, err)
			nprList, ok := itemList.(*intelligence.NetworkPolicyRecommendationList)
			assert.True(t, ok)
			assert.ElementsMatch(t, tt.expectResult, nprList.Items)
		})
	}
}

func (c *fakeQuerier) GetNetworkPolicyRecommendation(namespace, name string) (*crdv1alpha1.NetworkPolicyRecommendation, error) {
	if name == "non-existent-npr" {
		return nil, fmt.Errorf("not found")
	}
	return &crdv1alpha1.NetworkPolicyRecommendation{
		Spec: crdv1alpha1.NetworkPolicyRecommendationSpec{
			JobType: "NPR", PolicyType: "Allow"},
		Status: crdv1alpha1.NetworkPolicyRecommendationStatus{
			RecommendedNP: &crdv1alpha1.RecommendedNetworkPolicy{},
		},
	}, nil
}

func (c *fakeQuerier) CreateNetworkPolicyRecommendation(namespace string, networkPolicyRecommendation *crdv1alpha1.NetworkPolicyRecommendation) (*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return nil, nil
}

func (c *fakeQuerier) DeleteNetworkPolicyRecommendation(namespace, name string) error {
	return nil
}

func (c *fakeQuerier) ListNetworkPolicyRecommendation(namespace string) ([]*crdv1alpha1.NetworkPolicyRecommendation, error) {
	return []*crdv1alpha1.NetworkPolicyRecommendation{
		{ObjectMeta: v1.ObjectMeta{Name: "npr-1"}, Status: crdv1alpha1.NetworkPolicyRecommendationStatus{RecommendedNP: &crdv1alpha1.RecommendedNetworkPolicy{}}},
		{ObjectMeta: v1.ObjectMeta{Name: "npr-2"}, Status: crdv1alpha1.NetworkPolicyRecommendationStatus{RecommendedNP: &crdv1alpha1.RecommendedNetworkPolicy{}}},
	}, nil
}
