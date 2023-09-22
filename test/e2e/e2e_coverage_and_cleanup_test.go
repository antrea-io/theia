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
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
)

var workerNodeNames = []string{"kind-worker", "kind-worker2"}
var covDirs = []string{".coverage/clickhouse-monitor-coverage", ".coverage/theia-manager-coverage"}
var covDirPrefixes = []string{"cm", "tm"}

func tearDownBothNodes() error {
	log.Infof("Running final coverage copy and cleanup.\n")
	for _, prefix := range covDirPrefixes {
		for _, workerNode := range workerNodeNames {
			for _, covDir := range covDirs {
				if err := CopyCovFolder(workerNode, covDir, prefix); err != nil {
					return err
				}
			}
			if err := ClearCovFolder(workerNode, prefix); err != nil {
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
