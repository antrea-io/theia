#!/usr/bin/env bash

# Copyright 2022 Antrea Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

GOPATH=`go env GOPATH`
THEIA_PKG="antrea.io/theia"

function reset_year_change {
  set +x
  echo "=== Start resetting changes introduced by YEAR ==="
  # The call to 'tac' ensures that we cannot have concurrent git processes, by
  # waiting for the call to 'git diff  --numstat' to complete before iterating
  # over the files and calling 'git diff ${file}'.
  git diff  --numstat | awk '$1 == "1" && $2 == "1" {print $3}' | tac | while read file; do
    if [[ "$(git diff ${file})" == *"-// Copyright "*" Antrea Authors"* ]]; then
      git checkout HEAD -- "${file}"
      echo "=== ${file} is reset ==="
    fi
  done
}

# Generate clientset and apis code with K8s codegen tools.
$GOPATH/bin/client-gen \
  --clientset-name versioned \
  --input-base "${THEIA_PKG}/pkg/apis/" \
  --input "crd/v1alpha1" \
  --output-package "${THEIA_PKG}/pkg/client/clientset" \
  --go-header-file hack/boilerplate/license_header.go.txt

# Generate listers with K8s codegen tools.
$GOPATH/bin/lister-gen \
  --input-dirs "${THEIA_PKG}/pkg/apis/crd/v1alpha1" \
  --output-package "${THEIA_PKG}/pkg/client/listers" \
  --go-header-file hack/boilerplate/license_header.go.txt

# Generate informers with K8s codegen tools.
$GOPATH/bin/informer-gen \
  --input-dirs "${THEIA_PKG}/pkg/apis/crd/v1alpha1" \
  --versioned-clientset-package "${THEIA_PKG}/pkg/client/clientset/versioned" \
  --listers-package "${THEIA_PKG}/pkg/client/listers" \
  --output-package "${THEIA_PKG}/pkg/client/informers" \
  --go-header-file hack/boilerplate/license_header.go.txt

$GOPATH/bin/deepcopy-gen \
  --input-dirs "${THEIA_PKG}/pkg/apis/intelligence/v1alpha1" \
  --input-dirs "${THEIA_PKG}/pkg/apis/system/v1alpha1" \
  --input-dirs "${THEIA_PKG}/pkg/apis/crd/v1alpha1" \
  --input-dirs "${THEIA_PKG}/pkg/apis/stats/v1alpha1" \
  --input-dirs "${THEIA_PKG}/pkg/apis/anomalydetector/v1alpha1" \
  -O zz_generated.deepcopy \
  --go-header-file hack/boilerplate/license_header.go.txt

reset_year_change
