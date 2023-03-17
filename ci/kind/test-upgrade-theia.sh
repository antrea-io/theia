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

set -eo pipefail

function echoerr {
    >&2 echo "$@"
}

THEIA_FROM_TAG=
FROM_VERSION_N_MINUS=

_usage="Usage: $0 [--from-tag <TAG>] [--from-version-n-minus <COUNT>]
Perform some basic tests to make sure that Theia can be upgraded from the provided version to the
current checked-out version. One of [--from-tag <TAG>] or [--from-version-n-minus <COUNT>] must be
provided.
        --from-tag <TAG>                Upgrade from this version of Theia (pulled from upstream
                                        Antrea) to the current version.
        --from-version-n-minus <COUNT>  Get all the released versions of Theia and run the upgrade
                                        test from the latest bug fix release for *minor* version
                                        N-{COUNT}. N-1 designates the latest minor release. If this
                                        script is run from a release branch, it will only consider
                                        releases which predate that release branch.
        --help, -h                      Print this message and exit
"

function print_usage {
    echoerr "$_usage"
}

function print_help {
    echoerr "Try '$0 --help' for more information."
}

THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR=$THIS_DIR/../..

while [[ $# -gt 0 ]]
do
key="$1"

case $key in
    --from-tag)
    THEIA_FROM_TAG="$2"
    shift 2
    ;;
    --from-version-n-minus)
    FROM_VERSION_N_MINUS="$2"
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

if [ -z "$THEIA_FROM_TAG" ] && [ -z "$FROM_VERSION_N_MINUS" ]; then
    echoerr "One of --from-tag or --from-version-n-minus must be provided"
    print_help
    exit 1
fi

case $FROM_VERSION_N_MINUS in
    ''|*[!0-9]*)
    echoerr "--from-version-n-minus must be a number greater than 0"
    print_help
    exit 1
    ;;
    *)
    ;;
esac

if [ ! "$FROM_VERSION_N_MINUS" -gt "0" ]; then
    echoerr "--from-version-n-minus must be a number greater than 0"
    print_help
    exit 1
fi

function version_lt() { test "$(printf '%s\n' "$@" | sort -rV | head -n 1)" != "$1"; }

# We want to ignore all minor versions greater than the current version, as an upgrade test implies
# that we are upgrading from an *older* version. This is useful when running this script from a
# release branch (e.g. when testing patch release candidates).
CURRENT_VERSION=$(head -n1 $ROOT_DIR/VERSION)
CURRENT_VERSION=${CURRENT_VERSION:1} # strip leading 'v'
CURRENT_VERSION=${CURRENT_VERSION%-*} # strip "-dev" suffix if present

# Exclude peeled tags and release candidates from the version list.
VERSIONS=$(git ls-remote --tags --ref https://github.com/antrea-io/theia.git | \
               grep -v rc | \
               awk '{print $2}' | awk -F/ '{print $3}' | \
               sort --version-sort -r)

if [ ! -z "$THEIA_FROM_TAG" ]; then
    rc=0
    echo "$VERSIONS" | grep -q "$THEIA_FROM_TAG" || rc=$?
    if [ $rc -ne 0 ]; then
        echoerr "$THEIA_FROM_TAG is not a valid Theia tag"
        exit 1
    fi
