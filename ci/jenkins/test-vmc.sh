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

set -eo pipefail

function echoerr {
    >&2 echo "$@"
}

CLUSTER=""
DEFAULT_WORKDIR="/var/lib/jenkins"
DEFAULT_KUBECONFIG_PATH=$DEFAULT_WORKDIR/.kube/config
WORKDIR=$DEFAULT_WORKDIR
KUBECONFIG_PATH=$DEFAULT_KUBECONFIG_PATH
MODE="report"
RUN_GARBAGE_COLLECTION=false
RUN_SETUP_ONLY=false
RUN_CLEANUP_ONLY=false
COVERAGE=false
RUN_TEST_ONLY=false
TESTCASE=""
CODECOV_TOKEN=""
SECRET_EXIST=false
TEST_FAILURE=false
CLUSTER_READY=false
DOCKER_REGISTRY=""
# TODO: change to "control-plane" when testbeds are updated to K8s v1.20
CONTROL_PLANE_NODE_ROLE="master"

_usage="Usage: $0 [--cluster-name <VMCClusterNameToUse>] [--kubeconfig <KubeconfigSavePath>] [--workdir <HomePath>]
                  [--log-mode <SonobuoyResultLogLevel>] [--testcase <e2e|conformance|all-features-conformance|whole-conformance|networkpolicy>]
                  [--garbage-collection] [--setup-only] [--cleanup-only] [--coverage] [--test-only] [--registry]

Setup a VMC cluster to run K8s e2e community tests (E2e, Conformance, all features Conformance, whole Conformance & Network Policy).

        --cluster-name           The cluster name to be used for the generated VMC cluster.
        --kubeconfig             Path to save kubeconfig of generated VMC cluster.
        --workdir                Home path for Go, vSphere information and antrea_logs during cluster setup. Default is $WORKDIR.
        --log-mode               Use the flag to set either 'report', 'detail', or 'dump' level data for sonobouy results.
        --testcase               The testcase to run: e2e, conformance, all-features-conformance, whole-conformance or networkpolicy.
        --garbage-collection     Do garbage collection to clean up some unused testbeds.
        --setup-only             Only perform setting up the cluster and run test.
        --cleanup-only           Only perform cleaning up the cluster.
        --coverage               Run e2e with coverage.
        --test-only              Only run test on current cluster. Not set up/clean up the cluster.
        --codecov-token          Token used to upload coverage report(s) to Codecov.
        --registry               Using private registry to pull images."

function print_usage {
    echoerr "$_usage"
}

function print_help {
    echoerr "Try '$0 --help' for more information."
}

while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --cluster-name)
    CLUSTER="$2"
    shift 2
    ;;
    --kubeconfig)
    KUBECONFIG_PATH="$2"
    shift 2
    ;;
    --k8s-version)
    K8S_VERSION="$2"
    shift 2
    ;;
    --workdir)
    WORKDIR="$2"
    shift 2
    ;;
    --log-mode)
    MODE="$2"
    shift 2
    ;;
    --testcase)
    TESTCASE="$2"
    shift 2
    ;;
    --registry)
    DOCKER_REGISTRY="$2"
    shift 2
    ;;
    --username)
    CLUSTER_USERNAME="$2"
    shift 2
    ;;
    --password)
    CLUSTER_PASSWORD="$2"
    shift 2
    ;;
    --garbage-collection)
    RUN_GARBAGE_COLLECTION=true
    shift
    ;;
    --setup-only)
    RUN_SETUP_ONLY=true
    shift
    ;;
    --cleanup-only)
    RUN_CLEANUP_ONLY=true
    shift
    ;;
    --coverage)
    COVERAGE=true
    shift
    ;;
    --test-only)
    RUN_TEST_ONLY=true
    shift
    ;;
    --codecov-token)
    CODECOV_TOKEN="$2"
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

if [[ "$WORKDIR" != "$DEFAULT_WORKDIR" && "$KUBECONFIG_PATH" == "$DEFAULT_KUBECONFIG_PATH" ]]; then
    KUBECONFIG_PATH=$WORKDIR/.kube/config
fi

