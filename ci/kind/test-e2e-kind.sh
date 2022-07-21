#!/usr/bin/env bash

# Copyright 2020 Antrea Authors
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

# The script runs kind e2e tests with different traffic encapsulation modes.

set -eo pipefail

function echoerr {
    >&2 echo "$@"
}

_usage="Usage: $0 [--ip-family <v4|v6>] [--help|-h]
        --ip-family                   Configures the ipFamily for the KinD cluster.
        --skip                        A comma-separated list of keywords, with which tests should be skipped.
        --setup-only                  Only perform setting up the cluster and run test.
        --cleanup-only                Only perform cleaning up the cluster.
        --test-only                   Only run test on current cluster. Not set up/clean up the cluster.
        --help, -h                    Print this message and exit.
"

function print_usage {
    echoerr -n "$_usage"
}


TESTBED_CMD=$(dirname $0)"/kind-setup.sh"
YML_DIR=$(dirname $0)"/../../build/yamls"
FLOW_VISIBILITY_CMD=$(dirname $0)"/../../hack/generate-manifest.sh --ch-size 100Mi --ch-monitor-threshold 0.1"
FLOW_VISIBILITY_WITH_SPARK_CMD=$(dirname $0)"/../../hack/generate-manifest.sh --no-grafana --spark-operator"
CH_OPERATOR_YML=$(dirname $0)"/../../build/charts/theia/crds/clickhouse-operator-install-bundle.yaml"

make theia-linux
THEIACTL_BIN=$(dirname $0)"/../../bin/theia-linux"

function quit {
  result=$?
  if [[ $setup_only || $test_only ]]; then
    exit $result
  fi
  echoerr "Cleaning testbed"
  $TESTBED_CMD destroy kind
}

ipfamily="v4"
skiplist=""
setup_only=false
cleanup_only=false
test_only=false
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --ip-family)
    ipfamily="$2"
    shift 2
    ;;
    --skip)
    skiplist="$2"
    shift 2
    ;;
    --setup-only)
    setup_only=true
    shift
    ;;
    --cleanup-only)
    cleanup_only=true
    shift
    ;;
    --test-only)
    test_only=true
    shift
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

if [[ $cleanup_only == "true" ]];then
  $TESTBED_CMD destroy kind
  exit 0
fi

trap "quit" INT EXIT

COMMON_IMAGES_LIST=("k8s.gcr.io/e2e-test-images/agnhost:2.29" \
                    "projects.registry.vmware.com/antrea/busybox"  \
                    "projects.registry.vmware.com/antrea/nginx:1.21.6-alpine" \
                    "projects.registry.vmware.com/antrea/perftool" \
                    "antrea/antrea-ubuntu:latest" \
                    "antrea/flow-aggregator:latest" \
                    "projects.registry.vmware.com/antrea/theia-clickhouse-operator:0.18.2" \
                    "projects.registry.vmware.com/antrea/theia-metrics-exporter:0.18.2" \
                    "projects.registry.vmware.com/antrea/theia-clickhouse-server:21.11" \
                    "projects.registry.vmware.com/antrea/theia-clickhouse-monitor:latest" \
                    "projects.registry.vmware.com/antrea/theia-grafana:8.3.3" \
                    "projects.registry.vmware.com/antrea/theia-spark-operator:v1beta2-1.3.3-3.1.1")

for image in "${COMMON_IMAGES_LIST[@]}"; do
    for i in `seq 3`; do
        docker pull $image && break
        sleep 1
    done
done

COMMON_IMAGES_LIST+=("projects.registry.vmware.com/antrea/theia-policy-recommendation:latest")
printf -v COMMON_IMAGES "%s " "${COMMON_IMAGES_LIST[@]}"

function setup_cluster {
  args=$1

  if [[ "$ipfamily" == "v6" ]]; then
    args="$args --ip-family ipv6 --pod-cidr fd00:10:244::/56"
  elif [[ "$ipfamily" != "v4" ]]; then
    echoerr "invalid value for --ip-family \"$ipfamily\", expected \"v4\" or \"v6\""
    exit 1
  fi

  echo "creating test bed with args $args"
  eval "timeout 600 $TESTBED_CMD create kind $args"
}

function run_test {
  TMP_DIR=$(mktemp -d $(dirname $0)/tmp.XXXXXXXX)
  curl -o $TMP_DIR/antrea.yml https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/antrea.yml
  sed -i -e "s|image: \"projects.registry.vmware.com/antrea/antrea-ubuntu:latest\"|image: \"antrea/antrea-ubuntu:latest\"|g" $TMP_DIR/antrea.yml
  sed -i -e "s/#  FlowExporter: false/  FlowExporter: true/g" $TMP_DIR/antrea.yml
  sed -i -e "s/flowPollInterval: \"5s\"/flowPollInterval: \"1s\"/g" $TMP_DIR/antrea.yml
  sed -i -e "s/activeFlowExportTimeout: \"5s\"/activeFlowExportTimeout: \"2s\"/g" $TMP_DIR/antrea.yml
  sed -i -e "s/idleFlowExportTimeout: \"15s\"/idleFlowExportTimeout: \"1s\"/g" $TMP_DIR/antrea.yml

  curl -o $TMP_DIR/flow-aggregator.yml https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/flow-aggregator.yml
  sed -i -e "s|image: projects.registry.vmware.com/antrea/flow-aggregator:latest|image: antrea/flow-aggregator:latest|g" $TMP_DIR/flow-aggregator.yml
  perl -i -p0e 's/      # Enable is the switch to enable exporting flow records to ClickHouse.\n      enable: false/      # Enable is the switch to enable exporting flow records to ClickHouse.\n      enable: true/' $TMP_DIR/flow-aggregator.yml
  sed -i -e "s/    activeFlowRecordTimeout: 60s/    activeFlowRecordTimeout: 3500ms/g" $TMP_DIR/flow-aggregator.yml
  sed -i -e "s/    inactiveFlowRecordTimeout: 90s/    inactiveFlowRecordTimeout: 6s/g" $TMP_DIR/flow-aggregator.yml
  sed -i -e "s/      podLabels: false/      podLabels: true/g" $TMP_DIR/flow-aggregator.yml
  sed -i -e "s/      commitInterval: \"8s\"/      commitInterval: \"1s\"/g" $TMP_DIR/flow-aggregator.yml

  docker exec -i kind-control-plane dd of=/root/antrea.yml < $TMP_DIR/antrea.yml
  docker exec -i kind-control-plane dd of=/root/flow-aggregator.yml < $TMP_DIR/flow-aggregator.yml
  docker exec -i kind-control-plane dd of=/root/clickhouse-operator-install-bundle.yaml < $CH_OPERATOR_YML
  $FLOW_VISIBILITY_CMD | docker exec -i kind-control-plane dd of=/root/flow-visibility.yml
  $FLOW_VISIBILITY_WITH_SPARK_CMD | docker exec -i kind-control-plane dd of=/root/flow-visibility-with-spark.yml

  docker exec -i kind-control-plane dd of=/root/theia < $THEIACTL_BIN

  rm -rf $TMP_DIR
  sleep 1

  go test -v -timeout=20m antrea.io/theia/test/e2e -provider=kind --logs-export-dir=$ANTREA_LOG_DIR --skip=$skiplist
}

echo "======== Test encap mode =========="
if [[ $test_only == "false" ]];then
  setup_cluster "--images \"$COMMON_IMAGES\""
fi
run_test

rm -rf $PWD/bin

exit 0
