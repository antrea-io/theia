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

package util

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func ParseRecommendationName(npName string) error {
	if !strings.HasPrefix(npName, "pr-") {
		return fmt.Errorf("input name %s is not a valid policy recommendation job name", npName)

	}
	id := npName[3:]
	_, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("input name %s does not contain a valid UUID, parsing error: %v", npName, err)
	}
	return nil
}

func ParseADAlgorithmName(algoName string) error {
	switch algoName {
	case
		"EWMA",
		"ARIMA",
		"DBSCAN":
		return nil
	}
	return fmt.Errorf("input name %s is not a valid Throughput Anomaly Detection algorithm name", algoName)
}

func ParseADAlgorithmID(tadName string) error {
	if !strings.HasPrefix(tadName, "tad-") {
		return fmt.Errorf("input name %s is not a valid Throughput Anomaly Detection job name", tadName)
	}
	id := tadName[4:]
	_, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("input name %s does not contain a valid UUID, parsing error: %v", tadName, err)
	}
	return nil
}
