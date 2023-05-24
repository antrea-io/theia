#!/bin/bash
# Copyright 2023 Antrea Authors.
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


# Check if at least one argument is provided
if [ $# -lt 1 ]; then
    echo "No arguments provided."
    exit 1
fi

# set global variables
namespace="flow-visibility"
pod_name="chi-clickhouse-clickhouse-0-0-0"
backup_name="clickhouse_backup"
container_name="kind-worker"
total_wait_time=30
interval=5

# Set default values for optional arguments
loglevel=0
to_yaml=""
from_yaml=""
delete_backup=false
delete_yaml=false

# Process the arguments
while [[ $# -gt 0 ]]; do
  key="$1"

  case $key in
    --loglevel)
      loglevel="$2"
      shift
      shift
      ;;
    --toyaml)
      to_yaml="$2"
      shift
      shift
      ;;
    --fromyaml)
      from_yaml="$2"
      shift
      shift
      ;;
    --deletebackup)
      delete_backup=true
      shift
      ;;
    --deleteyaml)
      delete_yaml=true
      shift
      ;;
    *)
      echo "Invalid argument: $1"
      exit 1
      ;;
  esac
done

store_yaml_locally(){
    local from_yaml="$1"
    local to_yaml="$2"
    commands=(
        "docker cp kind-control-plane:/root/$from_yaml ."
        "docker cp kind-control-plane:/root/$to_yaml ."
    )
    
    for cmd in "${commands[@]}"; do
        run_bash_command "$cmd"
    done
}

backup_clickhouse_data(){
    commands=(
        "apt -y upgrade"
        "apt install -y wget"
        "wget https://github.com/AlexAkulov/clickhouse-backup/releases/download/v2.2.5/clickhouse-backup-linux-amd64.tar.gz"
        "tar -zxvf clickhouse-backup-linux-amd64.tar.gz"
        "./build/linux/amd64/clickhouse-backup delete local $backup_name"
        "./build/linux/amd64/clickhouse-backup create $backup_name "
    )

    for cmd in "${commands[@]}"; do
        run_command_on_pod "$cmd"
    done
}

save_clickhouse_data(){
    local command="kubectl cp flow-visibility/chi-clickhouse-clickhouse-0-0-0:/var/lib/clickhouse/backup/$backup_name/ $backup_name"
    run_bash_command "$command"
}

delete_flowvisibility_namespace(){
    local from_yaml="$1"

    commands=(
        "kubectl delete clickhouseinstallation.clickhouse.altinity.com clickhouse -n flow-visibility"
        "kubectl delete -f $from_yaml --ignore-not-found=true"
    )

    for cmd in "${commands[@]}"; do
        run_bash_command "$cmd"
    done
}

apply_new_flowvisibility_namespace(){
    local to_yaml="$1"

    local command="kubectl apply -f $to_yaml"

    run_bash_command "$command"
    sleep 5
}

wait_for_clickhouse_pod() {
    local timeout=300
    local interval=5
    local ready_replicas=0

    local end_time=$((SECONDS + timeout))

    echo "Waiting for all replicas of $pod_name to be ready in namespace $namespace..."

    while [ $SECONDS -lt $end_time ]; do
        ready_replicas=$(kubectl get pods "$pod_name" -n "$namespace" -o jsonpath='{.status.containerStatuses[0].ready}')

        if [ -z "$ready_replicas" ]; then
            echo "No replicas found for pod $pod_name in namespace $namespace"
            return
        fi

        if [[ "$ready_replicas" == *false* ]]; then
            echo "Not all replicas of $pod_name are ready. Waiting..."
            sleep "$interval"
        else
            echo "All replicas of $pod_name are ready in namespace $namespace"
            return
        fi
    done

    echo "Timeout reached. Not all replicas of $pod_name are ready in namespace $namespace"
    exit 1
}

recreate_clickhouse_dir(){
    commands=(
		"rm -rf /data/clickhouse"
		"mkdir -p /data/clickhouse"
    )

    for cmd in "${commands[@]}"; do
        run_command_on_docker  "$cmd"
    done
}

