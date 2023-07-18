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

package e2e

import (
	"flag"
	"testing"
)

var (
	upgradeToVersion = flag.String("upgrade.toVersion", "", "Version updated to")
)

const (
	upgradeToAntreaYML         = "antrea-new.yml"
	upgradeToFlowVisibilityYML = "flow-visibility-new.yml"
	upgradeToChOperatorYML     = "clickhouse-operator-install-bundle-new.yaml"
)

func skipIfNotUpgradeTest(t *testing.T) {
	if *upgradeToVersion == "" {
		t.Skipf("Skipping test as we are not testing for upgrade")
	}
}

// TestUpgrade tests that some basic functionalities are not broken when
// upgrading from one version of Theia to another. At the moment it checks
// that:
//   - ClickHouse data schema version
//   - Consistency of recommendations stored in ClickHouse
//
// To run the test, provide the -upgrade.toVersion flag.
func TestUpgrade(t *testing.T) {
	skipIfNotUpgradeTest(t)
	config := FlowVisibilitySetUpConfig{
		withSparkOperator:     false,
		withGrafana:           false,
		withClickHouseLocalPv: true,
		withFlowAggregator:    false,
	}
	data, _, _, err := setupTestForFlowVisibility(t, config)
	if err != nil {
		t.Fatalf("Error when setting up test: %v", err)
	}
	defer func() {
		teardownTest(t, data)
		teardownFlowVisibility(t, data, config)
		data.deleteClickHouseOperator(upgradeToChOperatorYML)
	}()
	// upgrade and check
	ApplyNewVersion(t, data, upgradeToAntreaYML, upgradeToChOperatorYML, upgradeToFlowVisibilityYML)
}
