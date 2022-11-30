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

package support

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/cp"

	"antrea.io/antrea/pkg/util/logdir"
)

const (
	clickhouseLogDir     = "/var/log/clickhouse-server/"
	grafanaLogDir        = "/var/log/grafana/"
	flowAggregatorLogDir = "/var/log/antrea/flow-aggregator/"

	flowAggregatorNS = "flow-aggregator"
)

// ManagerDumper is the interface for dumping runtime information of the
// flow visibility solution. Its functions should only work in the theia manager Pod.
type ManagerDumper interface {
	// DumpLog should create files that contains container logs of the manager
	// Pod under the basedir.
	DumpLog(basedir string) error
	DumpClickHouseServerLog(basedir string) error
	DumpFlowAggregatorLog(basedir string) error
	DumpGrafanaLog(basedir string) error
	DumpSparkDriverLog(basedir string) error
	DumpSparkExecutorLog(basedir string) error
	DumpSparkOperatorLog(basedir string) error
	DumpZookeeperLog(basedir string) error
}

type managerDumper struct {
	fs         afero.Fs
	restConfig *rest.Config
	clientSet  kubernetes.Interface
	since      string

	namespace              string
	clickhousePodNames     []string
	grafanaPodNames        []string
	flowAggregatorPodNames []string
	sparkDriverPodNames    []string
	sparkExecutorPodNames  []string
	sparkOperatorPodNames  []string
	zookeeperPodNames      []string
}

func NewManagerDumper(fs afero.Fs, restConfig *rest.Config, clientSet kubernetes.Interface, since string,
	namespace string, clickhousePodNames, grafanaPodNames, flowAggregatorPodNames, sparkDriverPodNames,
	sparkExecutorPodNames, sparkOperatorPodNames, zookeeperPodNames []string) ManagerDumper {
	return &managerDumper{
		fs:                     fs,
		restConfig:             restConfig,
		clientSet:              clientSet,
		since:                  since,
		namespace:              namespace,
		clickhousePodNames:     clickhousePodNames,
		grafanaPodNames:        grafanaPodNames,
		flowAggregatorPodNames: flowAggregatorPodNames,
		sparkDriverPodNames:    sparkDriverPodNames,
		sparkExecutorPodNames:  sparkExecutorPodNames,
		sparkOperatorPodNames:  sparkOperatorPodNames,
		zookeeperPodNames:      zookeeperPodNames,
	}
}

func (d *managerDumper) DumpLog(basedir string) error {
	logDir := logdir.GetLogDir()
	return directoryCopy(d.fs, path.Join(basedir, "logs", "theia-manager"), logDir, "theia-manager", timestampFilter(d.since))
}

