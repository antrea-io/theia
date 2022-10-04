CREATE VIEW IF NOT EXISTS policies 
as SELECT
 flowEndSeconds,
 octetDeltaCount,
 reverseOctetDeltaCount,
 egressNetworkPolicyName,
 egressNetworkPolicyNamespace,
 egressNetworkPolicyRuleAction,
 ingressNetworkPolicyName,
 ingressNetworkPolicyNamespace,
 ingressNetworkPolicyRuleAction,
 sourcePodName,
 sourcePodNamespace,
 sourceTransportPort,
 destinationIP,
 destinationPodName,
 destinationPodNamespace,
 destinationTransportPort,
 destinationServicePortName,
 destinationServicePort,
 throughput,
 flowtype,
 clusterUUID
FROM flows