restore_backup(){
    commands=(
		"apt -y upgrade"
		"apt install -y wget"
		"wget https://github.com/AlexAkulov/clickhouse-backup/releases/download/v2.2.5/clickhouse-backup-linux-amd64.tar.gz"
		"tar -zxvf clickhouse-backup-linux-amd64.tar.gz"
		"./build/linux/amd64/clickhouse-backup create "
    )

    for cmd in "${commands[@]}"; do
        run_command_on_pod "$cmd"
        sleep 2
    done

    k8s_cp_cmd="kubectl cp $backup_name flow-visibility/chi-clickhouse-clickhouse-0-0-0:/var/lib/clickhouse/backup/$backup_name/ "
    run_bash_command "$k8s_cp_cmd"

    restore_cmd="./build/linux/amd64/clickhouse-backup restore $backup_name"
    run_command_on_pod "$restore_cmd"
}

delete_yamls_locally(){
    local from_yaml=$1
    local to_yaml=$2

    commands=(
		"rm -rf $from_yaml "
		"rm -rf $to_yaml "
    )

    for cmd in "${commands[@]}"; do
        run_bash_command "$cmd"
    done
}

delete_CH_backup(){
    local cmd="rm -rf $backup_name"
    run_bash_command "$cmd"
}

run_command_on_pod(){
    local command="$1"

    # Execute the command and capture the exit code
    local run_command="kubectl exec -n $namespace $pod_name -- bash -c \"$command\""
    
    if [ "$loglevel" -ne 0 ]; then
        echo "$run_command"
    fi
    run_cmd "$run_command"
}

run_bash_command() {
    local command="$1"

    # Execute the command and capture the exit code
    local run_command="bash -c \"$command\""
    
    run_cmd "$run_command"
}

run_command_on_docker() {
    local command="$1"

    # Execute the command on Docker container
    local run_command="docker exec $container_name bash -c \"$command\""

    run_cmd "$run_command"
}

run_cmd() {
    local command="$1"

    # Execute the command and capture the exit code, stdout, and stderr
    output=$(eval "$command" 2>&1)
    exit_code=$?

    # Check if the exit code indicates an error
    if [ $exit_code -ne 0 ] && [[ $command != *"delete"* ]]; then
        echo "Error: Command failed with exit code $exit_code"
        echo "Error Output:"
        echo "$output"
        exit 1
    fi

    # Print stdout and stderr based on log level
    if [ "$loglevel" -ne 0 ]; then
        echo "Stdout:"
        echo "$output"
    fi
}

wait_and_print() {
    echo "Waiting for graceful deletion for $total_wait_time seconds..."

    for ((i = 0; i < total_wait_time; i += interval)); do
        sleep "$interval"
        if [ "$loglevel" -ne 0 ]; then
            echo "."
        fi
    done
}


if [ "$delete_yaml" = true ]; then
    store_yaml_locally "$from_yaml" "$to_yaml"
fi

echo "Backing up Clickhouse data"
backup_clickhouse_data
echo "Saving Clickhouse Data locally"
save_clickhouse_data
echo "Deleting Flowvisibility Namespace"
delete_flowvisibility_namespace "$from_yaml"
wait_and_print
echo "Recreating Clickhouse Directory for new PV if exists"
recreate_clickhouse_dir
echo "Applying new flowvisibility Yaml"
apply_new_flowvisibility_namespace "$to_yaml"
echo "Waiting for Clickhouse Pod to be Ready"
wait_for_clickhouse_pod
echo "Restoring Data back to Clickhouse"
restore_backup

if [ "$delete_yaml" = true ]; then
    delete_yamls_locally "$from_yaml" "$to_yaml"
fi

if [ "$delete_backup" = true ]; then
	echo "Deleting backup clickhouse_backup $backup_name"
    delete_CH_backup
fi

echo "Backup restored, exiting!"
