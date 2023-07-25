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

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

var workerNodeNames = []string{"kind-worker", "kind-worker2"}
var covDirs = []string{".coverage/clickhouse-monitor-coverage", ".coverage/theia-manager-coverage"}
var covDirPrefixes = []string{"cm", "tm"}

func copyCovFolder(nodeName, covDir, covPrefix string) error {
	covDirAbs, err := filepath.Abs("../../" + covDir)
	if err != nil {
		return fmt.Errorf("error while creating absolute file path: %v", err)
	}
	pathOnNode := nodeName + ":" + "/var/log/" + covPrefix + "-coverage/."
	cmd := exec.Command("docker", "cp", pathOnNode, covDirAbs)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		errStr := errb.String()
		outStr := outb.String()
		if !strings.Contains(errb.String(), "Could not find the file") {
			return fmt.Errorf("error while running docker cp command[%v] from node: %s: stdout: %s, stderr: %s", cmd, nodeName, outStr, errStr)
		}
	}
	return nil
}

func clearCovFolder(nodeName, covPrefix string) error {
	nestedCmd := "`rm -rf /var/log/" + covPrefix + "-coverage/*`"
	cmd := exec.Command("docker", "exec", nodeName, "sh", "-c", nestedCmd)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		errStr := errb.String()
		outStr := outb.String()
		if !strings.Contains(errb.String(), "not found") {
			return fmt.Errorf("error while running docker exec command[%v] from node: %s: stdout: %s, stderr: %s", cmd, nodeName, outStr, errStr)
		}
	}
	return nil
}

func tearDownBothNodes() error {
	log.Infof("Running final coverage copy and cleanup.\n")
	for _, prefix := range covDirPrefixes {
		for _, workerNode := range workerNodeNames {
			for _, covDir := range covDirs {
				if err := copyCovFolder(workerNode, covDir, prefix); err != nil {
					return err
				}
			}
			if err := clearCovFolder(workerNode, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestCoverageAndCleanup(t *testing.T) {
	if os.Getenv("COVERAGE") == "" {
		t.Skipf("COVERAGE env variable is not set until all tests are run, skipping test.")
	}
	tearDownBothNodes()
}
