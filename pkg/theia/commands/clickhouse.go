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

package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

var clickHouseCmd = &cobra.Command{
	Use:     "clickhouse",
	Aliases: []string{"ch"},
	Short:   "Commands of Theia ClickHouse feature",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Error: must also specify a subcommand to run like status")
	},
}

func init() {
	rootCmd.AddCommand(clickHouseCmd)
	clickHouseCmd.PersistentFlags().String(
		"clickhouse-endpoint",
		"",
		"The ClickHouse service endpoint.")
	clickHouseCmd.PersistentFlags().Bool(
		"use-cluster-ip",
		false,
		`Enable this option will use ClusterIP instead of port forwarding when connecting to the ClickHouse Service.
It can only be used when running in cluster.`,
	)
}
