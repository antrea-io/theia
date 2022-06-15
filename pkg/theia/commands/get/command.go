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

package get

import (
	"fmt"

	"github.com/spf13/cobra"

	"antrea.io/theia/pkg/theia/commands/get/clickhouse"
)

var GetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get diagnostic infos of ClickHouse DataBase or Spark",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Error: must also specify a subcommand to run")
	},
}

func init() {
	GetCmd.AddCommand(clickhouse.Command)
}
