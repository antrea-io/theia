import datetime
import json
import random
import string
import uuid
import sys

import kubernetes.client

import policy_recommendation.antrea_crd as antrea_crd
from policy_recommendation.policy_recommendation_utils import *
from policy_recommendation.preprocessing_udf import ROW_DELIMITER

DEFAULT_POLICY_PRIORITY = 5

def generate_policy_name(info):
    return "-".join([info, "".join(random.sample(string.ascii_lowercase + string.digits, 5))])

def generate_k8s_egress_rule(egress):
    if len(egress.split(ROW_DELIMITER)) == 4:
        ns, labels, port, protocol_identifier = egress.split(ROW_DELIMITER)
        egress_peer = kubernetes.client.V1NetworkPolicyPeer(
            namespace_selector = kubernetes.client.V1LabelSelector(
                match_labels = {
                    "name":ns
                }
            ),
            pod_selector = kubernetes.client.V1LabelSelector(
                match_labels = json.loads(labels)
            ),
        )
    elif len(egress.split(ROW_DELIMITER)) == 3:
        destination_ip, port, protocol_identifier = egress.split(ROW_DELIMITER)
        if get_IP_version(destination_ip) == "v4":
            cidr = destination_ip + "/32"
        else:
            cidr = destination_ip + "/128"
        egress_peer = kubernetes.client.V1NetworkPolicyPeer(
            ip_block = kubernetes.client.V1IPBlock(
                cidr = cidr,
            )
        )
    else:
        sys.exit(1)
    ports = kubernetes.client.V1NetworkPolicyPort(
        port = int(port),
        protocol = protocol_identifier
    )
    egress_rule = kubernetes.client.V1NetworkPolicyEgressRule(
        to = [egress_peer],
        ports = [ports]
    )
    return egress_rule

def generate_k8s_ingress_rule(ingress):
    if len(ingress.split(ROW_DELIMITER)) != 4:
        sys.exit(1)
    ns, labels, port, protocol_identifier = ingress.split(ROW_DELIMITER)
    ingress_peer = kubernetes.client.V1NetworkPolicyPeer(
        namespace_selector = kubernetes.client.V1LabelSelector(
            match_labels = {
                "name":ns
            }
        ),
        pod_selector = kubernetes.client.V1LabelSelector(
            match_labels = json.loads(labels)
        ),
    )
    ports = kubernetes.client.V1NetworkPolicyPort(
        port = int(port),
        protocol = protocol_identifier
    )
    ingress_rule = kubernetes.client.V1NetworkPolicyIngressRule(
        _from = [ingress_peer],
        ports = [ports]
    )
    return ingress_rule

def generate_k8s_np(applied_to, ingresses, egresses, ns_allow_list):
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in ns_allow_list:
        return ""
    ingress_list = sorted(list(ingresses))
    egress_list = sorted(list(egresses))
    egressRules = []
    for egress in egress_list:
        if ROW_DELIMITER in egress:
            egressRules.append(generate_k8s_egress_rule(egress))
    ingressRules = []
    for ingress in ingress_list:
        if ROW_DELIMITER in ingress:
            ingressRules.append(generate_k8s_ingress_rule(ingress))
    if egressRules or ingressRules:
        policy_types = []
        if egressRules:
            policy_types.append("Egress")
        if ingressRules:
            policy_types.append("Ingress")
        np_name = generate_policy_name("recommend-k8s-np")
        np = kubernetes.client.V1NetworkPolicy(
            api_version = "networking.k8s.io/v1",
            kind = "NetworkPolicy",
            metadata = kubernetes.client.V1ObjectMeta(
                name = np_name,
                namespace = ns
            ),
            spec = kubernetes.client.V1NetworkPolicySpec(
                egress = egressRules,
                ingress = ingressRules,
                pod_selector = kubernetes.client.V1LabelSelector(
                    match_labels = json.loads(labels)
                ),
                policy_types = policy_types
            )
        )
        return dict_to_yaml(np.to_dict())
    else:
        return ""

