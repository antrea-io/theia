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

function checkDataVersion {
    tables=$(clickhouse client -h 127.0.0.1 -q "SHOW TABLES")
    if [[ $tables == *"migrate_version"* ]]; then
        dataVersion=$(clickhouse client -h 127.0.0.1 -q "SELECT version FROM migrate_version")
    elif [[ $tables == *"flows"* ]]; then
        dataVersion="0.1.0"
    fi
}

function setDataVersion {
    tables=$(clickhouse client -h 127.0.0.1 -q "SHOW TABLES")
    if [[ $tables == *"migrate_version"* ]]; then
        clickhouse client -h 127.0.0.1 -q "ALTER TABLE migrate_version DELETE WHERE version!=''"
    else
        clickhouse client -h 127.0.0.1 -q "CREATE TABLE migrate_version (version String) engine=MergeTree ORDER BY version"
    fi
    clickhouse client -h 127.0.0.1 -q "INSERT INTO migrate_version (*) VALUES ('{{ .Chart.Version }}')"
    echo "=== Set data schema version to {{ .Chart.Version }} ==="
}

function addVersionsToList {
    if [[ "$versionListStr" != *"$1"* ]]; then
        versionListStr+="$1,"
    fi
}

function getMigrationPath {
    # Get versions based on the SQL file names
    versionListStr=""
    for fileName in $(ls "$migratorDir/upgrade")
    do
        # fileName upgrading from v0.1.0 to v0.2.0: 0-1-0_0-2-0.sql
        fileName=$(basename $fileName .sql)
        versionPair=(${fileName//_/ })
        addVersionsToList ${versionPair[0]//-/.}
        addVersionsToList ${versionPair[1]//-/.}
    done
    addVersionsToList $dataVersion
    addVersionsToList $theiaVersion
    # Sort the versions to generate the migration path
    versionList=(${versionListStr//,/ })
    old_IFS=$IFS
    IFS=$'\n' sortedversionList=($(sort -V <<<"${versionList[*]}"))
    IFS=$old_IFS
    # Define the upgrading/downgrading path
    index=0
    prev=""
    for version in "${sortedversionList[@]}"
    do
        migratePathStr+="$index:$version, "
        index=$((index+1))
        if [[ -z "$prev" ]]; then
            prev=${version//./-}
        else
            curr=${version//./-}
            UPGRADING_MIGRATORS+=("${prev}_${curr}.sql")
            DOWNGRADING_MIGRATORS+=("${curr}_${prev}.sql")
            prev=$curr
        fi
    done
}

function version_lt() { test "$(printf '%s\n' "$@" | sort -rV | head -n 1)" != "$1"; }

function migrate {
    THIS_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
    migratorDir="/var/lib/clickhouse/migrators"
    mkdir -p $migratorDir

    checkDataVersion
    if [[ -z "$dataVersion" ]]; then
        cp -r $THIS_DIR/migrators/* $migratorDir
        echo "=== No existing data schema. Migration skipped. ==="
        return 0
    fi
    if [[ -z "$THEIA_VERSION" ]]; then
        echo "=== Unable to load the environment variable THEIA_VERSION. Migration failed. ==="
        exit 1
    fi
    theiaVersion=$THEIA_VERSION

    migratePathStr=""
    getMigrationPath

    cp -r $THIS_DIR/migrators/* $migratorDir

    dataVersionIndex=$(echo $migratePathStr | tr ', ' '\n' | grep "$dataVersion" | sed 's/:/ /g' | awk '{print $1}')
    theiaVersionIndex=$(echo $migratePathStr | tr ', ' '\n' | grep "$theiaVersion" | sed 's/:/ /g' | awk '{print $1}')

    # Update along the path
    if [[ "$dataVersionIndex" -lt "$theiaVersionIndex" ]]; then
        for i in $(seq $dataVersionIndex $((theiaVersionIndex-1)) );
        do
            if test -f "$migratorDir/upgrade/${UPGRADING_MIGRATORS[$i]}"; then
                echo "=== Apply file ${UPGRADING_MIGRATORS[$i]} ==="
                clickhouse client -h 127.0.0.1 --queries-file $migratorDir/upgrade/${UPGRADING_MIGRATORS[$i]}
            fi
        done
    # Downgrade along the path
    elif [[ "$dataVersionIndex" -gt "$theiaVersionIndex" ]]; then
        for i in $(seq $((dataVersionIndex-1)) -1 $theiaVersionIndex);
        do
            if test -f "$migratorDir/downgrade/${DOWNGRADING_MIGRATORS[$i]}"; then
                echo "=== Apply file ${DOWNGRADING_MIGRATORS[$i]} ==="
                clickhouse client -h 127.0.0.1 --queries-file $migratorDir/downgrade/${DOWNGRADING_MIGRATORS[$i]}
            fi
        done
    else
        echo "=== Data schema version is the same as Theia version. Migration finished. ==="
    fi
    setDataVersion
}
