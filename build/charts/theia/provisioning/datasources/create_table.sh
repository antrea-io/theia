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

# ttlTimeout is calculated by the smaller value in its default value and TTL
{{- $ttl := split " " .Values.clickhouse.ttl }}
{{- $ttlTimeout := 14400 }}
{{- if eq $ttl._1 "SECOND" }}
{{- $ttlTimeout = min $ttl._0 $ttlTimeout }}
{{- else if eq $ttl._1 "MINUTE" }}
{{- $ttlTimeout = min (mul $ttl._0 60) $ttlTimeout }}
{{- else if eq $ttl._1 "HOUR" }}
{{- $ttlTimeout = min (mul $ttl._0 60 60) $ttlTimeout }}
{{- end }}

function createTable {
clickhouse client -n -h 127.0.0.1 <<-EOSQL
    --Create a table to store records
    CREATE TABLE IF NOT EXISTS flows_local (
        timeInserted DateTime DEFAULT now(),
        flowStartSeconds DateTime,
        flowEndSeconds DateTime,
        flowEndSecondsFromSourceNode DateTime,
        flowEndSecondsFromDestinationNode DateTime,
        flowEndReason UInt8,
        sourceIP String,
        destinationIP String,
        sourceTransportPort UInt16,
        destinationTransportPort UInt16,
        protocolIdentifier UInt8,
        packetTotalCount UInt64,
        octetTotalCount UInt64,
        packetDeltaCount UInt64,
        octetDeltaCount UInt64,
        reversePacketTotalCount UInt64,
        reverseOctetTotalCount UInt64,
        reversePacketDeltaCount UInt64,
        reverseOctetDeltaCount UInt64,
        sourcePodName String,
        sourcePodNamespace String,
        sourceNodeName String,
        destinationPodName String,
        destinationPodNamespace String,
        destinationNodeName String,
        destinationClusterIP String,
        destinationServicePort UInt16,
        destinationServicePortName String,
        ingressNetworkPolicyName String,
        ingressNetworkPolicyNamespace String,
        ingressNetworkPolicyRuleName String,
        ingressNetworkPolicyRuleAction UInt8,
        ingressNetworkPolicyType UInt8,
        egressNetworkPolicyName String,
        egressNetworkPolicyNamespace String,
        egressNetworkPolicyRuleName String,
        egressNetworkPolicyRuleAction UInt8,
        egressNetworkPolicyType UInt8,
        tcpState String,
        flowType UInt8,
        sourcePodLabels String,
        destinationPodLabels String,
        throughput UInt64,
        reverseThroughput UInt64,
        throughputFromSourceNode UInt64,
        throughputFromDestinationNode UInt64,
        reverseThroughputFromSourceNode UInt64,
        reverseThroughputFromDestinationNode UInt64,
        clusterUUID String,
        egressName String,
        egressIP String,
        trusted UInt8 DEFAULT 0
    ) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (timeInserted, flowEndSeconds);

    ALTER TABLE flows_local MODIFY TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }};
    ALTER TABLE flows_local MODIFY SETTING merge_with_ttl_timeout={{ $ttlTimeout }};

    --Create a Materialized View to aggregate data for Pods and save the data
    --to default.pod_view_table_local
    CREATE TABLE IF NOT EXISTS pod_view_table_local (
        timeInserted DateTime DEFAULT now(),
        flowEndSeconds DateTime,
        flowEndSecondsFromSourceNode DateTime,
        flowEndSecondsFromDestinationNode DateTime,
        sourcePodName String,
        destinationPodName String,
        destinationIP String,
        destinationServicePort UInt16,
        destinationServicePortName String,
        flowType UInt8,
        sourcePodNamespace String,
        destinationPodNamespace String,
        sourceTransportPort UInt16,
        destinationTransportPort UInt16,
        octetDeltaCount UInt64,
        reverseOctetDeltaCount UInt64,
        throughput UInt64,
        reverseThroughput UInt64,
        throughputFromSourceNode UInt64,
        throughputFromDestinationNode UInt64,
        clusterUUID String
    ) ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourcePodName,
        destinationPodName,
        destinationIP,
        destinationServicePort,
        destinationServicePortName,
        flowType,
        sourcePodNamespace,
        destinationPodNamespace,
        sourceTransportPort,
        destinationTransportPort,
        clusterUUID);

    ALTER TABLE "pod_view_table_local" MODIFY TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }};
    ALTER TABLE "pod_view_table_local" MODIFY SETTING merge_with_ttl_timeout={{ $ttlTimeout }};

    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_pod_view_local TO pod_view_table_local
    AS SELECT
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourcePodName,
        destinationPodName,
        destinationIP,
        destinationServicePort,
        destinationServicePortName,
        flowType,
        sourcePodNamespace,
        destinationPodNamespace,
        sourceTransportPort,
        destinationTransportPort,
        sum(octetDeltaCount) AS octetDeltaCount,
        sum(reverseOctetDeltaCount) AS reverseOctetDeltaCount,
        sum(throughput) AS throughput,
        sum(reverseThroughput) AS reverseThroughput,
        sum(throughputFromSourceNode) AS throughputFromSourceNode,
        sum(throughputFromDestinationNode) AS throughputFromDestinationNode,
        clusterUUID
    FROM flows_local
    GROUP BY
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourcePodName,
        destinationPodName,
        destinationIP,
        destinationServicePort,
        destinationServicePortName,
        flowType,
        sourcePodNamespace,
        destinationPodNamespace,
        sourceTransportPort,
        destinationTransportPort,
        clusterUUID;

    --Create a Materialized View to aggregate data for Nodes and save the data
    --to default.node_view_table_local
    CREATE TABLE IF NOT EXISTS node_view_table_local (
        timeInserted DateTime DEFAULT now(),
        flowEndSeconds DateTime,
        flowEndSecondsFromSourceNode DateTime,
        flowEndSecondsFromDestinationNode DateTime,
        sourceNodeName String,
        destinationNodeName String,
        sourcePodNamespace String,
        destinationPodNamespace String,
        octetDeltaCount UInt64,
        reverseOctetDeltaCount UInt64,
        throughput UInt64,
        reverseThroughput UInt64,
        throughputFromSourceNode UInt64,
        reverseThroughputFromSourceNode UInt64,
        throughputFromDestinationNode UInt64,
        reverseThroughputFromDestinationNode UInt64,
        clusterUUID String
    ) ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourceNodeName,
        destinationNodeName,
        sourcePodNamespace,
        destinationPodNamespace,
        clusterUUID);

    ALTER TABLE "node_view_table_local" MODIFY TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }};
    ALTER TABLE "node_view_table_local" MODIFY SETTING merge_with_ttl_timeout={{ $ttlTimeout }};

    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_node_view_local TO node_view_table_local
    AS SELECT
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourceNodeName,
        destinationNodeName,
        sourcePodNamespace,
        destinationPodNamespace,
        sum(octetDeltaCount) AS octetDeltaCount,
        sum(reverseOctetDeltaCount) AS reverseOctetDeltaCount,
        sum(throughput) AS throughput,
        sum(reverseThroughput) AS reverseThroughput,
        sum(throughputFromSourceNode) AS throughputFromSourceNode,
        sum(reverseThroughputFromSourceNode) AS reverseThroughputFromSourceNode,
        sum(throughputFromDestinationNode) AS throughputFromDestinationNode,
        sum(reverseThroughputFromDestinationNode) AS reverseThroughputFromDestinationNode,
        clusterUUID
    FROM flows_local
    GROUP BY
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourceNodeName,
        destinationNodeName,
        sourcePodNamespace,
        destinationPodNamespace,
        clusterUUID;

    --Create a Materialized View to aggregate data for network policies and
    --save the data to default.policy_view_table_local
    CREATE TABLE IF NOT EXISTS policy_view_table_local (
        timeInserted DateTime DEFAULT now(),
        flowEndSeconds DateTime,
        flowEndSecondsFromSourceNode DateTime,
        flowEndSecondsFromDestinationNode DateTime,
        egressNetworkPolicyName String,
        egressNetworkPolicyNamespace String,
        egressNetworkPolicyRuleAction UInt8,
        ingressNetworkPolicyName String,
        ingressNetworkPolicyNamespace String,
        ingressNetworkPolicyRuleAction UInt8,
        sourcePodName String,
        sourceTransportPort UInt16,
        sourcePodNamespace String,
        destinationPodName String,
        destinationTransportPort UInt16,
        destinationPodNamespace String,
        destinationServicePort UInt16,
        destinationServicePortName String,
        destinationIP String,
        octetDeltaCount UInt64,
        reverseOctetDeltaCount UInt64,
        throughput UInt64,
        reverseThroughput UInt64,
        throughputFromSourceNode UInt64,
        reverseThroughputFromSourceNode UInt64,
        throughputFromDestinationNode UInt64,
        reverseThroughputFromDestinationNode UInt64,
        clusterUUID String
    ) ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        egressNetworkPolicyName,
        egressNetworkPolicyNamespace,
        egressNetworkPolicyRuleAction,
        ingressNetworkPolicyName,
        ingressNetworkPolicyNamespace,
        ingressNetworkPolicyRuleAction,
        sourcePodName,
        sourceTransportPort,
        sourcePodNamespace,
        destinationPodName,
        destinationTransportPort,
        destinationPodNamespace,
        destinationServicePort,
        destinationServicePortName,
        destinationIP,
        clusterUUID);

    ALTER TABLE "policy_view_table_local" MODIFY TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }};
    ALTER TABLE "policy_view_table_local" MODIFY SETTING merge_with_ttl_timeout={{ $ttlTimeout }};

    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_policy_view_local to policy_view_table_local
    AS SELECT
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        egressNetworkPolicyName,
        egressNetworkPolicyNamespace,
        egressNetworkPolicyRuleAction,
        ingressNetworkPolicyName,
        ingressNetworkPolicyNamespace,
        ingressNetworkPolicyRuleAction,
        sourcePodName,
        sourceTransportPort,
        sourcePodNamespace,
        destinationPodName,
        destinationTransportPort,
        destinationPodNamespace,
        destinationServicePort,
        destinationServicePortName,
        destinationIP,
        sum(octetDeltaCount) AS octetDeltaCount,
        sum(reverseOctetDeltaCount) AS reverseOctetDeltaCount,
        sum(throughput) AS throughput,
        sum(reverseThroughput) AS reverseThroughput,
        sum(throughputFromSourceNode) AS throughputFromSourceNode,
        sum(reverseThroughputFromSourceNode) AS reverseThroughputFromSourceNode,
        sum(throughputFromDestinationNode) AS throughputFromDestinationNode,
        sum(reverseThroughputFromDestinationNode) AS reverseThroughputFromDestinationNode,
        clusterUUID
    FROM flows_local
    GROUP BY
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        egressNetworkPolicyName,
        egressNetworkPolicyNamespace,
        egressNetworkPolicyRuleAction,
        ingressNetworkPolicyName,
        ingressNetworkPolicyNamespace,
        ingressNetworkPolicyRuleAction,
        sourcePodName,
        sourceTransportPort,
        sourcePodNamespace,
        destinationPodName,
        destinationTransportPort,
        destinationPodNamespace,
        destinationServicePort,
        destinationServicePortName,
        destinationIP,
        clusterUUID;

    --Create a table to store the network policy recommendation results
    CREATE TABLE IF NOT EXISTS recommendations_local (
        id String,
        type String,
        timeCreated DateTime,
        policy String,
        kind String
    ) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (timeCreated);

    --Create a table to store the Throughput Anomaly Detector results
    CREATE TABLE IF NOT EXISTS tadetector_local (
        sourceIP String,
        sourceTransportPort UInt16,
        destinationIP String,
        destinationTransportPort UInt16,
        protocolIdentifier UInt16,
        flowStartSeconds DateTime,
        flowEndSeconds DateTime,
        throughputStandardDeviation Float64,
        algoType String,
        algoCalc Float64,
        throughput Float64,
        anomaly String,
        id String
    ) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    ORDER BY (flowStartSeconds);

    --Create distributed tables for cluster
    CREATE TABLE IF NOT EXISTS flows AS flows_local
    engine=Distributed('{cluster}', default, flows_local, rand());

    CREATE TABLE IF NOT EXISTS flows_pod_view AS flows_pod_view_local
    engine=Distributed('{cluster}', default, flows_pod_view_local, rand());

    CREATE TABLE IF NOT EXISTS flows_node_view AS flows_node_view_local
    engine=Distributed('{cluster}', default, flows_node_view_local, rand());

    CREATE TABLE IF NOT EXISTS flows_policy_view AS flows_policy_view_local
    engine=Distributed('{cluster}', default, flows_policy_view_local, rand());

    CREATE TABLE IF NOT EXISTS recommendations AS recommendations_local
    engine=Distributed('{cluster}', default, recommendations_local, rand());

    CREATE TABLE IF NOT EXISTS tadetector AS tadetector_local
    engine=Distributed('{cluster}', default, tadetector_local, rand());

EOSQL
}
