// Copyright 2022 Antrea Authors.
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

package config

import "time"

const (
	FlowVisibilityNS        = "flow-visibility"
	K8sQuantitiesReg        = "^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	SparkImage              = "projects.registry.vmware.com/antrea/theia-policy-recommendation:latest"
	SparkImagePullPolicy    = "IfNotPresent"
	SparkAppFile            = "local:///opt/spark/work-dir/policy_recommendation_job.py"
	SparkServiceAccount     = "policy-recommendation-spark"
	SparkVersion            = "3.1.1"
	StatusCheckPollInterval = 5 * time.Second
	StatusCheckPollTimeout  = 60 * time.Minute
)
