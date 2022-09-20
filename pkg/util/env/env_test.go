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
	"testing"
)

func TestGetPodNamespace(t *testing.T) {
	testTable := map[string]string{
		"test-namespace": "test-namespace",
		"":               "",
	}

	for k, v := range testTable {
		comparePodNamespace(k, v, t)
	}
}

func comparePodNamespace(k, v string, t *testing.T) {
	if k != "" {
		_ = os.Setenv(podNamespaceEnvKey, k)
		defer os.Unsetenv(podNamespaceEnvKey)
	}
	podNamespace := GetPodNamespace()
	if podNamespace != v {
		t.Errorf("Failed to retrieve pod namespace, want: %s, get: %s", v, podNamespace)
	}
}

func TestGetTheiaNamespace(t *testing.T) {
	testTable := map[string]string{
		"test-namespace": "test-namespace",
		"":               "flow-visibility",
	}

	for k, v := range testTable {
		compareTheiaNamespace(k, v, t)
	}
}

func compareTheiaNamespace(k, v string, t *testing.T) {
	if k != "" {
		_ = os.Setenv(podNamespaceEnvKey, k)
		defer os.Unsetenv(podNamespaceEnvKey)
	}
	podNamespace := GetTheiaNamespace()
	if podNamespace != v {
		t.Errorf("Failed to retrieve pod namespace, want: %s, get: %s", v, podNamespace)
	}
}
