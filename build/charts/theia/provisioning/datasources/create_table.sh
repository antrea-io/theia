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
set -e
clickhouse client -n -h 127.0.0.1 <<-EOSQL
    --Create a table to store records
    {{- if .Values.clickhouse.cluster.enable }}
    CREATE TABLE IF NOT EXISTS flows_local (
    {{- else }}
    CREATE TABLE IF NOT EXISTS flows (
    {{- end }}
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
        trusted UInt8 DEFAULT 0
    {{- if .Values.clickhouse.cluster.enable }}
    ) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    {{- else }}
    ) engine=MergeTree
    {{- end }}
    ORDER BY (timeInserted, flowEndSeconds)
    TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }}
    SETTINGS merge_with_ttl_timeout = {{ $ttlTimeout }};

    --Create a Materialized View to aggregate data for pods
    {{- if and .Values.clickhouse.cluster.enable }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_pod_view_local
    ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    {{- else }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_pod_view
    ENGINE = SummingMergeTree
    {{- end }}
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
        destinationTransportPort)
    TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }}
    SETTINGS merge_with_ttl_timeout = {{ $ttlTimeout }}
    POPULATE
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
        sum(throughputFromDestinationNode) AS throughputFromDestinationNode
    {{- if .Values.clickhouse.cluster.enable }}
    FROM flows_local
    {{- else }}
    FROM flows
    {{- end }}
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
        destinationTransportPort;

    --Create a Materialized View to aggregate data for nodes
    {{- if .Values.clickhouse.cluster.enable }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_node_view_local
    ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    {{- else }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_node_view
    ENGINE = SummingMergeTree
    {{- end }}
    ORDER BY (
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourceNodeName,
        destinationNodeName,
        sourcePodNamespace,
        destinationPodNamespace)
    TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }}
    SETTINGS merge_with_ttl_timeout = {{ $ttlTimeout }}
    POPULATE
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
        sum(reverseThroughputFromDestinationNode) AS reverseThroughputFromDestinationNode
    {{- if .Values.clickhouse.cluster.enable }}
    FROM flows_local
    {{- else }}
    FROM flows
    {{- end }}
    GROUP BY
        timeInserted,
        flowEndSeconds,
        flowEndSecondsFromSourceNode,
        flowEndSecondsFromDestinationNode,
        sourceNodeName,
        destinationNodeName,
        sourcePodNamespace,
        destinationPodNamespace;

    --Create a Materialized View to aggregate data for network policies
    {{- if .Values.clickhouse.cluster.enable }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_policy_view_local
    ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    {{- else }}
    CREATE MATERIALIZED VIEW IF NOT EXISTS flows_policy_view
    ENGINE = SummingMergeTree
    {{- end }}
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
        destinationIP)
    TTL timeInserted + INTERVAL {{ .Values.clickhouse.ttl }}
    SETTINGS merge_with_ttl_timeout = {{ $ttlTimeout }}
    POPULATE
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
        sum(reverseThroughputFromDestinationNode) AS reverseThroughputFromDestinationNode
    {{- if .Values.clickhouse.cluster.enable }}
    FROM flows_local
    {{- else }}
    FROM flows
    {{- end }}
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
        destinationIP;

    --Create a table to store the network policy recommendation results
    {{- if .Values.clickhouse.cluster.enable }}
    CREATE TABLE IF NOT EXISTS recommendations_local (
    {{- else }}
    CREATE TABLE IF NOT EXISTS recommendations (
    {{- end }}
        id String,
        type String,
        timeCreated DateTime,
        yamls String
    {{- if .Values.clickhouse.cluster.enable }}
    ) engine=ReplicatedMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
    {{- else }}
    ) engine=MergeTree
    {{- end }}
    ORDER BY (timeCreated);

    --Create distributed tables for cluster
    {{- if .Values.clickhouse.cluster.enable }}
    CREATE TABLE IF NOT EXISTS flows AS flows_local
    engine=Distributed('{cluster}', default, flows_local, rand());

    CREATE TABLE IF NOT EXISTS flows_pod_view AS flows_pod_view_local
    engine=Distributed('{cluster}', default, flows_pod_view_local, rand());

    CREATE TABLE IF NOT EXISTS flows_node_view AS flows_node_view_local
    engine=Distributed('{cluster}', default, flows_node_view_local, rand());

    CREATE TABLE IF NOT EXISTS flows_policy_view AS flows_policy_view_local
    engine=Distributed('{cluster}', default, flows_pod_view_local, rand());

    CREATE TABLE IF NOT EXISTS recommendations AS recommendations_local
    engine=Distributed('{cluster}', default, recommendations_local, rand());
    {{- end }}
EOSQL
