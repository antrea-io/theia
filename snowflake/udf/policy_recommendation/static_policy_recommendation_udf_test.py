import unittest
import random

from static_policy_recommendation_udf import *

class TestStaticPolicyRecommendation(unittest.TestCase):
    expected_ns_allow_policies = [
"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-allow-acnp-kube-system-y0cq6
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: kube-system
  egress:
  - action: Allow
    to:
    - podSelector: {}
  ingress:
  - action: Allow
    from:
    - podSelector: {}
  priority: 5
  tier: Platform
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-allow-acnp-flow-aggregator-5zt4w
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: flow-aggregator
  egress:
  - action: Allow
    to:
    - podSelector: {}
  ingress:
  - action: Allow
    from:
    - podSelector: {}
  priority: 5
  tier: Platform
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-allow-acnp-flow-visibility-n6isg
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: flow-visibility
  egress:
  - action: Allow
    to:
    - podSelector: {}
  ingress:
  - action: Allow
    from:
    - podSelector: {}
  priority: 5
  tier: Platform
""",
]

    expected_reject_all_acnp = [
"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-reject-all-acnp
spec:
  appliedTo:
  - namespaceSelector: {}
    podSelector: {}
  egress:
  - action: Reject
    to:
    - podSelector: {}
  ingress:
  - action: Reject
    from:
    - podSelector: {}
  priority: 5
  tier: Baseline
"""
]

    def setup(self):
        self.static_policy_recommendation = StaticPolicyRecommendation()

    def process(self,
                jobType="initial",
                recommendationId="",
                isolationMethod=1,
                nsAllowList=""):
        next(self.static_policy_recommendation.process(
            jobType=jobType,
            recommendationId=recommendationId,
            isolationMethod=isolationMethod,
            nsAllowList=nsAllowList,
        ))
    
    def test_end_partition(self):
        for isolationMethod, nsAllowList, expected_policies in [
            (1, "kube-system,flow-aggregator,flow-visibility", self.expected_ns_allow_policies),
            (2, "", self.expected_reject_all_acnp),
            (3, "", []),
        ]:
            self.setup()
            self.process(isolationMethod=isolationMethod, nsAllowList=nsAllowList)
            random.seed(0)
            for expected_policy, result in zip(expected_policies, self.static_policy_recommendation.end_partition()):
                _, _, _, yamls = result
                self.assertEqual(yamls, expected_policy)

if __name__ == "__main__":
    unittest.main()
