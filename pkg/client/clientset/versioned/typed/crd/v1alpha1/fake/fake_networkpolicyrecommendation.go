// Copyright 2022 Antrea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNetworkPolicyRecommendations implements NetworkPolicyRecommendationInterface
type FakeNetworkPolicyRecommendations struct {
	Fake *FakeCrdV1alpha1
	ns   string
}

var networkpolicyrecommendationsResource = schema.GroupVersionResource{Group: "crd.theia.antrea.io", Version: "v1alpha1", Resource: "networkpolicyrecommendations"}

var networkpolicyrecommendationsKind = schema.GroupVersionKind{Group: "crd.theia.antrea.io", Version: "v1alpha1", Kind: "NetworkPolicyRecommendation"}

// Get takes name of the networkPolicyRecommendation, and returns the corresponding networkPolicyRecommendation object, and an error if there is any.
func (c *FakeNetworkPolicyRecommendations) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.NetworkPolicyRecommendation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(networkpolicyrecommendationsResource, c.ns, name), &v1alpha1.NetworkPolicyRecommendation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkPolicyRecommendation), err
}

// List takes label and field selectors, and returns the list of NetworkPolicyRecommendations that match those selectors.
func (c *FakeNetworkPolicyRecommendations) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.NetworkPolicyRecommendationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(networkpolicyrecommendationsResource, networkpolicyrecommendationsKind, c.ns, opts), &v1alpha1.NetworkPolicyRecommendationList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.NetworkPolicyRecommendationList{ListMeta: obj.(*v1alpha1.NetworkPolicyRecommendationList).ListMeta}
	for _, item := range obj.(*v1alpha1.NetworkPolicyRecommendationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested networkPolicyRecommendations.
func (c *FakeNetworkPolicyRecommendations) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(networkpolicyrecommendationsResource, c.ns, opts))

}

// Create takes the representation of a networkPolicyRecommendation and creates it.  Returns the server's representation of the networkPolicyRecommendation, and an error, if there is any.
func (c *FakeNetworkPolicyRecommendations) Create(ctx context.Context, networkPolicyRecommendation *v1alpha1.NetworkPolicyRecommendation, opts v1.CreateOptions) (result *v1alpha1.NetworkPolicyRecommendation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(networkpolicyrecommendationsResource, c.ns, networkPolicyRecommendation), &v1alpha1.NetworkPolicyRecommendation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkPolicyRecommendation), err
}

// Update takes the representation of a networkPolicyRecommendation and updates it. Returns the server's representation of the networkPolicyRecommendation, and an error, if there is any.
func (c *FakeNetworkPolicyRecommendations) Update(ctx context.Context, networkPolicyRecommendation *v1alpha1.NetworkPolicyRecommendation, opts v1.UpdateOptions) (result *v1alpha1.NetworkPolicyRecommendation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(networkpolicyrecommendationsResource, c.ns, networkPolicyRecommendation), &v1alpha1.NetworkPolicyRecommendation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkPolicyRecommendation), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeNetworkPolicyRecommendations) UpdateStatus(ctx context.Context, networkPolicyRecommendation *v1alpha1.NetworkPolicyRecommendation, opts v1.UpdateOptions) (*v1alpha1.NetworkPolicyRecommendation, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(networkpolicyrecommendationsResource, "status", c.ns, networkPolicyRecommendation), &v1alpha1.NetworkPolicyRecommendation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkPolicyRecommendation), err
}

// Delete takes name of the networkPolicyRecommendation and deletes it. Returns an error if one occurs.
func (c *FakeNetworkPolicyRecommendations) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(networkpolicyrecommendationsResource, c.ns, name, opts), &v1alpha1.NetworkPolicyRecommendation{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNetworkPolicyRecommendations) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(networkpolicyrecommendationsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.NetworkPolicyRecommendationList{})
	return err
}

// Patch applies the patch and returns the patched networkPolicyRecommendation.
func (c *FakeNetworkPolicyRecommendations) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.NetworkPolicyRecommendation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(networkpolicyrecommendationsResource, c.ns, name, pt, data, subresources...), &v1alpha1.NetworkPolicyRecommendation{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.NetworkPolicyRecommendation), err
}
