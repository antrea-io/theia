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
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"antrea.io/theia/pkg/apis/system/v1alpha1"
)

const (
	timeFormat                = "20060102T150405Z0700"
	supportBundleResourceName = "theia-manager"
)

// Command is the support bundle command implementation.
var Command *cobra.Command

var option = &struct {
	dir          string
	since        string
	useClusterIP bool
}{}

var supportbundleLongDescription = strings.TrimSpace(`
Generate support bundles for the cluster, which include logs of: flow-aggregator, theia-manager, grafana, clickhouse-server, zookeeper, spark operator, spark executor and spark driver.
`)

var remoteControllerExample = strings.Trim(`
  Generate support bundle of flow visibility components in cluster and save them to current working dir
  $ theia supportbundle
  Generate support bundle of flow visibility components in cluster with only the logs generated during the last 1 hour
  $ theia supportbundle --since 1h
  Generate support bundle of flow visibility components in cluster and save them to specific dir
  $ theia supportbundle -d ~/Downloads
`, "\n")

var supportBundleCollectCmd = &cobra.Command{
	Use:     "supportbundle",
	Short:   "Generate support bundle",
	Long:    supportbundleLongDescription,
	Example: remoteControllerExample,
	RunE:    collectSupportBundleRunE,
}

func init() {
	supportBundleCollectCmd.Flags().StringVarP(&option.dir, "dir", "d", "", "support bundles output dir, the path will be created if it doesn't exist")
	supportBundleCollectCmd.Flags().StringVarP(&option.since, "since", "", "", "only return logs newer than a relative duration like 5s, 2m or 3h. Defaults to all logs")
	supportBundleCollectCmd.Flags().BoolVarP(&option.useClusterIP, "use-cluster-ip", "", false,
		`Enable this option will use ClusterIP instead of port forwarding when connecting to the Theia
Manager Service. It can only be used when running in cluster.`,
	)
	rootCmd.AddCommand(supportBundleCollectCmd)
}

func download(downloadPath string, client rest.Interface) error {
	for {
		var supportBundle v1alpha1.SupportBundle
		err := client.Get().
			AbsPath("/apis/system.theia.antrea.io/v1alpha1").
			Resource("supportbundles").
			Name(supportBundleResourceName).
			Do(context.TODO()).Into(&supportBundle)
		if err != nil {
			return fmt.Errorf("error when getting support bundle status: %w", err)
		}
		if supportBundle.Status == v1alpha1.SupportBundleStatusCollected {
			if len(downloadPath) == 0 {
				break
			}
			fileName := path.Join(downloadPath, fmt.Sprintf("%s.tar.gz", "theia-support-bundle"))

			f, err := os.Create(fileName)
			if err != nil {
				return fmt.Errorf("error when creating the support bundle tar gz: %w", err)
			}
			defer f.Close()
			stream, err := client.Get().
				AbsPath("/apis/system.theia.antrea.io/v1alpha1").
				Resource("supportbundles").
				Name(supportBundleResourceName).
				SubResource("download").
				Stream(context.TODO())
			if err != nil {
				return fmt.Errorf("error when downloading the support bundle: %w", err)
			}
			defer stream.Close()
			if _, err := io.Copy(f, stream); err != nil {
				return fmt.Errorf("error when downloading the support bundle: %w", err)
			}
			break
		}
	}
	return nil
}

func collectSupportBundleRunE(cmd *cobra.Command, args []string) error {
	if option.dir == "" {
		cwd, _ := os.Getwd()
		option.dir = filepath.Join(cwd, "support-bundles_"+time.Now().Format(timeFormat))
	}

	dir, err := filepath.Abs(option.dir)
	if err != nil {
		return fmt.Errorf("error when resolving path '%s': %w", option.dir, err)
	}
	if err := os.MkdirAll(option.dir, 0700|os.ModeDir); err != nil {
		return fmt.Errorf("error when creating output dir: %w", err)
	}

	theiaClient, pf, err := SetupTheiaClientAndConnection(cmd, option.useClusterIP)
	if err != nil {
		return fmt.Errorf("couldn't setup Theia manager client, %v", err)
	}
	if pf != nil {
		defer pf.Stop()
	}

	supportbundle := v1alpha1.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{Name: supportBundleResourceName},
		Since:      option.since,
	}

	err = theiaClient.Post().
		AbsPath("/apis/system.theia.antrea.io/v1alpha1").
		Resource("supportbundles").
		Body(&supportbundle).
		Do(context.TODO()).Error()
	if err != nil {
		return fmt.Errorf("failed to request support bundle: %v", err)
	}

	return download(dir, theiaClient)
}