func (d *managerDumper) DumpClickHouseServerLog(basedir string) error {
	for _, podName := range d.clickhousePodNames {

		if err := d.copyFromPod(d.namespace, podName, clickhouseLogDir,
			path.Join(basedir, "logs", "clickhouse-server", podName)); err != nil {
			return fmt.Errorf("error when copying logs from clickhouse %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) DumpFlowAggregatorLog(basedir string) error {
	for _, podName := range d.flowAggregatorPodNames {
		tmpLogDir := path.Join(basedir, "logs", "flow-aggregator", "tmp", podName)
		if err := d.copyFromPod(flowAggregatorNS, podName, flowAggregatorLogDir, tmpLogDir); err != nil {
			return fmt.Errorf("error when copying logs from flow-aggregator %s: %w", podName, err)
		}

		if err := directoryCopy(d.fs, path.Join(basedir, "logs", "flow-aggregator", podName), tmpLogDir,
			"flow-aggregator", timestampFilter(d.since)); err != nil {
			return err
		}
		if err := d.fs.RemoveAll(tmpLogDir); err != nil {
			return err
		}
	}
	return nil
}

func (d *managerDumper) DumpGrafanaLog(basedir string) error {
	for _, podName := range d.grafanaPodNames {
		if err := d.copyFromPod(d.namespace, podName, grafanaLogDir,
			path.Join(basedir, "logs", "grafana", podName)); err != nil {
			return fmt.Errorf("error when copying logs from grafana %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) DumpSparkDriverLog(basedir string) error {
	for _, podName := range d.sparkDriverPodNames {
		if err := d.copyLogFromPod(d.namespace, podName,
			path.Join(basedir, "logs", "spark-driver", podName)); err != nil {
			return fmt.Errorf("error when streaming logs from spark driver %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) DumpSparkExecutorLog(basedir string) error {
	for _, podName := range d.sparkExecutorPodNames {
		if err := d.copyLogFromPod(d.namespace, podName,
			path.Join(basedir, "logs", "spark-executor", podName)); err != nil {
			return fmt.Errorf("error when streaming logs from spark executor %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) DumpSparkOperatorLog(basedir string) error {
	for _, podName := range d.sparkOperatorPodNames {
		if err := d.copyLogFromPod(d.namespace, podName,
			path.Join(basedir, "logs", "spark-operator", podName)); err != nil {
			return fmt.Errorf("error when streaming logs from spark operator %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) DumpZookeeperLog(basedir string) error {
	for _, podName := range d.zookeeperPodNames {
		if err := d.copyLogFromPod(d.namespace, podName,
			path.Join(basedir, "logs", "zookeeper", podName)); err != nil {
			return fmt.Errorf("error when streaming logs from zookeeper %s: %w", podName, err)
		}
	}
	return nil
}

func (d *managerDumper) copyFromPod(namespace, pod, srcDir, dstDir string) error {
	ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams()
	copyOptions := cp.NewCopyOptions(ioStreams)
	copyOptions.Clientset = d.clientSet
	copyOptions.ClientConfig = d.restConfig
	copyOptions.Namespace = namespace

	if err := copyOptions.Run([]string{pod + ":" + srcDir, dstDir}); err != nil {
		return fmt.Errorf("could not run copy operation: %v", err)
	}
	klog.V(4).InfoS("Copy from pod finished", "pod", pod, "namespace", namespace,
		"stdout", out, "stderr", errOut)
	return nil
}

func (d *managerDumper) copyLogFromPod(namespace, pod, dstDir string) error {
	if err := d.fs.MkdirAll(dstDir, os.ModePerm); err != nil {
		return fmt.Errorf("error when creating target dir: %w", err)
	}
	dstPath := path.Join(dstDir, pod+".log")
	targetFile, err := d.fs.Create(dstPath)
	if err != nil {
		return fmt.Errorf("error when creating target file %s: %w", dstPath, err)
	}
	defer targetFile.Close()

	req := d.clientSet.CoreV1().Pods(namespace).GetLogs(pod, &v1.PodLogOptions{})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return err
	}
	defer podLogs.Close()
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, podLogs); err != nil {
		return err
	}

	_, err = targetFile.WriteString(buf.String())
	return err
}

func timestampFilter(since string) *time.Time {
	var timeFilter *time.Time
	if since != "" {
		duration, _ := time.ParseDuration(since)
		start := time.Now().Add(-duration)
		timeFilter = &start
	}
	return timeFilter

}

// parseTimeFromFileName parse time from log file name.
// example log file format: <component>.<hostname>.<user>.log.<level>.<yyyymmdd>-<hhmmss>.1
func parseTimeFromFileName(name string) (time.Time, error) {
	ss := strings.Split(name, ".")
	ts := ss[len(ss)-2]
	return time.Parse("20060102-150405", ts)

}

// parseTimeFromLogLine parse timestamp from the log line.
// example(kubelet/agent/controller): "I0817 06:55:10.804384       1 shared_informer.go:270] caches populated"
// example(ovs): "2021-06-02T16:18:52.285Z|00004|reconnect|INFO|unix:/var/run/openvswitch/db.sock: connecting..."
// the first char indicates the log level.
func parseTimeFromLogLine(log string, year string, prefix string) (time.Time, error) {
	ss := strings.Split(log, ".")
	if ss[0] == "" {
		return time.Time{}, fmt.Errorf("log line is empty")
	}

	dateStr := year + ss[0][1:]
	layout := "20060102 15:04:05"
	if prefix == "ovs" {
		dateStr = ss[0]
		layout = "2006-01-02T15:04:05"
	}

	return time.Parse(layout, dateStr)

}

// directoryCopy copies files under the srcDir to the targetDir. Only files whose name matches
// the prefixFilter will be copied. If prefixFiler is "", no filter is performed. At the same time, if the timeFilter is set,
// only files whose modTime is later than the timeFilter will be copied. If a file contains both older logs and matched logs, only
// the matched logs will be copied. Copied files will be located under the same relative path.
func directoryCopy(fs afero.Fs, targetDir string, srcDir string, prefixFilter string, timeFilter *time.Time) error {
	err := fs.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error when creating target dir: %w", err)
	}
	return afero.Walk(fs, srcDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if prefixFilter != "" && !strings.HasPrefix(info.Name(), prefixFilter) {
			return nil
		}

		if timeFilter != nil && info.ModTime().Before(*timeFilter) {
			return nil
		}

		targetPath := path.Join(targetDir, info.Name())
		targetFile, err := fs.Create(targetPath)
		if err != nil {
			return fmt.Errorf("error when creating target file %s: %w", targetPath, err)
		}
		defer targetFile.Close()

		srcFile, err := fs.Open(filePath)
		if err != nil {
			return fmt.Errorf("error when opening source file %s: %w", filePath, err)
		}
		defer srcFile.Close()

		startTime, err := parseTimeFromFileName(info.Name())
		if timeFilter != nil {
			// if name contains timestamp, use it to find the first matched file. If not, such as ovs log file,
			// just parse the log file (usually there is only one log file for each component)
			if err == nil && startTime.Before(*timeFilter) || err != nil {
				data := ""
				scanner := bufio.NewScanner(srcFile)
				for scanner.Scan() {
					// the size limit of single log line is 64k. marked it as known issue and fix it if
					// error occurs
					line := scanner.Text()
					if data != "" {
						data += line + "\n"
					} else {
						ts, err := parseTimeFromLogLine(line, strconv.Itoa(timeFilter.Year()), prefixFilter)
						if err == nil {
							if !ts.Before(*timeFilter) {
								data += line + "\n"
							}
						}
					}
				}
				_, err = targetFile.WriteString(data)
				return err
			}
		}
		_, err = io.Copy(targetFile, srcFile)
		return err
	})
}
