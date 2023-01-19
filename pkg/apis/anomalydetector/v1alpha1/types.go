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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ThroughputAnomalyDetector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type                string                           `json:"jobType,omitempty"`
	StartInterval       metav1.Time                      `json:"startInterval,omitempty"`
	EndInterval         metav1.Time                      `json:"endInterval,omitempty"`
	ExecutorInstances   int                              `json:"executorInstances,omitempty"`
	DriverCoreRequest   string                           `json:"driverCoreRequest,omitempty"`
	DriverMemory        string                           `json:"driverMemory,omitempty"`
	ExecutorCoreRequest string                           `json:"executorCoreRequest,omitempty"`
	ExecutorMemory      string                           `json:"executorMemory,omitempty"`
	Status              ThroughputAnomalyDetectorStatus  `json:"status,omitempty"`
	Stats               []ThroughputAnomalyDetectorStats `json:"stats,omitempty"`
}

type ThroughputAnomalyDetectorStatus struct {
	State            string      `json:"state,omitempty"`
	SparkApplication string      `json:"sparkApplication,omitempty"`
	CompletedStages  int         `json:"completedStages,omitempty"`
	TotalStages      int         `json:"totalStages,omitempty"`
	ErrorMsg         string      `json:"errorMsg,omitempty"`
	StartTime        metav1.Time `json:"startTime,omitempty"`
	EndTime          metav1.Time `json:"endTime,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ThroughputAnomalyDetectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThroughputAnomalyDetector `json:"items"`
}

type ThroughputAnomalyDetectorStats struct {
	Id                       string `json:"id,omitempty"`
	SourceIP                 string `json:"sourceIP,omitempty"`
	SourceTransportPort      string `json:"sourceTransportPort,omitempty"`
	DestinationIP            string `json:"destinationIP,omitempty"`
	DestinationTransportPort string `json:"destinationTransportPort,omitempty"`
	FlowStartSeconds         string `json:"FlowStartSeconds,omitempty"`
	FlowEndSeconds           string `json:"FlowEndSeconds,omitempty"`
	Throughput               string `json:"Throughput,omitempty"`
	AlgoCalc                 string `json:"AlgoCalc,omitempty"`
	Anomaly                  string `json:"anomaly,omitempty"`
}
