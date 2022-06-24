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
	"os"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// rootCmd represents the base command when called without any subcommands
var (
	verbose = 0
	rootCmd = &cobra.Command{
		Use:   "theia",
		Short: "theia is the command line tool for Theia",
		Long: `theia is the command line tool for Theia which provides access 
to Theia network flow visibility capabilities`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			verboseLevel, err := cmd.Flags().GetInt("verbose")
			if err != nil {
				return err
			}
			var l klog.Level
			l.Set(fmt.Sprint(verboseLevel))
			return nil
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0, "set verbose level")
	rootCmd.PersistentFlags().StringP(
		"kubeconfig",
		"k",
		"",
		"absolute path to the k8s config file, will use $KUBECONFIG if not specified",
	)
}
