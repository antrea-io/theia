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

package controller

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

const (
	// Set resyncPeriod to 0 to disable resyncing.
	ResyncPeriod time.Duration = 0
	// How long to wait before retrying the processing of an Service change.
	MinRetryDelay = 5 * time.Second
	MaxRetryDelay = 300 * time.Second
	// Default number of workers processing an Service change.
	DefaultWorkers = 4
	// Time format for parsing input time
	InputTimeFormat  = "2006-01-02 15:04:05"
	K8sQuantitiesReg = "^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$"
	// Spark related parameters for Policy Recommendation and Throughput Anomaly Detection
	// spark jobs
	SparkImage           = "projects.registry.vmware.com/antrea/theia-spark-jobs:latest"
	SparkImagePullPolicy = "IfNotPresent"
	SparkServiceAccount  = "theia-spark"
	SparkVersion         = "3.1.1"
	SparkPort            = 4040
)

func ConstStrToPointer(constStr string) *string {
	return &constStr
}

func ValidateCluster(client kubernetes.Interface, namespace string) error {
	err := CheckPodByLabel(client, namespace, "app=clickhouse")
	if err != nil {
		return fmt.Errorf("failed to find the ClickHouse Pod, please check the deployment, error: %v", err)
	}
	err = CheckPodByLabel(client, namespace, "app.kubernetes.io/name=spark-operator")
	if err != nil {
		return fmt.Errorf("failed to find the Spark Operator Pod, please check the deployment, error: %v", err)
	}
	return nil
}

func CheckPodByLabel(client kubernetes.Interface, namespace string, label string) error {
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return fmt.Errorf("error %v when finding the Pod", err)
	}
	if len(pods.Items) < 1 {
		return fmt.Errorf("expected at least 1 pod, but found %d", len(pods.Items))
	}
	hasRunningPod := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Running" {
			hasRunningPod = true
			break
		}
	}
	if !hasRunningPod {
		return fmt.Errorf("can't find a running Pod")
	}
	return nil
}

func GetResponseFromSparkMonitoringSvc(url string) ([]byte, error) {
	sparkMonitoringClient := http.Client{}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := sparkMonitoringClient.Do(request)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("response is nil")
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}
	return body, nil
}

func GetSparkAppProgress(baseUrl string) (completedStages int, totalStages int, err error) {
	// Get the id of current Spark application
	url := fmt.Sprintf("%s/api/v1/applications", baseUrl)
	response, err := GetResponseFromSparkMonitoringSvc(url)
	if err != nil {
		return completedStages, totalStages, fmt.Errorf("failed to get response from the Spark Monitoring Service: %v", err)
	}
	var getAppsResult []map[string]interface{}
	json.Unmarshal([]byte(response), &getAppsResult)
	if len(getAppsResult) != 1 {
		return completedStages, totalStages, fmt.Errorf("wrong Spark Application number, expected 1, got %d", len(getAppsResult))
	}
	sparkAppID := getAppsResult[0]["id"]
	// Check the percentage of completed stages
	url = fmt.Sprintf("%s/api/v1/applications/%s/stages", baseUrl, sparkAppID)
	response, err = GetResponseFromSparkMonitoringSvc(url)
	if err != nil {
		return completedStages, totalStages, fmt.Errorf("failed to get response from the Spark Monitoring Service: %v", err)
	}
	var getStagesResult []map[string]interface{}
	json.Unmarshal([]byte(response), &getStagesResult)
	// totalStages can be 0 when the SparkApplication just starts and the stages have not be determined
	totalStages = len(getStagesResult)
	completedStages = 0
	for _, stage := range getStagesResult {
		if stage["status"] == "COMPLETE" || stage["status"] == "SKIPPED" {
			completedStages++
		}
	}
	return completedStages, totalStages, nil
}

func DeleteSparkResult(connect *sql.DB, query string, id string) (err error) {
	_, err = connect.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete throughput anomaly detector result with id %s: %v", id, err)
	}
	return nil
}

func GetPolicyRecommendationIds(connect *sql.DB) ([]string, error) {
	query := "SELECT DISTINCT id FROM recommendations;"
	rows, err := connect.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to read from ClickHouse: %v", err)
	}
	defer rows.Close()
	var idList []string
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("failed to parse id %s: %v", id, err)
		}
		idList = append(idList, id)
	}
	return idList, nil
}

func GetSparkApplication(client kubernetes.Interface, name string, namespace string) (sparkApp sparkv1.SparkApplication, err error) {
	err = client.CoreV1().RESTClient().Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Name(name).
		Do(context.TODO()).
		Into(&sparkApp)
	if err != nil {
		return sparkApp, err
	}
	return sparkApp, nil
}

func ListSparkApplicationWithLabel(client kubernetes.Interface, label string) (*sparkv1.SparkApplicationList, error) {
	sparkApplicationList := &sparkv1.SparkApplicationList{}
	err := client.CoreV1().RESTClient().Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Resource("sparkapplications").
		VersionedParams(&metav1.ListOptions{
			LabelSelector: label,
		}, scheme.ParameterCodec).
		Do(context.TODO()).Into(sparkApplicationList)
	return sparkApplicationList, err
}

func ListSparkApplication(client kubernetes.Interface, namespace string) (*sparkv1.SparkApplicationList, error) {
	sparkApplicationList := &sparkv1.SparkApplicationList{}
	err := client.CoreV1().RESTClient().Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Do(context.TODO()).Into(sparkApplicationList)
	return sparkApplicationList, err
}

func DeleteSparkApplication(client kubernetes.Interface, name string, namespace string) {
	client.CoreV1().RESTClient().Delete().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Name(name).
		Do(context.TODO())
}

func CreateSparkApplication(client kubernetes.Interface, namespace string, sparkApplication *sparkv1.SparkApplication) error {
	response := &sparkv1.SparkApplication{}
	return client.CoreV1().RESTClient().
		Post().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Body(sparkApplication).
		Do(context.TODO()).
		Into(response)
}

func GetSparkMonitoringSvcDNS(id string, namespace string, sparkPort int) string {
	return fmt.Sprintf("http://pr-%s-ui-svc.%s.svc:%d", id, namespace, sparkPort)
}
