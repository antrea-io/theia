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
	"regexp"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	bundleCollectCmd = "./theia supportbundle -d ./support-bundle"
	bundleExpandCmd  = "tar -xvzf ./support-bundle/theia-support-bundle.tar.gz -C ./support-bundle"
	bundleDeleteCmd  = "rm -rf ./support-bundle"
)

type supportBundleTestCase struct {
	filePath  string
	filenames []string
}

var supportBundleTestCases = []supportBundleTestCase{
	{
		filePath: "clickhouse-server/chi-clickhouse-clickhouse-[a-z0-9-]+",
		filenames: []string{
			"clickhouse-server.err.log",
			"clickhouse-server.log",
		},
	},
	{
		filePath: "flow-aggregator/flow-aggregator-[a-z0-9-]+",
		filenames: []string{
			"flow-aggregator.flow-aggregator-[a-z0-9-]+.root.log.(INFO|WARNING|ERROR|FATAL).[0-9.-]+",
		},
	},
	{
		filePath: "grafana/grafana-[a-z0-9-]+",
		filenames: []string{
			"grafana.log",
		},
	},
	{
		filePath: "theia-manager",
		filenames: []string{
			"theia-manager.theia-manager-[a-z0-9-]+.root.log.(INFO|WARNING|ERROR|FATAL).[0-9.-]+",
		},
	},
	{
		filePath: "zookeeper/zookeeper-[a-z0-9-]+",
		filenames: []string{
			"zookeeper-[a-z0-9-]+.log",
		},
	},
}

func testSupportBundleCollection(t *testing.T, data *TestData) {
	defer cleanupSupportBundle(t, data)

	log.Infof("Collecting support bundle")
	filesCollected, err := collectBundle(t, data)
	require.NoError(t, err)
	log.Infof("Support bundle collected")

	filePathRegexMap := parseFilePathRegex("logs", supportBundleTestCases)
	for _, file := range filesCollected {
		for filePathRegex := range filePathRegexMap {
			matchResult := filePathRegex.MatchString(file)
			if matchResult {
				filePathRegexMap[filePathRegex] = true
				break
			}
		}
	}

	filesNotMatched := make([]string, 0)
	for filePathRegex, matched := range filePathRegexMap {
		if !matched {
			filesNotMatched = append(filesNotMatched, filePathRegex.String())
		}
	}
	if len(filesNotMatched) > 0 {
		err = fmt.Errorf("expected files not found in support bundle: %s", filesNotMatched)
		failOnError(err, t, data)
	}
}

func collectBundle(t *testing.T, data *TestData) ([]string, error) {
	files := make([]string, 0)
	cmd := "chmod +x ./theia"
	rc, stdout, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), cmd)
	if err != nil || rc != 0 {
		return files, fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	rc, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), bundleCollectCmd)
	if err != nil || rc != 0 {
		return files, fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	rc, stdout, stderr, err = data.RunCommandOnNode(controlPlaneNodeName(), bundleExpandCmd)
	if err != nil || rc != 0 {
		return files, fmt.Errorf("error when running %s from %s: %v\nstdout:%s\nstderr:%s", cmd, controlPlaneNodeName(), err, stdout, stderr)
	}
	log.Infof("Files expanded:\n%s", stdout)
	stdout = strings.TrimSuffix(stdout, "\n")
	files = strings.Split(stdout, "\n")
	return files, nil
}

func cleanupSupportBundle(t *testing.T, data *TestData) {
	rc, _, stderr, err := data.RunCommandOnNode(controlPlaneNodeName(), bundleDeleteCmd)
	if err != nil || rc != 0 {
		log.Errorf("Failed to cleanup support bundle: %v\nstderr: %s", err, stderr)
	}
}

func parseFilePathRegex(baseDir string, testcases []supportBundleTestCase) map[*regexp.Regexp]bool {
	fileChecker := make(map[*regexp.Regexp]bool)
	for _, tc := range testcases {
		dir := baseDir + "/" + tc.filePath
		for _, filename := range tc.filenames {
			re := regexp.MustCompile(fmt.Sprintf("^%s/%s$", dir, filename))
			fileChecker[re] = false
		}
	}
	return fileChecker
}
