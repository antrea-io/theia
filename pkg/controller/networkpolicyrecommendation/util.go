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

package networkpolicyrecommendation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	crdv1alpha1 "antrea.io/theia/pkg/apis/crd/v1alpha1"
	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

func constStrToPointer(constStr string) *string {
	return &constStr
}

func validateCluster(client kubernetes.Interface, namespace string) error {
	err := checkPodByLabel(client, namespace, "app=clickhouse")
	if err != nil {
		return fmt.Errorf("failed to find the ClickHouse Pod, please check the deployment, error: %v", err)
	}
	err = checkPodByLabel(client, namespace, "app.kubernetes.io/name=spark-operator")
	if err != nil {
		return fmt.Errorf("failed to find the Spark Operator Pod, please check the deployment, error: %v", err)
	}
	return nil
}

func checkPodByLabel(client kubernetes.Interface, namespace string, label string) error {
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

func getPolicyRecommendationStatus(client kubernetes.Interface, id string, namespace string) (state string, errorMessage string, err error) {
	sparkApplication, err := GetSparkApplication(client, "pr-"+id, namespace)
	if err != nil {
		return state, errorMessage, err
	}
	state = strings.TrimSpace(string(sparkApplication.Status.AppState.State))
	errorMessage = strings.TrimSpace(string(sparkApplication.Status.AppState.ErrorMessage))

	return state, errorMessage, nil
}

func getResponseFromSparkMonitoringSvc(url string) ([]byte, error) {
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

func getPolicyRecommendationProgress(baseUrl string) (completedStages int, totalStages int, err error) {
	// Get the id of current Spark application
	url := fmt.Sprintf("%s/api/v1/applications", baseUrl)
	response, err := getResponseFromSparkMonitoringSvc(url)
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
	response, err = getResponseFromSparkMonitoringSvc(url)
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

func getPolicyRecommendationResult(connect *sql.DB, id string) (*crdv1alpha1.RecommendedNetworkPolicy, error) {
	var resultType, resultTimeCreatedStr, resultYamls string
	query := "SELECT type, timeCreated, yamls FROM recommendations WHERE id = (?);"
	err := connect.QueryRow(query, id).Scan(&resultType, &resultTimeCreatedStr, &resultYamls)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendation result with id %s: %v", id, err)
	}
	resultTimeCreated, err := time.Parse(clickHouseTimeFormat, resultTimeCreatedStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timeCreated of the result for %s: %v", id, err)
	}
	recommendedNetworkPolicy := &crdv1alpha1.RecommendedNetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "rnp-",
		},
		Spec: crdv1alpha1.RecommendedNetworkPolicySpec{
			Id:          id,
			Type:        resultType,
			TimeCreated: metav1.NewTime(resultTimeCreated),
			Yamls:       resultYamls,
		},
	}
	return recommendedNetworkPolicy, nil
}

func deleteSparkApplicationIfExists(client kubernetes.Interface, namespace string, id string) error {
	sparkApplicationList, err := ListSparkApplication(client, namespace)
	if err != nil {
		return err
	}
	existed := false
	for _, sparkApplication := range sparkApplicationList.Items {
		existedId := sparkApplication.ObjectMeta.Name[3:]
		if existedId == id {
			existed = true
		}
	}
	if existed {
		DeleteSparkApplication(client, "pr-"+id, namespace)
	}
	return nil
}

func deletePolicyRecommendationResult(connect *sql.DB, id string) (err error) {
	query := "ALTER TABLE recommendations_local ON CLUSTER '{cluster}' DELETE WHERE id = (?);"
	_, err = connect.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete recommendation result with id %s: %v", id, err)
	}
	return nil
}

func getSparkApplication(client kubernetes.Interface, name string, namespace string) (sparkApp sparkv1.SparkApplication, err error) {
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

func listSparkApplication(client kubernetes.Interface, namespace string) (*sparkv1.SparkApplicationList, error) {
	sparkApplicationList := &sparkv1.SparkApplicationList{}
	err := client.CoreV1().RESTClient().Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Do(context.TODO()).Into(sparkApplicationList)
	return sparkApplicationList, err
}

func deleteSparkApplication(client kubernetes.Interface, name string, namespace string) {
	client.CoreV1().RESTClient().Delete().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Name(name).
		Do(context.TODO())
}

func createSparkApplication(client kubernetes.Interface, namespace string, recommendationApplication *sparkv1.SparkApplication) error {
	response := &sparkv1.SparkApplication{}
	return client.CoreV1().RESTClient().
		Post().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(namespace).
		Resource("sparkapplications").
		Body(recommendationApplication).
		Do(context.TODO()).
		Into(response)
}

type IlleagelArguementError struct {
	error
}

func getSparkMonitoringSvcDNS(id string, namespace string) string {
	return fmt.Sprintf("http://pr-%s-ui-svc.%s.svc:%d", id, namespace, sparkPort)
}
