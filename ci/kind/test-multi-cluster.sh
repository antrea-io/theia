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

_usage="Usage: $0 [--help|-h]
        --help, -h                    Print this message and exit.
        --coverage                    Collect coverage data while running test.
"

function print_usage {
    echoerr -n "$_usage"
}


TESTBED_CMD=$(dirname $0)"/kind-setup.sh"
FLOW_VISIBILITY_CH_ONLY_CMD=$(dirname $0)"/../../hack/generate-manifest.sh --ch-only --node-port --secure-connection"
CH_OPERATOR_YML=$(dirname $0)"/../../build/charts/theia/crds/clickhouse-operator-install-bundle.yaml"

WORKDIR=$HOME
MULTICLUSTER_KUBECONFIG_PATH=$WORKDIR/.kube
EAST_CLUSTER_CONFIG="--kubeconfig=$MULTICLUSTER_KUBECONFIG_PATH/east"
WEST_CLUSTER_CONFIG="--kubeconfig=$MULTICLUSTER_KUBECONFIG_PATH/west"
CLUSTER_NAMES=("east" "west")

coverage=false
while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --coverage)
    coverage=true
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

if $coverage; then
  echo "creating coverage folder"
  mkdir -p .coverage/clickhouse-monitor-coverage/
  mkdir -p .coverage/theia-manager-coverage/
  mkdir -p .coverage/merged/
fi

COMMON_IMAGES_LIST=("registry.k8s.io/e2e-test-images/agnhost:2.29" \
                    "projects.registry.vmware.com/antrea/busybox"  \
                    "projects.registry.vmware.com/antrea/nginx:1.21.6-alpine" \
                    "projects.registry.vmware.com/antrea/perftool" \
                    "antrea/antrea-ubuntu:latest" \
                    "antrea/flow-aggregator:latest" \
                    "projects.registry.vmware.com/antrea/clickhouse-operator:0.21.0" \
                    "projects.registry.vmware.com/antrea/metrics-exporter:0.21.0" \
                    "projects.registry.vmware.com/antrea/theia-zookeeper:3.8.0")

for image in "${COMMON_IMAGES_LIST[@]}"; do
    echo "Pulling $image"
    for i in `seq 3`; do
        docker pull $image && break
        sleep 1
    done
done

COMMON_IMAGES_LIST+=("projects.registry.vmware.com/antrea/theia-clickhouse-monitor:latest"\
                     "projects.registry.vmware.com/antrea/theia-clickhouse-server:latest")

printf -v COMMON_IMAGES "%s " "${COMMON_IMAGES_LIST[@]}"

function setup_multi_cluster {
  args=$1

  echo "creating test bed with args $args"
  for i in {0..1}; do
      eval "timeout 600 $TESTBED_CMD create ${CLUSTER_NAMES[$i]} $args --num-workers 1"
  done

  for name in ${CLUSTER_NAMES[*]}; do
      kind get kubeconfig --name ${name} > ${MULTICLUSTER_KUBECONFIG_PATH}/${name}
  done
}

function clean_multicluster {
  echoerr "Cleaning testbed"
  for name in ${CLUSTER_NAMES[*]}; do
      $TESTBED_CMD destroy ${name}
  done
}

function wait_for_antrea_multicluster_pods_ready {
    kubeconfig=$1
    antreYML=$2
    kubectl apply -f $antreYML "${kubeconfig}"
    kubectl rollout restart deployment/coredns -n kube-system "${kubeconfig}"
    kubectl rollout status deployment/coredns -n kube-system "${kubeconfig}"
    kubectl rollout status deployment.apps/antrea-controller -n kube-system "${kubeconfig}"
    kubectl rollout status daemonset/antrea-agent -n kube-system "${kubeconfig}"
}

function run_test {
  TMP_DIR=$(mktemp -d $(dirname $0)/tmp.XXXXXXXX)
  curl -o $TMP_DIR/antrea.yml https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/antrea.yml
  sed -i -e "s|image: \"projects.registry.vmware.com/antrea/antrea-ubuntu:latest\"|image: \"antrea/antrea-ubuntu:latest\"|g" $TMP_DIR/antrea.yml
  sed -i -e "s/#  FlowExporter: false/  FlowExporter: true/g" $TMP_DIR/antrea.yml
  perl -i -p0e 's/      # feature, you need to set "enable" to true, and ensure that the FlowExporter\n      # feature gate is also enabled.\n      enable: false/      # feature, you need to set "enable" to true, and ensure that the FlowExporter\n      # feature gate is also enabled.\n      enable: true/' $TMP_DIR/antrea.yml
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

  eastNodeControlPlaneIP=$(kubectl get nodes -o wide --no-headers=true $EAST_CLUSTER_CONFIG | grep east-control-plane | awk '{print $6}')
  FLOW_VISIBILITY_CH_ONLY_CMD+=" --ipaddress $eastNodeControlPlaneIP"

  for name in ${CLUSTER_NAMES[*]}; do
      docker exec -i ${name}-control-plane dd of=/root/antrea.yml < $TMP_DIR/antrea.yml
      docker exec -i ${name}-control-plane dd of=/root/flow-aggregator.yml < $TMP_DIR/flow-aggregator.yml
      docker exec -i ${name}-control-plane dd of=/root/clickhouse-operator-install-bundle.yaml < $CH_OPERATOR_YML
      $FLOW_VISIBILITY_CH_ONLY_CMD | docker exec -i ${name}-control-plane dd of=/root/flow-visibility-ch-only.yml
  done

  wait_for_antrea_multicluster_pods_ready "${EAST_CLUSTER_CONFIG}" "$TMP_DIR/antrea.yml"
  wait_for_antrea_multicluster_pods_ready "${WEST_CLUSTER_CONFIG}" "$TMP_DIR/antrea.yml"

  rm -rf $TMP_DIR
  sleep 1

  if $coverage; then
    COVERAGE=true go test -v -timeout=45m antrea.io/theia/test/e2e_mc -provider=kind --logs-export-dir=$ANTREA_LOG_DIR -cover -covermode=atomic
  else
    go test -v -timeout=45m antrea.io/theia/test/e2e_mc -provider=kind --logs-export-dir=$ANTREA_LOG_DIR
  fi
}

function coverage_and_cleanup_test {
  if $coverage; then
    echo "generating coverage.txt file"
    touch .coverage/complete-multi-cluster-kind-e2e-coverage.txt
    go tool covdata merge -i=.coverage/clickhouse-monitor-coverage,.coverage/theia-manager-coverage -o .coverage/merged
    go tool covdata textfmt -i=.coverage/merged -o .coverage/complete-multi-cluster-kind-e2e-coverage.txt
    rm -rf .coverage/clickhouse-monitor-coverage
    rm -rf .coverage/theia-manager-coverage
    rm -rf .coverage/merged
  fi
}

trap clean_multicluster EXIT
setup_multi_cluster "--images \"$COMMON_IMAGES\""
run_test
coverage_and_cleanup_test

exit 0
