import json

ROW_DELIMITER = "#"

def parseLabels(labels, omitKeys = []):
    if not labels:
        return "{}"
    # Just for PoC, generated records having labels in single-quote
    labels = labels.replace("\'", "\"")
    labels_dict = json.loads(labels)
    labels_dict = {
        key: value
        for key, value in labels_dict.items()
        if key not in omitKeys
    }
    return json.dumps(labels_dict, sort_keys=True)

def get_flow_type(flowType, destinationServicePortName, destinationPodLabels):
    if flowType == 3:
        return "pod_to_external"
    elif destinationServicePortName:
        return "pod_to_svc"
    elif destinationPodLabels:
        return "pod_to_pod"
    else:
        return "pod_to_external"

def get_protocol_string(protocolIdentifier):
    if protocolIdentifier == 6:
        return "TCP"
    elif protocolIdentifier == 17:
        return "UDP"
    else:
        return "UNKNOWN"
    
class Result:
    def __init__(self, applied_to, ingress, egress):
        self.applied_to = applied_to
        self.ingress = ingress
        self.egress = egress

class PreProcessing:
    def __init__(self):
        return

    def process(self,
                jobType,
                isolationMethod,
                nsAllowList,
                labelIgnoreList,
                sourcePodNamespace,
                sourcePodLabels,
                destinationIP,
                destinationPodNamespace,
                destinationPodLabels,
                destinationServicePortName,
                destinationTransportPort,
                protocolIdentifier,
                flowType):
        labelsToIgnore = []
        if labelIgnoreList:
            labelsToIgnore = labelIgnoreList.split(',')
        sourcePodLabels = parseLabels(sourcePodLabels, labelsToIgnore)
        destinationPodLabels = parseLabels(destinationPodLabels, labelsToIgnore)
        flowType = get_flow_type(flowType, destinationServicePortName, destinationPodLabels)
        protocolIdentifier = get_protocol_string(protocolIdentifier)
        
        # Build row for source Pod as applied_to
        applied_to = ROW_DELIMITER.join([sourcePodNamespace, sourcePodLabels])
        if flowType == "pod_to_external":
            egress = ROW_DELIMITER.join([destinationIP, str(destinationTransportPort), protocolIdentifier])
        elif flowType == "pod_to_svc" and isolationMethod != 3:
            # K8s policies don't support Pod to Service rules
            svc_ns, svc_name = destinationServicePortName.partition(':')[0].split('/')
            egress = ROW_DELIMITER.join([svc_ns, svc_name])
        else:
            egress = ROW_DELIMITER.join([destinationPodNamespace, destinationPodLabels, str(destinationTransportPort), protocolIdentifier])
        row = Result(applied_to, "", egress)
        yield(row.applied_to, row.ingress, row.egress)

        # Build row for destination Pod (if possible) as applied_to
        if flowType != "pod_to_external":
            applied_to = ROW_DELIMITER.join([destinationPodNamespace, destinationPodLabels])
            ingress = ROW_DELIMITER.join([sourcePodNamespace, sourcePodLabels, str(destinationTransportPort), protocolIdentifier])
            row = Result(applied_to, ingress, "")
            yield(row.applied_to, row.ingress, row.egress)
