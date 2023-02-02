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

def get_flow_type(flow_type, destination_service_port_name, destination_pod_labels):
    if flow_type == 3:
        return "pod_to_external"
    elif destination_service_port_name:
        return "pod_to_svc"
    elif destination_pod_labels:
        return "pod_to_pod"
    else:
        return "pod_to_external"

def get_protocol_string(protocol_identifier):
    if protocol_identifier == 6:
        return "TCP"
    elif protocol_identifier == 17:
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
                job_type,
                isolation_method,
                ns_allow_list,
                label_ignore_list,
                source_pod_namespace,
                source_pod_labels,
                destination_ip,
                destination_pod_namespace,
                destination_pod_labels,
                destination_service_port_name,
                destination_transport_port,
                protocol_identifier,
                flow_type):
        labelsToIgnore = []
        if label_ignore_list:
            labelsToIgnore = label_ignore_list.split(',')
        source_pod_labels = parseLabels(source_pod_labels, labelsToIgnore)
        destination_pod_labels = parseLabels(destination_pod_labels, labelsToIgnore)
        flow_type = get_flow_type(flow_type, destination_service_port_name, destination_pod_labels)
        protocol_identifier = get_protocol_string(protocol_identifier)
        
        # Build row for source Pod as applied_to
        applied_to = ROW_DELIMITER.join([source_pod_namespace, source_pod_labels])
        if flow_type == "pod_to_external":
            egress = ROW_DELIMITER.join([destination_ip, str(destination_transport_port), protocol_identifier])
        elif flow_type == "pod_to_svc" and isolation_method != 3:
            # K8s policies don't support Pod to Service rules
            svc_ns, svc_name = destination_service_port_name.partition(':')[0].split('/')
            egress = ROW_DELIMITER.join([svc_ns, svc_name])
        else:
            egress = ROW_DELIMITER.join([destination_pod_namespace, destination_pod_labels, str(destination_transport_port), protocol_identifier])
        row = Result(applied_to, "", egress)
        yield(row.applied_to, row.ingress, row.egress)

        # Build row for destination Pod (if possible) as applied_to
        if flow_type != "pod_to_external":
            applied_to = ROW_DELIMITER.join([destination_pod_namespace, destination_pod_labels])
            ingress = ROW_DELIMITER.join([source_pod_namespace, source_pod_labels, str(destination_transport_port), protocol_identifier])
            row = Result(applied_to, ingress, "")
            yield(row.applied_to, row.ingress, row.egress)
