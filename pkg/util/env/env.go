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

package env

import (
	"os"

	"k8s.io/klog/v2"
)

const (
	podNamespaceEnvKey = "POD_NAMESPACE"

	defaultTheiaNamespace = "flow-visibility"
)

// GetPodNamespace returns Namespace of the Pod where the code executes.
func GetPodNamespace() string {
	podNamespace := os.Getenv(podNamespaceEnvKey)
	if podNamespace == "" {
		klog.V(2).InfoS("Environment variable not found", "Environment Key", podNamespaceEnvKey)
	}
	return podNamespace
}

// GetTheiaNamespace tries to determine the Namespace in which Theia is running by looking at the
// POD_NAMESPACE environment variable. If this environment variable is not set (e.g. because the
// Theia component is not run as a Pod), "flow-visibility" is returned.
func GetTheiaNamespace() string {
	namespace := GetPodNamespace()
	if namespace == "" {
		klog.V(2).InfoS("Failed to get Pod Namespace from environment. Using default Theia namespace", "namespace", defaultTheiaNamespace)
		namespace = defaultTheiaNamespace
	}
	return namespace
}
