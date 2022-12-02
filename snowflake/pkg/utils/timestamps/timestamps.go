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

package timestamps

import (
	"fmt"
	"strings"
	"time"
)

func ParseTimestamp(t string, now time.Time) (string, error) {
	ts, err := func() (time.Time, error) {
		fields := strings.Split(t, "-")
		if len(fields) == 0 {
			return now, nil
		}
		if len(fields) > 1 && fields[0] != "now" {
			return now, fmt.Errorf("bad timestamp: %s", t)
		}
		if len(fields) == 1 {
			return now, nil
		}
		if len(fields) == 2 {
			d, err := time.ParseDuration(fields[1])
			if err != nil {
				return now, fmt.Errorf("bad timestamp: %s", t)
			}
			return now.Add(-d), nil
		}
		return now, fmt.Errorf("bad timestamp: %s", t)
	}()
	if err != nil {
		return "", err
	}
	return ts.UTC().Format(time.RFC3339), nil
}
