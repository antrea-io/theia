CREATE TABLE flows (
  flowStartSeconds TIMESTAMP_TZ,
  flowEndSeconds TIMESTAMP_TZ,
  flowEndSecondsFromSourceNode TIMESTAMP_TZ,
  flowEndSecondsFromDestinationNode TIMESTAMP_TZ,
  flowEndReason NUMBER(3, 0),
  sourceIP STRING(50),
  destinationIP STRING(50),
  sourceTransportPort NUMBER(5, 0),
  destinationTransportPort NUMBER(5, 0),
  protocolIdentifier NUMBER(3, 0),
  packetTotalCount NUMBER(20, 0),
  octetTotalCount NUMBER(20, 0),
  packetDeltaCount NUMBER(20, 0),
  octetDeltaCount NUMBER(20, 0),
  reversePacketTotalCount NUMBER(20, 0),
  reverseOctetTotalCount NUMBER(20, 0),
  reversePacketDeltaCount NUMBER(20, 0),
  reverseOctetDeltaCount NUMBER(20, 0),
  sourcePodName STRING(256),
  sourcePodNamespace STRING(256),
  sourceNodeName STRING(256),
  destinationPodName STRING(256),
  destinationPodNamespace STRING(256),
  destinationNodeName STRING(256),
  destinationClusterIP STRING(50),
  destinationServicePort NUMBER(5, 0),
  destinationServicePortName STRING(256),
  ingressNetworkPolicyName STRING(256),
  ingressNetworkPolicyNamespace STRING(256),
  ingressNetworkPolicyRuleName STRING(256),
  ingressNetworkPolicyRuleAction NUMBER(3, 0),
  ingressNetworkPolicyType NUMBER(3, 0),
  egressNetworkPolicyName STRING(256),
  egressNetworkPolicyNamespace STRING(256),
  egressNetworkPolicyRuleName STRING(256),
  egressNetworkPolicyRuleAction NUMBER(3, 0),
  egressNetworkPolicyType NUMBER(3, 0),
  tcpState STRING(20),
  flowType NUMBER(3, 0),
  sourcePodLabels STRING(10000),
  destinationPodLabels STRING(10000),
  throughput NUMBER(20, 0),
  reverseThroughput NUMBER(20, 0),
  throughputFromSourceNode NUMBER(20, 0),
  throughputFromDestinationNode NUMBER(20, 0),
  reverseThroughputFromSourceNode NUMBER(20, 0),
  reverseThroughputFromDestinationNode NUMBER(20, 0),
  clusterUUID STRING(36),
  timeInserted TIMESTAMP_TZ DEFAULT current_timestamp(),
  egressName STRING(256),
  egressIP STRING(50)
) IF NOT EXISTS
