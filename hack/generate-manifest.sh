#!/usr/bin/env bash

# Copyright 2022 Antrea Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eo pipefail

function echoerr {
    >&2 echo "$@"
}

_usage="Usage: $0 [--mode (dev|release)] [--keep] [--help|-h]
Generate a YAML manifest for the Clickhouse-Grafana Flow-visibility Solution, using Helm and
Kustomize, and print it to stdout.
        --mode (dev|release|antrea-e2e)     Choose the configuration variant that you need (default is 'dev')
                                            antrea-e2e mode generates YAML manifest for e2e tests in Antrea
                                            repository, which only includes a ClickHouse server with default
                                            credentials.
        --spark-operator                    Generate a manifest with Spark Operator enabled.
        --theia-manager                     Generate a manifest with Theia Manager enabled.
        --no-grafana                        Generate a manifest with Grafana disabled.
        --ch-size <size>                    Deploy the ClickHouse with a specific storage size. Can be a 
                                            plain integer or as a fixed-point number using one of these quantity
                                            suffixes: E, P, T, G, M, K. Or the power-of-two equivalents:
                                            Ei, Pi, Ti, Gi, Mi, Ki. (default is 8Gi)
        --ch-monitor-threshold <threshold>  Deploy the ClickHouse monitor with a specific threshold. Can
                                            vary from 0 to 1. (default is 0.5)
        --local <path>                      Create the PersistentVolume for Clickhouse DB with a provided
                                            local path.
This tool uses Helm 3 (https://helm.sh/) and Kustomize (https://github.com/kubernetes-sigs/kustomize)
to generate manifests for Theia. You can set the HELM and KUSTOMIZE environment variables to
the paths of the helm and kustomize binaries you want us to use. Otherwise we will download the
appropriate version of the helm and kustomize binaries and use them (this is the recommended
approach since different versions of helm and kustomize may create different output YAMLs)."

function print_usage {
    echoerr "$_usage"
}

function print_help {
    echoerr "Try '$0 --help' for more information."
}

MODE="dev"
SPARK_OP=false
THEIA_MANAGER=false
GRAFANA=true
CH_SIZE="8Gi"
CH_THRESHOLD=0.5
LOCALPATH=""

while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --mode)
    MODE="$2"
    shift 2
    ;;
    --spark-operator)
    SPARK_OP=true
    shift 1
    ;;
    --theia-manager)
    THEIA_MANAGER=true
    shift 1
    ;;
    --no-grafana)
    GRAFANA=false
    shift 1
    ;;
    --ch-size)
    CH_SIZE="$2"
    shift 2
    ;;
    --ch-monitor-threshold)
    CH_THRESHOLD="$2"
    shift 2
    ;;
    --local)
    LOCALPATH="$2"
    shift 2
    ;;
    -h|--help)
    print_usage
    exit 0
    ;;
    *)    # unknown option
    echoerr "Unknown option $1"
    exit 1
    ;;
esac
done

if [ "$MODE" != "dev" ] && [ "$MODE" != "release" ] && [ "$MODE" != "antrea-e2e" ] ; then
    echoerr "--mode must be one of 'dev', 'release' or 'antrea-e2e'"
    print_help
    exit 1
fi

if [ "$MODE" == "release" ] && [ -z "$IMG_NAME" ]; then
    echoerr "In 'release' mode, environment variable IMG_NAME must be set"
    print_help
    exit 1
fi

if [ "$MODE" == "release" ] && [ -z "$IMG_TAG" ]; then
    echoerr "In 'release' mode, environment variable IMG_TAG must be set"
    print_help
    exit 1
fi

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

source $THIS_DIR/verify-helm.sh

if [ -z "$HELM" ]; then
    HELM="$(verify_helm)"
elif ! $HELM version > /dev/null 2>&1; then
    echoerr "$HELM does not appear to be a valid helm binary"
    print_help
    exit 1
fi

HELM_VALUES=()

HELM_VALUES+=("clickhouse.storage.size=$CH_SIZE" "clickhouse.monitor.threshold=$CH_THRESHOLD")

if [ "$MODE" == "dev" ] && [ -n "$IMG_NAME" ]; then
    HELM_VALUES+=("clickhouse.monitor.image.repository=$IMG_NAME")
fi
if [ "$MODE" == "release" ]; then
    HELM_VALUES+=("clickhouse.monitor.image.repository=$IMG_NAME" "clickhouse.monitor.image.tag=$IMG_TAG")
    if [ $IMG_TAG == "v0.1.0" ] || [ $IMG_TAG == "v0.2.0" ]; then
        HELM_VALUES+=("clickhouse.image.tag=21.11")
    else
        HELM_VALUES+=("clickhouse.image.tag=$IMG_TAG")
    fi