def generate_anp_egress_rule(egress):
    if len(egress.split(ROW_DELIMITER)) == 4:
        # Pod-to-Pod flow
        ns, labels, port, protocol_identifier = egress.split(ROW_DELIMITER)
        egress_peer = antrea_crd.NetworkPolicyPeer(
            namespace_selector = kubernetes.client.V1LabelSelector(
                match_labels = {
                    "kubernetes.io/metadata.name":ns
                }
            ),
            pod_selector = kubernetes.client.V1LabelSelector(
                match_labels = json.loads(labels)
            ),
        )
        ports = antrea_crd.NetworkPolicyPort(
            protocol = protocol_identifier,
            port = int(port)
        )
        egress_rule = antrea_crd.Rule(
            action = "Allow",
            to = [egress_peer],
            ports = [ports]
        )
    elif len(egress.split(ROW_DELIMITER)) == 3:
        # Pod-to-External flow
        destination_ip, port, protocol_identifier = egress.split(ROW_DELIMITER)
        if get_IP_version(destination_ip) == "v4":
            cidr = destination_ip + "/32"
        else:
            cidr = destination_ip + "/128"
        egress_peer = antrea_crd.NetworkPolicyPeer(
            ip_block = antrea_crd.IPBlock(
                CIDR = cidr,
            )
        )
        ports = antrea_crd.NetworkPolicyPort(
            protocol = protocol_identifier,
            port = int(port)
        )
        egress_rule = antrea_crd.Rule(
            action = "Allow",
            to = [egress_peer],
            ports = [ports]
        )
    elif len(egress.split(ROW_DELIMITER)) == 2:
        # Pod-to-Svc flow
        svc_ns, svc_name = egress.split(ROW_DELIMITER)
        egress_rule = antrea_crd.Rule(
            action = "Allow",
            to_services = [
                antrea_crd.NamespacedName(
                    namespace = svc_ns,
                    name = svc_name
                )
            ]
        )
    else:
        sys.exit(1)
    return egress_rule

def generate_anp_ingress_rule(ingress):
    if len(ingress.split(ROW_DELIMITER)) != 4:
        sys.exit(1)
    ns, labels, port, protocol_identifier = ingress.split(ROW_DELIMITER)
    ingress_peer = antrea_crd.NetworkPolicyPeer(
        namespace_selector = kubernetes.client.V1LabelSelector(
            match_labels = {
                "kubernetes.io/metadata.name":ns
            }
        ),
        pod_selector = kubernetes.client.V1LabelSelector(
            match_labels = json.loads(labels)
        ),
    )
    ports = antrea_crd.NetworkPolicyPort(
        protocol = protocol_identifier,
        port = int(port)
    )
    ingress_rule = antrea_crd.Rule(
        action = "Allow",
        _from = [ingress_peer],
        ports = [ports]
    )
    return ingress_rule

def generate_anp(applied_to, ingresses, egresses, ns_allow_list):
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in ns_allow_list:
        return ""
    ingress_list = sorted(list(ingresses))
    egress_list = sorted(list(egresses))
    egressRules = []
    for egress in egress_list:
        if ROW_DELIMITER in egress:
            egress_rule = generate_anp_egress_rule(egress)
            if egress_rule:
                egressRules.append(egress_rule)
    ingressRules = []
    for ingress in ingress_list:
        if ROW_DELIMITER in ingress:
            ingress_rule = generate_anp_ingress_rule(ingress)
            if ingress_rule:
                ingressRules.append(ingress_rule)
    if egressRules or ingressRules:
        np_name = generate_policy_name("recommend-allow-anp")
        np = antrea_crd.NetworkPolicy(
            api_version = "crd.antrea.io/v1alpha1",
            kind = "NetworkPolicy",
            metadata = kubernetes.client.V1ObjectMeta(
                name = np_name,
                namespace = ns,
            ),
            spec = antrea_crd.NetworkPolicySpec(
                tier = "Application",
                priority = DEFAULT_POLICY_PRIORITY,
                applied_to = [antrea_crd.NetworkPolicyPeer(
                    pod_selector = kubernetes.client.V1LabelSelector(
                        match_labels = json.loads(labels)
                    ),
                )],
                egress = egressRules,
                ingress = ingressRules,
            )
        )
        return dict_to_yaml(np.to_dict())
    else:
        return ""