# If DOCKER_REGISTRY is non null, we ensure that "make" commands never pull from docker.io.
NO_PULL=
if [[ ${DOCKER_REGISTRY} != "" ]]; then
    NO_PULL=1
fi
export NO_PULL

function saveLogs() {
    echo "=== Truncate old logs ==="
    mkdir -p $WORKDIR/antrea_logs
    LOG_DIR=$WORKDIR/antrea_logs
    find ${LOG_DIR}/* -type d -mmin +10080 | xargs -r rm -rf

    CLUSTER_LOG_DIR="${LOG_DIR}/${CLUSTER}"
    echo "=== Saving capi logs ==="
    mkdir -p ${CLUSTER_LOG_DIR}/capi
    kubectl get -n capi-system pods -o name | awk '{print $1}' | while read capi_pod; do
        capi_pod_name=$(echo ${capi_pod} | cut -d'/' -f 2)
        kubectl logs ${capi_pod_name} -c manager -n capi-system > ${CLUSTER_LOG_DIR}/capi/${capi_pod_name} || true
    done

    echo "=== Saving capv logs ==="
    mkdir -p ${CLUSTER_LOG_DIR}/capv
    kubectl get -n capv-system pods -o name | awk '{print $1}' | while read capv_pod; do
        capv_pod_name=$(echo ${capv_pod} | cut -d'/' -f 2)
        kubectl logs ${capv_pod_name} -c manager -n capv-system > ${CLUSTER_LOG_DIR}/capv/${capv_pod_name} || true
    done

    echo "=== Saving cluster_api.yaml ==="
    mkdir -p ${CLUSTER_LOG_DIR}/cluster_api
    kubectl get cluster-api -A -o yaml > ${CLUSTER_LOG_DIR}/cluster_api/cluster_api.yaml || true
}

function release_static_ip() {
    echo '=== Releasing IP ==='
    cat "$DEFAULT_WORKDIR/host-local.json" | CNI_COMMAND=DEL CNI_CONTAINERID="$CLUSTER" CNI_NETNS=/dev/null CNI_IFNAME=dummy0 CNI_PATH=. /usr/bin/host-local
    echo Released IP
}

function setup_cluster() {
    export KUBECONFIG=$KUBECONFIG_PATH
    if [ -z $K8S_VERSION ]; then
      export K8S_VERSION=v1.19.1
    fi
    if [ -z $TEST_OS ]; then
      export TEST_OS=ubuntu-1804
    fi
    export OVA_TEMPLATE_NAME=${TEST_OS}-kube-${K8S_VERSION}
    rm -rf ${GIT_CHECKOUT_DIR}/jenkins || true

    echo '=== Allocating IP ==='
    cat "$DEFAULT_WORKDIR/host-local.json" | CNI_COMMAND=ADD CNI_CONTAINERID="$CLUSTER" CNI_NETNS=/dev/null CNI_IFNAME=dummy0 CNI_PATH=. /usr/bin/host-local > ip-result.json
    CONTROL_VIP=$(cat ip-result.json | jq -r '.ips[0].address' | awk -F '/' '{print $1}')
    echo Allocated "$CONTROL_VIP"

    echo '=== Generate key pair ==='
    mkdir -p ${GIT_CHECKOUT_DIR}/jenkins/key
    ssh-keygen -b 2048 -t rsa -f  "${GIT_CHECKOUT_DIR}/jenkins/key/antrea-ci-key" -q -N ""
    publickey="$(cat ${GIT_CHECKOUT_DIR}/jenkins/key/antrea-ci-key.pub)"

    echo "=== namespace value substitution ==="
    mkdir -p ${GIT_CHECKOUT_DIR}/jenkins/out
    cp ${GIT_CHECKOUT_DIR}/ci/cluster-api/vsphere/templates/* ${GIT_CHECKOUT_DIR}/jenkins/out
    sed -i "s/CLUSTERNAMESPACE/${CLUSTER}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/K8SVERSION/${K8S_VERSION}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/OVATEMPLATENAME/${OVA_TEMPLATE_NAME}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/CLUSTERNAME/${CLUSTER}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s|SSHAUTHORIZEDKEYS|${publickey}|g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/CLUSTERUSERNAME/${CLUSTER_USERNAME}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/CLUSTERPASSWORD/${CLUSTER_PASSWORD}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/CONTROLVIP/${CONTROL_VIP}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/CLUSTERNAMESPACE/${CLUSTER}/g" ${GIT_CHECKOUT_DIR}/jenkins/out/namespace.yaml


    # replace some common vc settings for all cluster.
    sed -i "s/DATASTORE/$DATASTORE/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    sed -i "s/VMFOLDERNAME/$VMFOLDERNAME/g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml


    echo "=== network spec value substitution==="
    index="$(($BUILD_NUMBER % 2))"
    cluster_defaults="${WORKDIR}/utils/CLUSTERDEFAULTS-${index}"
    while IFS= read -r line; do
        IFS='=' read -ra kv <<< "$line"
        sed -i "s|${kv[0]}|${kv[1]}|g" ${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml
    done < "$cluster_defaults"

    echo '=== Create a cluster in management cluster ==='
    kubectl apply -f "${GIT_CHECKOUT_DIR}/jenkins/out/namespace.yaml"
    kubectl apply -f "${GIT_CHECKOUT_DIR}/jenkins/out/cluster.yaml"

    echo '=== Wait for 20 min to get workload cluster secret ==='
    for t in {1..20}
    do
        sleep 1m
        echo '=== Get kubeconfig (try for 1m) ==='
        if kubectl get secret/${CLUSTER}-kubeconfig -n${CLUSTER} ; then
            kubectl get secret/${CLUSTER}-kubeconfig -n${CLUSTER} -o json \
            | jq -r .data.value \
            | base64 --decode \
            > "${GIT_CHECKOUT_DIR}/jenkins/out/kubeconfig"
            SECRET_EXIST=true
            break
        fi
    done

    if [[ "$SECRET_EXIST" == false ]]; then
        echo "=== Failed to get secret ==="
        saveLogs
        kubectl delete ns ${CLUSTER}
        exit 1
    else
        export KUBECONFIG="${GIT_CHECKOUT_DIR}/jenkins/out/kubeconfig"
        echo "=== Waiting for 20 minutes for all nodes to be up ==="

        set +e
        for t in {1..20}
        do
            sleep 1m
            echo "=== Get node (try for 1m) ==="
            mdNum="$(kubectl get node | grep -c ${CLUSTER}-md)"
            if [ "${mdNum}" == "2" ]; then
                echo "=== Setup workload cluster succeeded ==="
                CLUSTER_READY=true
                break
            fi
        done
        set -e

        if [[ "$CLUSTER_READY" == false ]]; then
            echo "=== Failed to bring up all the nodes ==="
            saveLogs
            KUBECONFIG=$KUBECONFIG_PATH kubectl delete ns ${CLUSTER}
            exit 1
        fi
    fi
}


function copy_image {
  filename=$1
  image=$2
  IP=$3
  version=$4
  need_cleanup=$5
  ${SCP_WITH_ANTREA_CI_KEY} $filename capv@${IP}:/home/capv
  if [ $TEST_OS == 'centos-7' ]; then
      ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "sudo chmod 777 /run/containerd/containerd.sock"
      if [[ $need_cleanup == 'true' ]]; then
          ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "sudo crictl images | grep $image | awk '{print \$3}' | uniq | xargs -r crictl rmi"
      fi
      ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "ctr -n=k8s.io images import /home/capv/$filename ; ctr -n=k8s.io images tag $image:$version $image:latest --force"
  else
      if [[ $need_cleanup == 'true' ]]; then
          ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "sudo crictl images | grep $image | awk '{print \$3}' | uniq | xargs -r crictl rmi"
      fi
      ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "sudo ctr -n=k8s.io images import /home/capv/$filename ; sudo ctr -n=k8s.io images tag $image:$version $image:latest --force"
  fi
  ${SSH_WITH_ANTREA_CI_KEY} -n capv@${IP} "sudo crictl images | grep '<none>' | awk '{print \$3}' | xargs -r crictl rmi"
}



function deliver_antrea {
    wget -c  https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/antrea.yml -O  ${GIT_CHECKOUT_DIR}/build/yamls/antrea.yml
    sed -i -e "s/#  FlowExporter: false/  FlowExporter: true/g" ${GIT_CHECKOUT_DIR}/build/yamls/antrea.yml
    antrea_yml="antrea.yml"
    # Enable verbose log for troubleshooting.
    sed -i "s/--v=0/--v=4/g" $GIT_CHECKOUT_DIR/build/yamls/$antrea_yml
    sed -i -e "s/flowPollInterval: \"5s\"/flowPollInterval: \"1s\"/g" $GIT_CHECKOUT_DIR/build/yamls/$antrea_yml
    sed -i -e "s/activeFlowExportTimeout: \"5s\"/activeFlowExportTimeout: \"2s\"/g" $GIT_CHECKOUT_DIR/build/yamls/$antrea_yml
    sed -i -e "s/idleFlowExportTimeout: \"15s\"/idleFlowExportTimeout: \"1s\"/g" $GIT_CHECKOUT_DIR/build/yamls/$antrea_yml

    wget -c https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/flow-aggregator.yml -O ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml

    # sed -i '215s/.*/      enable: true/' ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml
    perl -0777 -i.original -pe 's/    clickHouse:\n      # Enable is the switch to enable exporting flow records to ClickHouse.\n      #enable: false/    clickHouse:\n      # Enable is the switch to enable exporting flow records to ClickHouse.\n      enable: true/igs' $GIT_CHECKOUT_DIR/build/yamls/flow-aggregator.yml
    sed -i -e "s/      #podLabels: false/      podLabels: true/g" ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml
    sed -i -e "s/      #commitInterval: \"8s\"/      commitInterval: \"1s\"/g" ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml
    sed -i -e "s/    #activeFlowRecordTimeout: 60s/    activeFlowRecordTimeout: 3500ms/g" ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml
    sed -i -e "s/    #inactiveFlowRecordTimeout: 90s/    inactiveFlowRecordTimeout: 6s/g" ${GIT_CHECKOUT_DIR}/build/yamls/flow-aggregator.yml

    control_plane_ip="$(kubectl get nodes -o wide --no-headers=true | awk -v role="$CONTROL_PLANE_NODE_ROLE" '$3 ~ role {print $6}')"


    ${SCP_WITH_ANTREA_CI_KEY} $GIT_CHECKOUT_DIR/build/yamls/*.yml capv@${control_plane_ip}:~




    # copy images
    docker pull projects.registry.vmware.com/antrea/antrea-ubuntu:latest
    docker pull projects.registry.vmware.com/antrea/flow-aggregator:latest
    docker save -o antrea-ubuntu.tar projects.registry.vmware.com/antrea/antrea-ubuntu:latest
    docker save -o flow-aggregator.tar projects.registry.vmware.com/antrea/flow-aggregator:latest

    # not sure the exact image tag, so read from yaml
    # and we assume the image tag is the same for all images in this yaml
    image_tag="latest"
    while read -r line; do
        image=$(cut -d ':' -f2- <<< "$line")
        docker pull $image
        image_name=$(echo $image |  awk -F ":" '{print $1}' | awk -F "/" '{print $3}')
        image_tag=$(echo $image | awk -F ":" '{print $2}')
        docker save -o $image_name.tar $image
    done < <(grep "image:" ${GIT_CHECKOUT_DIR}/build/yamls/clickhouse-operator-install-bundle.yml)

    IPs=($(kubectl get nodes -o wide --no-headers=true | awk '{print $6}' | xargs))
    for i in "${!IPs[@]}"
    do
        ssh-keygen -f "/var/lib/jenkins/.ssh/known_hosts" -R ${IPs[$i]}
        copy_image antrea-ubuntu.tar projects.registry.vmware.com/antrea/antrea-ubuntu ${IPs[$i]} latest true
        copy_image flow-aggregator.tar projects.registry.vmware.com/antrea/flow-aggregator ${IPs[$i]} latest  true
        copy_image theia-clickhouse-operator.tar projects.registry.vmware.com/antrea/theia-clickhouse-operator  ${IPs[$i]} $image_tag true
        copy_image theia-metrics-exporter.tar projects.registry.vmware.com/antrea/theia-metrics-exporter  ${IPs[$i]} $image_tag true
    done

}

function run_e2e {
    echo "====== Running Antrea E2E Tests ======"

    export GO111MODULE=on
    export GOPATH=$WORKDIR/go
    export GOROOT=/usr/local/go
    export GOCACHE=$WORKDIR/.cache/go-build
    export GOMODCACHE=$WORKDIR/.cache/go-mod-cache
    export PATH=$GOROOT/bin:$PATH
    export KUBECONFIG=$GIT_CHECKOUT_DIR/jenkins/out/kubeconfig

    mkdir -p $GIT_CHECKOUT_DIR/test/e2e/infra/vagrant/playbook/kube
    CLUSTER_KUBECONFIG="${GIT_CHECKOUT_DIR}/jenkins/out/kubeconfig"
    CLUSTER_SSHCONFIG="${GIT_CHECKOUT_DIR}/jenkins/out/ssh-config"

    echo "=== Generate ssh-config ==="
    kubectl get nodes -o wide --no-headers=true | awk '{print $1}' | while read sshconfig_nodename; do
        echo "Generating ssh-config for Node ${sshconfig_nodename}"
        sshconfig_nodeip="$(kubectl get node "${sshconfig_nodename}" -o jsonpath='{.status.addresses[0].address}')"
        cp "${GIT_CHECKOUT_DIR}/ci/jenkins/ssh-config" "${CLUSTER_SSHCONFIG}.new"
        sed -i "s/SSHCONFIGNODEIP/${sshconfig_nodeip}/g" "${CLUSTER_SSHCONFIG}.new"
        sed -i "s/SSHCONFIGNODENAME/${sshconfig_nodename}/g" "${CLUSTER_SSHCONFIG}.new"
        echo "    IdentityFile ${GIT_CHECKOUT_DIR}/jenkins/key/antrea-ci-key" >> "${CLUSTER_SSHCONFIG}.new"
        cat "${CLUSTER_SSHCONFIG}.new" >> "${CLUSTER_SSHCONFIG}"
    done

    echo "=== Move kubeconfig to control-plane Node ==="
    control_plane_ip="$(kubectl get nodes -o wide --no-headers=true | awk -v role="$CONTROL_PLANE_NODE_ROLE" '$3 ~ role {print $6}')"
    ${SSH_WITH_ANTREA_CI_KEY} -n capv@${control_plane_ip} "if [ ! -d ".kube" ]; then mkdir .kube; fi"
    ${SCP_WITH_ANTREA_CI_KEY} $GIT_CHECKOUT_DIR/jenkins/out/kubeconfig capv@${control_plane_ip}:~/.kube/config

    set +e
    mkdir -p ${GIT_CHECKOUT_DIR}/theia-test-logs
    go version
    go mod tidy -compat=1.17

    set +e
    # HACK: see https://github.com/antrea-io/antrea/issues/2292
    go test -v -timeout=100m antrea.io/theia/test/e2e --logs-export-dir ${GIT_CHECKOUT_DIR}/theia-test-logs --provider remote --remote.sshconfig "${CLUSTER_SSHCONFIG}" --remote.kubeconfig "${CLUSTER_KUBECONFIG}"


    test_rc=$?
    set -e

    if [[ "$test_rc" != "0" ]]; then
        echo "=== TEST FAILURE !!! ==="
        TEST_FAILURE=true
    else
        echo "=== TEST SUCCESS !!! ==="
    fi

    tar -zcf ${GIT_CHECKOUT_DIR}/theia-test-logs.tar.gz ${GIT_CHECKOUT_DIR}/theia-test-logs

}

function cleanup_cluster() {
    echo "=== Cleaning up VMC cluster ${CLUSTER} ==="
    export KUBECONFIG=$KUBECONFIG_PATH

    kubectl delete ns ${CLUSTER}
    rm -rf "${GIT_CHECKOUT_DIR}/jenkins"
    echo "=== Cleanup cluster ${CLUSTER} succeeded ==="

    release_static_ip
}

function garbage_collection() {
    echo "=== Auto cleanup starts ==="
    export KUBECONFIG=$KUBECONFIG_PATH

    kubectl get ns -l antrea-ci -o custom-columns=Name:.metadata.name,DATE:.metadata.creationTimestamp --no-headers=true | awk '{cmd="echo $(( $(date +%s) - $(date -d "$2" +%s) ))"; cmd | getline t ; print $1, t}' | awk '$1 ~ "matrix" && $2 > 36000 {print $1}' | while read cluster; do
        # Matrix tests
        echo "=== Currently ${cluster} has been live for more than 10h ==="
        kubectl delete ns ${cluster}
        echo "=== Old namespace ${cluster} has been deleted !!! ==="
    done

    kubectl get ns -l antrea-ci -o custom-columns=Name:.metadata.name,DATE:.metadata.creationTimestamp --no-headers=true | awk '{cmd="echo $(( $(date +%s) - $(date -d "$2" +%s) ))"; cmd | getline t ; print $1, t}' | awk '$1 ~ "whole-conformance" && $2 > 14400 {print $1}' | while read cluster; do
        # Whole-conformance test
        echo "=== Currently ${cluster} has been live for more than 4h ==="
        kubectl delete ns ${cluster}
        echo "=== Old namespace ${cluster} has been deleted !!! ==="
    done

    kubectl get ns -l antrea-ci -o custom-columns=Name:.metadata.name,DATE:.metadata.creationTimestamp --no-headers=true | awk '{cmd="echo $(( $(date +%s) - $(date -d "$2" +%s) ))"; cmd | getline t ; print $1, t}' | awk '$1 !~ "matrix" && $1 !~ "whole-conformance" && $2 > 9000 {print $1}' | while read cluster; do
        # e2e, conformance, networkpolicy tests
        echo "=== Currently ${cluster} has been live for more than 2.5h ==="
        kubectl delete ns ${cluster}
        echo "=== Old namespace ${cluster} has been deleted !!! ==="
    done

    echo "=== Auto cleanup finished ==="
}

function clean_tmp() {
    echo "===== Clean up stale files & folders older than 7 days under /tmp ====="
    CLEAN_LIST=(
        "*codecov*"
        "kustomize-*"
        "*antrea*"
        "go-build*"
    )
    for item in "${CLEAN_LIST[@]}"; do
        find /tmp -name "${item}" -mtime +7 -exec rm -rf {} \; 2>&1 | grep -v "Permission denied" || true
    done
}

# ensures that the script can be run from anywhere
THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
GIT_CHECKOUT_DIR=${THIS_DIR}/../..
pushd "$THIS_DIR" > /dev/null

SCP_WITH_ANTREA_CI_KEY="scp -q -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${GIT_CHECKOUT_DIR}/jenkins/key/antrea-ci-key"
SSH_WITH_ANTREA_CI_KEY="ssh -q -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${GIT_CHECKOUT_DIR}/jenkins/key/antrea-ci-key"
SSH_WITH_UTILS_KEY="ssh -q -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${WORKDIR}/utils/key"
SCP_WITH_UTILS_KEY="scp -q -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${WORKDIR}/utils/key"

clean_tmp
if [[ "$RUN_GARBAGE_COLLECTION" == true ]]; then
    garbage_collection
    exit 0
fi

if [[ "$RUN_SETUP_ONLY" == true ]]; then
    setup_cluster
    # deliver_antrea
    exit 0
fi

if [[ "$RUN_CLEANUP_ONLY" == true ]]; then
    cleanup_cluster
    exit 0
fi

if [[ "$TESTCASE" != "e2e" ]]; then
    echoerr "testcase should be e2e, integration, multicluster-integration, conformance, whole-conformance or networkpolicy"
    exit 1
fi

if [[ "$RUN_TEST_ONLY" == true ]]; then
    if [[ "$TESTCASE" == "e2e" ]]; then
        run_e2e
    fi
    if [[ "$TEST_FAILURE" == true ]]; then
        exit 1
    fi
    exit 0
fi



trap cleanup_cluster EXIT
if [[ "$TESTCASE" == "e2e" ]]; then
    setup_cluster
    deliver_antrea
    run_e2e

fi

if [[ "$TEST_FAILURE" == true ]]; then
    exit 1
fi
