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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"

	antreav1alpha1 "antrea.io/antrea/pkg/apis/crd/v1alpha1"
	antreav1alpha2 "antrea.io/antrea/pkg/apis/crd/v1alpha2"
)

var (
	migrateToVersion   = flag.String("migrate.toVersion", "", "Version migrated to")
	migrateFromVersion = flag.String("migrate.fromVersion", "", "Version migrated from")
)

const (
	migrateFromFlowVisibilityYML = "flow-visibility-ch-only.yml"
	migrateFromChOperatorYML     = "clickhouse-operator-install-bundle.yaml"
	latestAntreaYML              = "antrea-new.yml"
	migrateToFlowVisibilityYML   = "flow-visibility-new.yml"
	migrateToChOperatorYML       = "clickhouse-operator-install-bundle-new.yaml"
)

func skipIfNotMigrateTest(t *testing.T) {
	if *migrateToVersion == "" {
		t.Skipf("Skipping test as we are not testing for migrate")
	}
}

// TestUpgrade tests that some basic functionalities are not broken when
// upgrading from one version of Theia to another. At the moment it checks
// that:
//   - ClickHouse data schema version
//   - Consistency of recommendations stored in ClickHouse
//
// To run the test, provide the -upgrade.toVersion flag.
func TestMigrate(t *testing.T) {
	skipIfNotMigrateTest(t)
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
		TeardownFlowVisibility(t, data, config, controlPlaneNodeName())
		data.deleteClickHouseOperator(migrateToChOperatorYML)
	}()
	checkClickHouseVersionTable(t, data, *migrateFromVersion)
	if needCheckRecommendationsSchema(*migrateFromVersion) {
		insertRecommendations(t, data)
	}
	// upgrade and check
	ApplyNewVersion(t, data, latestAntreaYML, migrateToChOperatorYML, migrateToFlowVisibilityYML)
	checkClickHouseVersionTable(t, data, *migrateToVersion)
	// This check only works when upgrading from v0.3.0 to v0.4.0 for now
	// as the recommendations schema only changes between these 2 version.
	// More versions can be added when we add other changes to recommendations
	// table.
	if needCheckRecommendationsSchema(*migrateFromVersion) {
		checkRecommendations(t, data, *migrateToVersion)
	}
	// downgrade and check
	ApplyNewVersion(t, data, latestAntreaYML, migrateFromChOperatorYML, migrateFromFlowVisibilityYML)
	checkClickHouseVersionTable(t, data, *migrateFromVersion)
	if needCheckRecommendationsSchema(*migrateFromVersion) {
		checkRecommendations(t, data, *migrateFromVersion)
	}
}

func checkClickHouseVersionTable(t *testing.T, data *TestData, version string) {
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

func needCheckRecommendationsSchema(fromVersion string) bool {
	return fromVersion == "v0.3.0"
}

func insertRecommendations(t *testing.T, data *TestData) {
	command := fmt.Sprintf("clickhouse client -q \"INSERT INTO recommendations (id, yamls) VALUES ('%s', '%s'), ('%s', '%s')\"",
		id1, strings.Join([]string{getNetworkPolicyYaml("knp"), getNetworkPolicyYaml("anp")}, "\n---\n"),
		id2, strings.Join([]string{getNetworkPolicyYaml("acg"), getNetworkPolicyYaml("acnp")}, "\n---\n"),
	)
	stdout, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", command})
	require.NoErrorf(t, err, "Fail to get tables from ClickHouse, stdout: %v, stderr: %v", stdout, stderr)
}

func checkRecommendations(t *testing.T, data *TestData, version string) {
	if version == "v0.3.0" {
		checkRecommendationsVersion3(t, data, id1, []string{"knp", "anp"})
		checkRecommendationsVersion3(t, data, id2, []string{"acnp", "acg"})
	} else if version == "v0.4.0" {
		checkRecommendationsVersion4(t, data, id1, "knp")
		checkRecommendationsVersion4(t, data, id1, "anp")
		checkRecommendationsVersion4(t, data, id2, "acnp")
		checkRecommendationsVersion4(t, data, id2, "acg")
	}
}

func checkRecommendationsVersion3(t *testing.T, data *TestData, id string, kinds []string) {
	// Get the recommendationed policy and check if it is equal to the original yaml
	command := fmt.Sprintf("clickhouse client -q \"select yamls from recommendations where id='%s'\"", id)
	queryOutput, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", command})
	require.NoErrorf(t, err, "Fail to get recommendations from ClickHouse, stderr: %v", stderr)
	queryOutput = strings.ReplaceAll(queryOutput, "\\n", "\n")
	for _, kind := range kinds {
		assert.Contains(t, queryOutput, getNetworkPolicyYaml(kind))
	}
}

func checkRecommendationsVersion4(t *testing.T, data *TestData, id string, kind string) {
	// Get the recommendationed policy and check if it is equal to the original yaml
	command := fmt.Sprintf("clickhouse client -q \"select policy from recommendations where id='%s' and kind='%s'\"", id, kind)
	queryOutput, stderr, err := data.RunCommandFromPod(flowVisibilityNamespace, clickHousePodName, "clickhouse", []string{"bash", "-c", command})
	require.NoErrorf(t, err, "Fail to get recommendations from ClickHouse, stderr: %v", stderr)
	queryOutput = strings.ReplaceAll(queryOutput, "\\n", "\n")
	assert.Contains(t, queryOutput, getNetworkPolicyYaml(kind))
	// Parse the recommendationed policy to corresponding type to verify there is no error
	switch kind {
	case "acnp":
		var acnp antreav1alpha1.ClusterNetworkPolicy
		err = yaml.Unmarshal([]byte(queryOutput), &acnp)
		require.NoErrorf(t, err, "failed to parse the policy with kind acnp, yaml: %s", queryOutput)
	case "anp":
		var anp antreav1alpha1.NetworkPolicy
		err = yaml.Unmarshal([]byte(queryOutput), &anp)
		require.NoErrorf(t, err, "failed to parse the policy with kind anp, yaml: %s", queryOutput)
	case "acg":
		var acg antreav1alpha2.ClusterGroup
		err = yaml.Unmarshal([]byte(queryOutput), &acg)
		require.NoErrorf(t, err, "failed to parse the policy with kind acg, yaml: %s", queryOutput)
	case "knp":
		var knp networkingv1.NetworkPolicy
		err = yaml.Unmarshal([]byte(queryOutput), &knp)
		require.NoErrorf(t, err, "failed to parse the policy with kind knp, yaml: %s", queryOutput)
	}
}
