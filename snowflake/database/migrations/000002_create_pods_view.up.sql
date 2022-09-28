CREATE VIEW IF NOT EXISTS pods(
  flowStartSeconds,
  flowEndSeconds,
  packetDeltaCount,
  octetDeltaCount,
  reversePacketDeltaCount,
  reverseOctetDeltaCount,
  sourcePodName,
  sourcePodNamespace,
  sourceTransportPort,
  source,
  destinationPodName,
  destinationPodNamespace,
  destinationTransportPort,
  destination,
  throughput,
  reverseThroughput,
  clusterUUID,
  flowType,
) as SELECT
  flowStartSeconds,
  flowEndSeconds,
  packetDeltaCount,
  octetDeltaCount,
  reversePacketDeltaCount,
  reverseOctetDeltaCount,
  sourcePodName,
  sourcePodNamespace,
  sourceTransportPort,
  sourcePodNamespace || '/' || sourcePodName,
  destinationPodName,
  destinationPodNamespace,
  destinationTransportPort,
  destinationPodNamespace || '/' || destinationPodName,
  throughput,
  reverseThroughput,
  flowtype,
  clusterUUID,
 FROM flows