fi
if [ "$MODE" == "antrea-e2e" ]; then
    HELM_VALUES+=("grafana.enable=false" "clickhouse.monitor.enable=false")
fi
if $SPARK_OP; then
    HELM_VALUES+=("sparkOperator.enable=true")
fi
if $THEIA_MANAGER; then
  HELM_VALUES+=("theiaManager.enable=true")
fi
if [ "$GRAFANA" == false ]; then
    HELM_VALUES+=("grafana.enable=false")
fi
if [[ $LOCALPATH != "" ]]; then
    HELM_VALUES+=("clickhouse.storage.createPersistentVolume.type=Local" "clickhouse.storage.createPersistentVolume.local.path=$LOCALPATH")
fi

delim=""
HELM_VALUES_OPTION=""
for v in "${HELM_VALUES[@]}"; do
    HELM_VALUES_OPTION="$HELM_VALUES_OPTION$delim$v"
    delim=","
done
if [ "$HELM_VALUES_OPTION" != "" ]; then
    HELM_VALUES_OPTION="--set $HELM_VALUES_OPTION"
fi

THEIA_CHART=$THIS_DIR/../build/charts/theia

# For antrea-e2e mode, extra Namespace resource is not required.
# The manifest can be generated with Helm directly.
if [ "$MODE" == "antrea-e2e" ]; then
    # Suppress potential Helm warnings about invalid permissions for Kubeconfig file
    # by throwing away related warnings.
    $HELM template \
      --namespace flow-visibility \
      $HELM_VALUES_OPTION \
      "$THEIA_CHART"\
      2> >(grep -v 'This is insecure' >&2)
    exit 0
fi

# For release or dev mode, generate an intermediate manifest by Helm,
# and add flow-visibility Namespace resource with Kustomize.
# Install Kustomize
source $THIS_DIR/verify-kustomize.sh
if [ -z "$KUSTOMIZE" ]; then
    KUSTOMIZE="$(verify_kustomize)"
elif ! $KUSTOMIZE version > /dev/null 2>&1; then
    echoerr "$KUSTOMIZE does not appear to be a valid kustomize binary"
    print_help
    exit 1
fi

KUSTOMIZATION_DIR=$THIS_DIR/../build/yamls/base
# intermediate manifest
MANIFEST=$KUSTOMIZATION_DIR/flow-visibility/manifest.yaml
# Suppress potential Helm warnings about invalid permissions for Kubeconfig file
# by throwing away related warnings.
$HELM template \
      --namespace flow-visibility \
      $HELM_VALUES_OPTION \
      "$THEIA_CHART"\
      2> >(grep -v 'This is insecure' >&2)\
      > $MANIFEST

cd $KUSTOMIZATION_DIR/flow-visibility

# This version number is used for ClickHouse data schema auto-migration.
# We ignore minor versions like dev versions.
VERSION=$(head -n1 $THIS_DIR/../VERSION)
VERSION=${VERSION:1} # strip leading 'v'
VERSION=${VERSION%-*} # strip "-dev" suffix if present
# Replace version placeholder
sed -i.bak "s/0\.0\.0/$VERSION/g" $MANIFEST

INCLUDE_CRD=false
if $SPARK_OP || $THEIA_MANAGER; then
    INCLUDE_CRD=true
    CRDS_DIR=$THIS_DIR/../build/charts/theia/crds
    TMP_DIR=$(mktemp -d $KUSTOMIZATION_DIR/overlays.XXXXXXXX)
    pushd $TMP_DIR > /dev/null
    touch kustomization.yml
    $KUSTOMIZE edit add base ../flow-visibility
fi
# Add Spark Operator CRDs with Kustomize
if $SPARK_OP; then
    mkdir sparkop
    cp $CRDS_DIR/spark-operator-crds.yaml sparkop/spark-operator-crds.yaml
    $KUSTOMIZE edit add base sparkop/spark-operator-crds.yaml
fi

# Add Theia Manager CRDs with Kustomize
if $THEIA_MANAGER; then
   mkdir manager
   cp $CRDS_DIR/network-policy-recommendation-crd.yaml manager/network-policy-recommendation-crd.yaml
   $KUSTOMIZE edit add base manager/network-policy-recommendation-crd.yaml
   cp $CRDS_DIR/anomaly-detector-crd.yaml manager/anomaly-detector-crd.yaml
   $KUSTOMIZE edit add base manager/anomaly-detector-crd.yaml
fi

$KUSTOMIZE build

# clean
if $INCLUDE_CRD; then
    popd > /dev/null
    rm -rf $TMP_DIR
fi
rm -rf $MANIFEST
