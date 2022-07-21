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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	sparkv1 "antrea.io/theia/third_party/sparkoperator/v1beta2"
)

// policyRecommendationStatusCmd represents the policy-recommendation status command
var policyRecommendationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of a policy recommendation Spark job",
	Long: `Check the current status of a policy recommendation Spark job by ID.
It will return the status of this Spark application like SUBMITTED, RUNNING, COMPLETED, or FAILED.`,
	Args: cobra.RangeArgs(0, 1),
	Example: `
Check the current status of job with ID e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation status --id e998433e-accb-4888-9fc8-06563f073e86
Or
$ theia policy-recommendation status e998433e-accb-4888-9fc8-06563f073e86
Use Service ClusterIP when checking the current status of job with ID e998433e-accb-4888-9fc8-06563f073e86
$ theia policy-recommendation status e998433e-accb-4888-9fc8-06563f073e86 --use-cluster-ip
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		recoID, err := cmd.Flags().GetString("id")
		if err != nil {
			return err
		}
		if recoID == "" && len(args) == 1 {
			recoID = args[0]
		}
		err = ParseRecommendationID(recoID)
		if err != nil {
			return err
		}
		kubeconfig, err := ResolveKubeConfig(cmd)
		if err != nil {
			return err
		}
		clientset, err := CreateK8sClient(kubeconfig)
		if err != nil {
			return fmt.Errorf("couldn't create k8s client using given kubeconfig, %v", err)
		}
		endpoint, err := cmd.Flags().GetString("clickhouse-endpoint")
		if err != nil {
			return err
		}
		if endpoint != "" {
			err = ParseEndpoint(endpoint)
			if err != nil {
				return err
			}
		}
		useClusterIP, err := cmd.Flags().GetBool("use-cluster-ip")
		if err != nil {
			return err
		}

		err = PolicyRecoPreCheck(clientset)
		if err != nil {
			return err
		}
		var state, errorMessage string
		// Check the ClickHouse first because completed jobs will store results in ClickHouse
		_, err = getPolicyRecommendationResult(clientset, kubeconfig, endpoint, useClusterIP, "", recoID)
		if err != nil {
			state, err = getPolicyRecommendationStatus(clientset, recoID)
			if err != nil {
				return err
			}
			if state == "" {
				state = "NEW"
			}
			if state == "RUNNING" {
				var endpoint string
				service := fmt.Sprintf("pr-%s-ui-svc", recoID)
				if useClusterIP {
					serviceIP, servicePort, err := GetServiceAddr(clientset, service)
					if err != nil {
						klog.V(2).ErrorS(err, "error when getting the progress of the job, cannot get Spark Monitor Service address")
					} else {
						endpoint = fmt.Sprintf("tcp://%s:%d", serviceIP, servicePort)
					}
				} else {
					servicePort := 4040
					listenAddress := "localhost"
					listenPort := 4040
					pf, err := StartPortForward(kubeconfig, service, servicePort, listenAddress, listenPort)
					if err != nil {
						klog.V(2).ErrorS(err, "error when getting the progress of the job, cannot forward port")
					} else {
						endpoint = fmt.Sprintf("http://%s:%d", listenAddress, listenPort)
						defer pf.Stop()
					}
				}
				// Check the working progress of running recommendation job
				if endpoint != "" {
					stateProgress, err := getPolicyRecommendationProgress(endpoint)
					if err != nil {
						klog.V(2).ErrorS(err, "failed to get the progress of the job")
					}
					state += stateProgress
				}
			}
			errorMessage, err = getPolicyRecommendationErrorMsg(clientset, recoID)
			if err != nil {
				return err
			}
		} else {
			state = "COMPLETED"
		}
		fmt.Printf("Status of this policy recommendation job is %s\n", state)
		if errorMessage != "" {
			fmt.Printf("Error message: %s\n", errorMessage)
		}
		return nil
	},
}

func getSparkAppByRecommendationID(clientset kubernetes.Interface, id string) (sparkApp sparkv1.SparkApplication, err error) {
	err = clientset.CoreV1().RESTClient().
		Get().
		AbsPath("/apis/sparkoperator.k8s.io/v1beta2").
		Namespace(flowVisibilityNS).
		Resource("sparkapplications").
		Name("pr-" + id).
		Do(context.TODO()).
		Into(&sparkApp)
	if err != nil {
		return sparkApp, err
	}
	return sparkApp, nil
}

func getPolicyRecommendationStatus(clientset kubernetes.Interface, id string) (string, error) {
	sparkApplication, err := getSparkAppByRecommendationID(clientset, id)
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(string(sparkApplication.Status.AppState.State))
	if state == "" {
		state = "NEW"
	}
	return state, nil
}

func getPolicyRecommendationErrorMsg(clientset kubernetes.Interface, id string) (string, error) {
	sparkApplication, err := getSparkAppByRecommendationID(clientset, id)
	if err != nil {
		return "", err
	}
	errorMessage := strings.TrimSpace(string(sparkApplication.Status.AppState.ErrorMessage))
	return errorMessage, nil
}

func getPolicyRecommendationProgress(baseUrl string) (string, error) {
	// Get the id of current Spark application
	url := fmt.Sprintf("%s/api/v1/applications", baseUrl)
	response, err := getResponseFromSparkMonitoringSvc(url)
	if err != nil {
		return "", fmt.Errorf("failed to get response from the Spark Monitoring Service: %v", err)
	}
	var getAppsResult []map[string]interface{}
	json.Unmarshal([]byte(response), &getAppsResult)
	if len(getAppsResult) != 1 {
		return "", fmt.Errorf("wrong Spark Application number, expected 1, got %d", len(getAppsResult))
	}
	sparkAppID := getAppsResult[0]["id"]
	// Check the percentage of completed stages
	url = fmt.Sprintf("%s/api/v1/applications/%s/stages", baseUrl, sparkAppID)
	response, err = getResponseFromSparkMonitoringSvc(url)
	if err != nil {
		return "", fmt.Errorf("failed to get response from the Spark Monitoring Service: %v", err)
	}
	var getStagesResult []map[string]interface{}
	json.Unmarshal([]byte(response), &getStagesResult)
	NumStageResult := len(getStagesResult)
	if NumStageResult < 1 {
		return "", fmt.Errorf("wrong Spark Application stages number, expected at least 1, got %d", NumStageResult)
	}
	completedStages := 0
	for _, stage := range getStagesResult {
		if stage["status"] == "COMPLETE" || stage["status"] == "SKIPPED" {
			completedStages++
		}
	}
	return fmt.Sprintf(": %d/%d (%d%%) stages completed", completedStages, NumStageResult, completedStages*100/NumStageResult), nil
}

func getResponseFromSparkMonitoringSvc(url string) ([]byte, error) {
	sparkMonitoringClient := http.Client{}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	var res *http.Response
	var getErr error
	connRetryInterval := 1 * time.Second
	connTimeout := 10 * time.Second
	if err := wait.PollImmediate(connRetryInterval, connTimeout, func() (bool, error) {
		res, err = sparkMonitoringClient.Do(request)
		if err != nil {
			getErr = err
			return false, nil
		}
		return true, nil
	}); err != nil {
		return nil, getErr
	}
	if res == nil {
		return nil, fmt.Errorf("response is nil")
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}
	return body, nil
}

func init() {
	policyRecommendationCmd.AddCommand(policyRecommendationStatusCmd)
	policyRecommendationStatusCmd.Flags().StringP(
		"id",
		"i",
		"",
		"ID of the policy recommendation Spark job.",
	)
}
