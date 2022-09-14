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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	upgradeToAntreaYML         = "antrea-new.yml"
	upgradeToFlowVisibilityYML = "flow-visibility-new.yml"
	upgradeToVersion           = flag.String("upgrade.toVersion", "", "Version updated to")
	upgradeFromVersion         = flag.String("upgrade.fromVersion", "", "Version updated from")
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
//
// To run the test, provide the -upgrade.toVersion flag.
func TestUpgrade(t *testing.T) {
	skipIfNotUpgradeTest(t)
	config := FlowVisibiltiySetUpConfig{
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
	}()
	checkClickHouseDataSchema(t, data, *upgradeFromVersion)

	t.Logf("Upgrading Antrea YAML to %s", upgradeToAntreaYML)
	// Do not wait for agent rollout as its updateStrategy is set to OnDelete for upgrade test.
	if err := data.deployAntreaCommon(upgradeToAntreaYML, "", false); err != nil {
		t.Fatalf("Error upgrading Antrea: %v", err)
	}
	t.Logf("Restarting all Antrea DaemonSet Pods")
	if err := data.restartAntreaAgentPods(defaultTimeout); err != nil {
		t.Fatalf("Error when restarting Antrea: %v", err)
	}

	t.Logf("Upgrading Flow Visibility YAML to %s", upgradeToFlowVisibilityYML)
	if err := data.deployFlowVisibilityCommon(upgradeToFlowVisibilityYML); err != nil {
		t.Fatalf("Error upgrading Flow Visibility: %v", err)
	}
	t.Logf("Waiting for the ClickHouse Pod restarting")
	if err := data.waitForClickHousePod(); err != nil {
		t.Fatalf("Error when waiting for the ClickHouse Pod restarting: %v", err)
	}
	checkClickHouseDataSchema(t, data, *upgradeToVersion)
}

func checkClickHouseDataSchema(t *testing.T, data *TestData, version string) {
	queryOutput, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", "clickhouse client -q \"SHOW TABLES\""})
	require.NoErrorf(t, err, "Fail to get tables from ClickHouse: %v", stderr)
	if version != "v0.1.0" {
		require.Contains(t, queryOutput, "flows")
		require.Contains(t, queryOutput, "flows_local")
		if version == "v0.2.0" {
			require.Contains(t, queryOutput, "migrate_version")
			queryOutput, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", "clickhouse client -q \"SELECT version FROM migrate_version\""})
			require.NoErrorf(t, err, "Fail to get version from ClickHouse: %v", stderr)
			// strip leading 'v'
			assert.Contains(t, queryOutput, version[1:])
		} else {
			require.Contains(t, queryOutput, "schema_migrations")
		}
	}
}
