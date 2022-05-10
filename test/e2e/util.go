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

package e2e

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
)

const nameSuffixLength int = 8

func createDirectory(path string) error {
	return os.Mkdir(path, 0700)
}

// IsDirEmpty checks whether a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

// A DNS-1123 subdomain must consist of lower case alphanumeric characters
var lettersAndDigits = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		// #nosec G404: random number generator not used for security purposes
		randIdx := rand.Intn(len(lettersAndDigits))
		b[i] = lettersAndDigits[randIdx]
	}
	return string(b)
}

func randName(prefix string) string {
	return prefix + randSeq(nameSuffixLength)
}

// runAntctl runs antctl commands on antrea Pods, the controller, or agents.
func runAntctl(podName string, cmds []string, data *TestData) (string, string, error) {
	var containerName string
	var namespace string
	if strings.Contains(podName, "agent") {
		containerName = "antrea-agent"
		namespace = antreaNamespace
	} else if strings.Contains(podName, "flow-aggregator") {
		containerName = "flow-aggregator"
		namespace = flowAggregatorNamespace
	} else {
		containerName = "antrea-controller"
		namespace = antreaNamespace
	}
	stdout, stderr, err := data.RunCommandFromPod(namespace, podName, containerName, cmds)
	// remove Bincover metadata if needed
	if err == nil {
		index := strings.Index(stdout, "START_BINCOVER_METADATA")
		if index != -1 {
			stdout = stdout[:index]
		}
	}
	return stdout, stderr, err
}

// antctlOutput is a helper function for generating antctl outputs.
func antctlOutput(stdout, stderr string) string {
	return fmt.Sprintf("antctl stdout:\n%s\nantctl stderr:\n%s", stdout, stderr)
}
