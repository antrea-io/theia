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

package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type chOptions struct {
	diskInfo    bool
	tableInfo   bool
	insertRate  bool
	stackTraces bool
}

var options *chOptions

var clickHouseStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Get diagnostic infos of ClickHouse database",
	Example: example,
	Args:    cobra.NoArgs,
	RunE:    getStatus,
}

var example = strings.Trim(`
theia clickhouse status --diskInfo
theia clickhouse status --diskInfo --tableInfo
theia clickhouse status --diskInfo --tableInfo --insertRate
`, "\n")

func init() {
	clickHouseCmd.AddCommand(clickHouseStatusCmd)
	options = &chOptions{}
	clickHouseStatusCmd.Flags().BoolVar(&options.diskInfo, "diskInfo", false, "check disk usage information")
	clickHouseStatusCmd.Flags().BoolVar(&options.tableInfo, "tableInfo", false, "check basic table information")
	clickHouseStatusCmd.Flags().BoolVar(&options.insertRate, "insertRate", false, "check the insertion-rate of clickhouse")
	clickHouseStatusCmd.Flags().BoolVar(&options.stackTraces, "stackTraces", false, "check stacktrace of clickhouse")
}

func getStatus(cmd *cobra.Command, args []string) error {
	if !options.diskInfo && !options.tableInfo && !options.insertRate && !options.stackTraces {
		return fmt.Errorf("no metric related flag is specified")
	}
	useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
	if err != nil {
		return err
	}
	theiaClient, pf, err := SetupTheiaClientAndConnection(cmd, useClusterIP)
	if err != nil {
		return fmt.Errorf("couldn't setup Theia manager client, %v", err)
	}
	if pf != nil {
		defer pf.Stop()
	}
	var names []string
	if options.diskInfo {
		names = append(names, "diskInfo")
	}
	if options.tableInfo {
		names = append(names, "tableInfo")
	}
	if options.insertRate {
		names = append(names, "insertRate")
	}
	if options.stackTraces {
		names = append(names, "stackTraces")
	}
	for _, name := range names {
		data, err := getClickHouseStatusByCategory(theiaClient, name)
		if err != nil {
			return fmt.Errorf("error when getting clickhouse %v status: %s", name, err)
		}
		if name == "stackTraces" {
			TableOutputVertical(data.Stat)
		} else {
			TableOutput(data.Stat)
		}
	}
	return nil
}
