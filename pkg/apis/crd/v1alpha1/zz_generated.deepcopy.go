//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright 2023 Antrea Authors
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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkPolicyRecommendation) DeepCopyInto(out *NetworkPolicyRecommendation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkPolicyRecommendation.
func (in *NetworkPolicyRecommendation) DeepCopy() *NetworkPolicyRecommendation {
	if in == nil {
		return nil
	}
	out := new(NetworkPolicyRecommendation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NetworkPolicyRecommendation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkPolicyRecommendationList) DeepCopyInto(out *NetworkPolicyRecommendationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NetworkPolicyRecommendation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkPolicyRecommendationList.
func (in *NetworkPolicyRecommendationList) DeepCopy() *NetworkPolicyRecommendationList {
	if in == nil {
		return nil
	}
	out := new(NetworkPolicyRecommendationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NetworkPolicyRecommendationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkPolicyRecommendationSpec) DeepCopyInto(out *NetworkPolicyRecommendationSpec) {
	*out = *in
	in.StartInterval.DeepCopyInto(&out.StartInterval)
	in.EndInterval.DeepCopyInto(&out.EndInterval)
	if in.NSAllowList != nil {
		in, out := &in.NSAllowList, &out.NSAllowList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkPolicyRecommendationSpec.
func (in *NetworkPolicyRecommendationSpec) DeepCopy() *NetworkPolicyRecommendationSpec {
	if in == nil {
		return nil
	}
	out := new(NetworkPolicyRecommendationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkPolicyRecommendationStatus) DeepCopyInto(out *NetworkPolicyRecommendationStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.EndTime.DeepCopyInto(&out.EndTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkPolicyRecommendationStatus.
func (in *NetworkPolicyRecommendationStatus) DeepCopy() *NetworkPolicyRecommendationStatus {
	if in == nil {
		return nil
	}
	out := new(NetworkPolicyRecommendationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ThroughputAnomalyDetector) DeepCopyInto(out *ThroughputAnomalyDetector) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ThroughputAnomalyDetector.
func (in *ThroughputAnomalyDetector) DeepCopy() *ThroughputAnomalyDetector {
	if in == nil {
		return nil
	}
	out := new(ThroughputAnomalyDetector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ThroughputAnomalyDetectorSpec) DeepCopyInto(out *ThroughputAnomalyDetectorSpec) {
	*out = *in
	in.StartInterval.DeepCopyInto(&out.StartInterval)
	in.EndInterval.DeepCopyInto(&out.EndInterval)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ThroughputAnomalyDetectorSpec.
func (in *ThroughputAnomalyDetectorSpec) DeepCopy() *ThroughputAnomalyDetectorSpec {
	if in == nil {
		return nil
	}
	out := new(ThroughputAnomalyDetectorSpec)
	in.DeepCopyInto(out)
	return out
}