else # Set THEIA_FROM_TAG using the provided FROM_VERSION_N_MINUS value
    arr=( ${CURRENT_VERSION//./ } ) # x.y.z -> (x y z)
    minor_version="${arr[0]}.${arr[1]}"
    count=
    for version in $VERSIONS; do
        version_nums=${version:1} # strip leading 'v'
        arr=( ${version_nums//./ } ) # x.y.z -> (x y z)
        new_minor_version="${arr[0]}.${arr[1]}"
        if version_lt $new_minor_version $minor_version; then # change in minor version, increase $count
            ((count+=1))
            minor_version=$new_minor_version
            if [ "$count" == "$FROM_VERSION_N_MINUS" ]; then # we went back enough, use this version
                THEIA_FROM_TAG="${version%-*}" # strip "-alpha" suffix if present
                break
            fi
        fi
    done

    if [ -z "$THEIA_FROM_TAG" ]; then
        echoerr "Cannot determine tag for provided --from-version-n-minus value"
        exit 1
    fi
fi

echo "Running upgrade test for tag $THEIA_FROM_TAG"
ANTREA_FROM_TAG=$(grep "$THEIA_FROM_TAG"  < $ROOT_DIR/VERSION_MAP | awk '{print $2}')

# From v0.3.0, ClickHouse Image is labeled with Theia version
if [[ $THEIA_FROM_TAG == "v0.2.0" || $THEIA_FROM_TAG == "v0.1.0" ]]; then
    CLICKHOUSE_FROM_TAG="21.11"
else
    CLICKHOUSE_FROM_TAG=$THEIA_FROM_TAG
fi

DOCKER_IMAGES=("registry.k8s.io/e2e-test-images/agnhost:2.29" \
                "projects.registry.vmware.com/antrea/busybox"  \
                "projects.registry.vmware.com/antrea/nginx:1.21.6-alpine" \
                "projects.registry.vmware.com/antrea/perftool" \
                "projects.registry.vmware.com/antrea/theia-clickhouse-operator:0.18.2" \
                "projects.registry.vmware.com/antrea/theia-metrics-exporter:0.18.2" \
                "projects.registry.vmware.com/antrea/theia-zookeeper:3.8.0" \
                "projects.registry.vmware.com/antrea/theia-grafana:8.3.3" \
                "projects.registry.vmware.com/antrea/antrea-ubuntu:$ANTREA_FROM_TAG" \
                "projects.registry.vmware.com/antrea/theia-clickhouse-monitor:$THEIA_FROM_TAG" \
                "projects.registry.vmware.com/antrea/theia-clickhouse-server:$CLICKHOUSE_FROM_TAG" \
                "antrea/antrea-ubuntu:latest")

for img in "${DOCKER_IMAGES[@]}"; do
    echo "Pulling $img"
    for i in `seq 3`; do
        docker pull $img > /dev/null && break
        sleep 1
    done
done

DOCKER_IMAGES+=("projects.registry.vmware.com/antrea/theia-clickhouse-monitor:latest\
                 projects.registry.vmware.com/antrea/theia-clickhouse-server:latest")

echo "Creating Kind cluster"
IMAGES="${DOCKER_IMAGES[@]}"
$THIS_DIR/kind-setup.sh create kind --images "$IMAGES"

# When running this script as part of a Github Action, we do *not* want to use
# the pre-installed version of kustomize, as it is a snap and cannot access
# /tmp. See:
#  * https://github.com/actions/virtual-environments/issues/1514
#  * https://forum.snapcraft.io/t/interfaces-allow-access-tmp-directory/5129
unset KUSTOMIZE

# Load latest Antrea yaml files
TMP_DIR=$(mktemp -d $(dirname $0)/tmp.XXXXXXXX)
curl -o $TMP_DIR/antrea.yml https://raw.githubusercontent.com/antrea-io/antrea/main/build/yamls/antrea.yml

sed -i -e "s|image: \"projects.registry.vmware.com/antrea/antrea-ubuntu:latest\"|image: \"antrea/antrea-ubuntu:latest\"|g" $TMP_DIR/antrea.yml
sed -i -e "s|type: RollingUpdate|type: OnDelete|g" $TMP_DIR/antrea.yml
# Load latest Theia yaml files
docker exec -i kind-control-plane dd of=/root/antrea-new.yml < $TMP_DIR/antrea.yml
$ROOT_DIR/hack/generate-manifest.sh --local /data/clickhouse --no-grafana | docker exec -i kind-control-plane dd of=/root/flow-visibility-new.yml
# Always use the latest ClickHouse operator through the test
docker exec -i kind-control-plane dd of=/root/clickhouse-operator-install-bundle.yaml < $ROOT_DIR/build/charts/theia/crds/clickhouse-operator-install-bundle.yaml
rm -rf $TMP_DIR

# Load previous version yaml files from Antrea repo
TMP_DIR=$(mktemp -d $(dirname $0)/tmp.XXXXXXXX)
git clone --branch $ANTREA_FROM_TAG --depth 1 https://github.com/antrea-io/antrea.git $TMP_DIR

pushd $TMP_DIR > /dev/null
export IMG_NAME=projects.registry.vmware.com/antrea/antrea-ubuntu
export IMG_TAG=$ANTREA_FROM_TAG
./hack/generate-manifest.sh --mode release | docker exec -i kind-control-plane dd of=/root/antrea.yml
popd
rm -rf $TMP_DIR

# Load previous version yaml files from Theia repo
TMP_THEIA_DIR=$(mktemp -d $(dirname $0)/tmp.XXXXXXXX)
git clone --branch $THEIA_FROM_TAG --depth 1 https://github.com/antrea-io/theia.git $TMP_THEIA_DIR

pushd $TMP_THEIA_DIR > /dev/null
export IMG_NAME=projects.registry.vmware.com/antrea/theia-clickhouse-monitor
export IMG_TAG=$THEIA_FROM_TAG
# In Theia v0.1.0, we do not support --local option when generating manifest,
# Copy the latest script for release v0.1.0 to generate manifest.
if [[ $THEIA_FROM_TAG == "v0.1.0" ]]; then
    cp $ROOT_DIR/hack/generate-manifest.sh hack/generate-manifest.sh
fi
./hack/generate-manifest.sh --mode release --local /data/clickhouse --no-grafana | docker exec -i kind-control-plane dd of=/root/flow-visibility-ch-only.yml

popd
rm -rf $TMP_THEIA_DIR

rc=0
go test -v -run=TestUpgrade antrea.io/theia/test/e2e -provider=kind --logs-export-dir=$ANTREA_LOG_DIR --upgrade.toVersion=$CURRENT_VERSION --upgrade.fromVersion=$THEIA_FROM_TAG || rc=$?

$THIS_DIR/kind-setup.sh destroy kind

exit $rc
