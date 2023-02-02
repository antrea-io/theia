import datetime
import uuid

import kubernetes.client

import policy_recommendation.antrea_crd as antrea_crd
from policy_recommendation.policy_recommendation_utils import *
from policy_recommendation.policy_recommendation_udf import generate_policy_name, DEFAULT_POLICY_PRIORITY

def recommend_policies_for_ns_allow_list(ns_allow_list):
    policies = []
    for ns in ns_allow_list:
        np_name = generate_policy_name("recommend-allow-acnp-{}".format(ns))
        acnp = antrea_crd.ClusterNetworkPolicy(
            kind = "ClusterNetworkPolicy",
            api_version = "crd.antrea.io/v1alpha1",
            metadata = kubernetes.client.V1ObjectMeta(
                name = np_name,
            ),
            spec = antrea_crd.NetworkPolicySpec(
                tier = "Platform",
                priority = DEFAULT_POLICY_PRIORITY,
                applied_to = [antrea_crd.NetworkPolicyPeer(
                    namespace_selector = kubernetes.client.V1LabelSelector(
                        match_labels = {
                            "kubernetes.io/metadata.name":ns
                        }
                    )
                )],
                egress = [antrea_crd.Rule(
                    action = "Allow",
                    to = [antrea_crd.NetworkPolicyPeer(
                        pod_selector = kubernetes.client.V1LabelSelector())]
                )],
                ingress = [antrea_crd.Rule(
                    action = "Allow",
                    _from = [antrea_crd.NetworkPolicyPeer(
                        pod_selector = kubernetes.client.V1LabelSelector())]
                )],
            )
        )
        policies.append(dict_to_yaml(acnp.to_dict()))
    return policies

def reject_all_acnp():
    np = antrea_crd.ClusterNetworkPolicy(
        kind = "ClusterNetworkPolicy",
        api_version = "crd.antrea.io/v1alpha1",
        metadata = kubernetes.client.V1ObjectMeta(
            name = "recommend-reject-all-acnp",
        ),
        spec = antrea_crd.NetworkPolicySpec(
            tier = "Baseline",
            priority = DEFAULT_POLICY_PRIORITY,
            applied_to = [antrea_crd.NetworkPolicyPeer(
                pod_selector = kubernetes.client.V1LabelSelector(),
                namespace_selector = kubernetes.client.V1LabelSelector()
            )],
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

class StaticPolicyRecommendation:
    def __init__(self):
       return

    def process(self,
                job_type,
                recommendation_id,
                isolation_method,
                ns_allow_list):
        self._job_type = job_type
        self._recommendation_id = recommendation_id
        self._ns_allow_list = ns_allow_list
        self._isolation_method = isolation_method
        yield None
    
    def end_partition(self):
        if self._ns_allow_list:
            ns_allow_policies = recommend_policies_for_ns_allow_list(self._ns_allow_list.split(','))
            for policy in ns_allow_policies:
                result = Result(self._job_type, self._recommendation_id, policy)
                yield(result.job_type, result.recommendation_id, result.time_created, result.yamls)
        if self._isolation_method == 2:
            reject_all_policy = reject_all_acnp()
            result = Result(self._job_type, self._recommendation_id, reject_all_policy)
            yield(result.job_type, result.recommendation_id, result.time_created, result.yamls)