def generate_reject_acnp(applied_to, ns_allow_list):
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in ns_allow_list:
        return ""
    np_name = generate_policy_name("recommend-reject-acnp")
    applied_to = antrea_crd.NetworkPolicyPeer(
        pod_selector = kubernetes.client.V1LabelSelector(
            match_labels = json.loads(labels)
        ),
        namespace_selector = kubernetes.client.V1LabelSelector(
            match_labels = {
                "kubernetes.io/metadata.name":ns
            }
        )
    )
    np = antrea_crd.ClusterNetworkPolicy(
        kind = "ClusterNetworkPolicy",
        api_version = "crd.antrea.io/v1alpha1",
        metadata = kubernetes.client.V1ObjectMeta(
            name = np_name,
        ),
        spec = antrea_crd.NetworkPolicySpec(
            tier = "Baseline",
            priority = DEFAULT_POLICY_PRIORITY,
            applied_to = [applied_to],
            egress = [antrea_crd.Rule(
                action = "Reject",
                to = [antrea_crd.NetworkPolicyPeer(
                    pod_selector = kubernetes.client.V1LabelSelector())]
            )],
            ingress = [antrea_crd.Rule(
                action = "Reject",
                _from = [antrea_crd.NetworkPolicyPeer(
                    pod_selector = kubernetes.client.V1LabelSelector())]
            )],
        )
    )
    return dict_to_yaml(np.to_dict())

class Result:
    def __init__(self, job_type, recommendation_id, policy):
        self.job_type = job_type
        if not recommendation_id:
            self.recommendation_id = str(uuid.uuid4())
        else:
            self.recommendation_id = recommendation_id
        self.time_created = datetime.datetime.now()
        self.yamls = policy

class PolicyRecommendation:
    def __init__(self):
        self._ingresses = set()
        self._egresses = set()

    def process(self,
                job_type,
                recommendation_id,
                isolation_method,
                ns_allow_list,
                applied_to,
                ingress,
                egress):
        assert(job_type == "initial")
        # ideally this would be done in the constructor, but this is not
        # supported in Snowflake (passing arguments once via the constructor)
        # instead we will keep overriding self._job_type with the same value
        self._job_type = job_type
        self._recommendation_id = recommendation_id
        self._isolation_method = isolation_method
        self._ns_allow_list = ns_allow_list
        self._applied_to = applied_to
        self._ingresses.add(ingress)
        self._egresses.add(egress)
        yield None

    def end_partition(self):
        ns_allow_list = self._ns_allow_list.split(',')
        if self._isolation_method == 3:
            allow_policy = generate_k8s_np(self._applied_to, self._ingresses, self._egresses, ns_allow_list)
            if allow_policy:
                result = Result(self._job_type,  self._recommendation_id, allow_policy)
                yield(result.job_type, result.recommendation_id, result.time_created, result.yamls)
        else:
            allow_policy = generate_anp(self._applied_to, self._ingresses, self._egresses, ns_allow_list)
            if allow_policy:
                result = Result(self._job_type,  self._recommendation_id, allow_policy)
                yield(result.job_type, result.recommendation_id, result.time_created, result.yamls)
            if self._isolation_method == 1:
                reject_policy = generate_reject_acnp(self._applied_to, ns_allow_list)
                if reject_policy:
                    result = Result(self._job_type,  self._recommendation_id, reject_policy)
                    yield(result.job_type, result.recommendation_id, result.time_created, result.yamls)
