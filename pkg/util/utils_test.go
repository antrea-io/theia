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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	npName  = "pr-1234abcd-1234-abcd-12ab-12345678abcd"
	tadName = "tad-1234abcd-1234-abcd-12ab-12345678abcd"
)

func TestParseRecommendationName(t *testing.T) {
	testCases := []struct {
		name             string
		npName           string
		expectedErrorMsg string
	}{
		{
			name:             "Valid case",
			npName:           npName,
			expectedErrorMsg: "",
		},
		{
			name:             "Invalid name",
			npName:           "mock_name",
			expectedErrorMsg: "not a valid policy recommendation job name",
		},
		{
			name:             "Invalid uuid",
			npName:           "pr-name",
			expectedErrorMsg: "does not contain a valid UUID",
		},
	}
	for _, tt := range testCases {
		err := ParseRecommendationName(tt.npName)
		if err != nil {
			assert.Contains(t, err.Error(), tt.expectedErrorMsg)
		}
	}
}

func TestParseADAlgorithmID(t *testing.T) {
	testCases := []struct {
		name             string
		tadName          string
		expectedErrorMsg string
	}{
		{
			name:             "Valid case",
			tadName:          tadName,
			expectedErrorMsg: "",
		},
		{
			name:             "Invalid name",
			tadName:          "mock_name",
			expectedErrorMsg: "not a valid Throughput Anomaly Detection job name",
		},
		{
			name:             "Invalid uuid",
			tadName:          "tad-name",
			expectedErrorMsg: "does not contain a valid UUID",
		},
	}
	for _, tt := range testCases {
		err := ParseADAlgorithmID(tt.tadName)
		if err != nil {
			assert.Contains(t, err.Error(), tt.expectedErrorMsg)
		}
	}
}

func TestParseADAlgorithmName(t *testing.T) {
	testCases := []struct {
		name             string
		algoName         string
		expectedErrorMsg string
	}{
		{
			name:             "Valid case",
			algoName:         "ARIMA",
			expectedErrorMsg: "",
		},
		{
			name:             "Invalid name",
			algoName:         "mock_name",
			expectedErrorMsg: "not a valid Throughput Anomaly Detection algorithm name",
		},
	}
	for _, tt := range testCases {
		err := ParseADAlgorithmName(tt.algoName)
		if err != nil {
			assert.Contains(t, err.Error(), tt.expectedErrorMsg)
		}
	}
}
