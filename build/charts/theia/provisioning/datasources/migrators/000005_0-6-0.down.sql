DROP VIEW flows_pod_view_local;
DROP VIEW flows_node_view_local;
DROP VIEW flows_policy_view_local;

--Create a Materialized View to aggregate data for pods
CREATE MATERIALIZED VIEW IF NOT EXISTS flows_pod_view_local
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
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
    clusterUUID)
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

--Create a Materialized View to aggregate data for nodes
CREATE MATERIALIZED VIEW IF NOT EXISTS flows_node_view_local
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
ORDER BY (
    timeInserted,
    flowEndSeconds,
    flowEndSecondsFromSourceNode,
    flowEndSecondsFromDestinationNode,
    sourceNodeName,
    destinationNodeName,
    sourcePodNamespace,
    destinationPodNamespace,
    clusterUUID)
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

--Create a Materialized View to aggregate data for network policies
CREATE MATERIALIZED VIEW IF NOT EXISTS flows_policy_view_local
ENGINE = ReplicatedSummingMergeTree('/clickhouse/tables/{shard}/{database}/{table}', '{replica}')
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
    clusterUUID)
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

INSERT INTO ".inner.flows_pod_view_local" SELECT * FROM pod_view_table_local;
INSERT INTO ".inner.flows_node_view_local" SELECT * FROM node_view_table_local;
INSERT INTO ".inner.flows_policy_view_local" SELECT * FROM policy_view_table_local;

DROP TABLE pod_view_table_local;
DROP TABLE node_view_table_local;
DROP TABLE policy_view_table_local;

--Alter table to drop new columns
ALTER TABLE flows
    DROP COLUMN egressName,
    DROP COLUMN egressIP;
ALTER TABLE flows_local
    DROP COLUMN egressName,
    DROP COLUMN egressIP;
ALTER TABLE tadetector
    DROP COLUMN podNamespace,
    DROP COLUMN podLabels,
    DROP COLUMN destinationServicePortName,
    DROP COLUMN aggType,
    DROP COLUMN direction,
    DROP COLUMN podName;
ALTER TABLE tadetector_local
    DROP COLUMN podNamespace,
    DROP COLUMN podLabels,
    DROP COLUMN destinationServicePortName,
    DROP COLUMN aggType,
    DROP COLUMN direction,
    DROP COLUMN podName;
