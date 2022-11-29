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
	diskInfo   bool
	tableInfo  bool
	insertRate bool
	stackTrace bool
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
	clickHouseStatusCmd.Flags().BoolVar(&options.stackTrace, "stackTrace", false, "check stacktrace of clickhouse")
}

func getStatus(cmd *cobra.Command, args []string) error {
	if !options.diskInfo && !options.tableInfo && !options.insertRate && !options.stackTrace {
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
	if options.stackTrace {
		names = append(names, "stackTrace")
	}
	for _, name := range names {
		data, err := getClickHouseStatusByCategory(theiaClient, name)
		if err != nil {
			return fmt.Errorf("error when getting clickhouse %v status: %s", name, err)
		}
		if len(data.ErrorMsg) != 0 {
			for _, errorMsg := range data.ErrorMsg {
				fmt.Printf("Error message: %s\n", errorMsg)
			}
		}
		var result [][]string
		switch name {
		case "diskInfo":
			result = append(result, []string{"Shard", "DatabaseName", "Path", "Free", "Total", "Used_Percentage"})
			for _, diskInfo := range data.DiskInfos {
				result = append(result, []string{diskInfo.Shard, diskInfo.Database, diskInfo.Path, diskInfo.FreeSpace, diskInfo.TotalSpace, diskInfo.UsedPercentage})
			}
		case "tableInfo":
			result = append(result, []string{"Shard", "DatabaseName", "TableName", "TotalRows", "TotalBytes", "TotalCols"})
			for _, tableInfo := range data.TableInfos {
				result = append(result, []string{tableInfo.Shard, tableInfo.Database, tableInfo.TableName, tableInfo.TotalRows, tableInfo.TotalBytes, tableInfo.TotalCols})
			}
		case "insertRate":
			result = append(result, []string{"Shard", "RowsPerSecond", "BytesPerSecond"})
			for _, insertRate := range data.InsertRates {
				result = append(result, []string{insertRate.Shard, insertRate.RowsPerSec, insertRate.BytesPerSec})
			}
		case "stackTrace":
			result = append(result, []string{"Shard", "TraceFunctions", "Count()"})
			for _, stackTrace := range data.StackTraces {
				result = append(result, []string{stackTrace.Shard, stackTrace.TraceFunctions, stackTrace.Count})
			}
		}
		if name == "stackTrace" {
			TableOutputVertical(result)
		} else {
			TableOutput(result)
		}
	}
	return nil
}
