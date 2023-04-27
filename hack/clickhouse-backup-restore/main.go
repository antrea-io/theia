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

package main

import (
	"flag"
	"fmt"
	"os"

	"antrea.io/theia/hack/clickhouse-backup-restore/clickhouseBackupRestore"
)

func main() {
	// Define command-line flags for namespace and pod name.
	fromYaml := flag.String("fromYaml", "", "YAML file for cluster update from")
	toYaml := flag.String("toYaml", "", "YAML file for cluster update to")
	deleteBackup := flag.Bool("deleteBackup", true, "Delete Backup after success")
	deleteYamls := flag.Bool("deleteYamls", false, "Delete Yamls after success")
	loglevel := flag.String("loglevel", "0", "Set loglevel 1 for debug")
	flag.Parse()

	fmt.Println("Cluster update from:", *fromYaml)
	fmt.Println("Cluster update to:", *toYaml)

	// Check if to and from yamls are provided.
	if *toYaml == "" || *fromYaml == "" {
		fmt.Println("Please provide both YAML files for upgrade ")
		os.Exit(1)
	}
	clickhouseBackupRestore.BackupAndRestoreClickhouse(*fromYaml, *toYaml, *deleteBackup, *loglevel, *deleteYamls)
}
