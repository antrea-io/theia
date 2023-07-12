-- Create underlying tables for Materialized Views to attach data
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

--Move data from old mv underlying tables and drop old mvs
INSERT INTO pod_view_table_local SELECT * FROM ".inner.flows_pod_view_local";
INSERT INTO node_view_table_local SELECT * FROM ".inner.flows_node_view_local";
INSERT INTO policy_view_table_local SELECT * FROM ".inner.flows_policy_view_local";

DROP VIEW flows_pod_view_local;
DROP VIEW flows_node_view_local;
DROP VIEW flows_policy_view_local;

--Alter table to add new columns
ALTER TABLE flows
    ADD COLUMN egressName String,
    ADD COLUMN egressIP String;
ALTER TABLE flows_local
    ADD COLUMN egressName String,
    ADD COLUMN egressIP String;
ALTER TABLE tadetector
    ADD COLUMN podNamespace String,
    ADD COLUMN podLabels String,
    ADD COLUMN destinationServicePortName String,
    ADD COLUMN aggType String,
    ADD COLUMN direction String,
    ADD COLUMN podName String;
ALTER TABLE tadetector_local
    ADD COLUMN podNamespace String,
    ADD COLUMN podLabels String,
    ADD COLUMN destinationServicePortName String,
    ADD COLUMN aggType String,
    ADD COLUMN direction String,
    ADD COLUMN podName String;
