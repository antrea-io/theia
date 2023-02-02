import unittest
import random

from policy_recommendation_udf import *
    
class TestPolicyRecommendation(unittest.TestCase):
    flows_processed = [
        [
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-test#{"podname": "perftest-b"}#5201#TCP'
            ],
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-e2e#perftestsvc'
            ],
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                '192.168.0.1#80#TCP'
            ],
        ],
        [
            [
                'antrea-test#{"podname": "perftest-b"}',
                'antrea-test#{"podname": "perftest-a"}#5201#TCP',
                ''
            ]
        ],
        [
            [
                'antrea-test#{"podname": "perftest-c"}',
                'antrea-test#{"podname": "perftest-a"}#5201#TCP',
                ''
            ]
        ],
        [
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-test#{"podname": "perftest-b"}#5201#TCP'
            ],
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-test#{"podname": "perftest-c"}#5201#TCP'
            ],
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                '192.168.0.1#80#TCP'
            ],
        ],
    ]

    expected_k8s_policies = [
"""apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: recommend-k8s-np-y0cq6
  namespace: antrea-test
spec:
  egress:
  - ports:
    - port: 80
      protocol: TCP
    to:
    - ipBlock:
        cidr: 192.168.0.1/32
  - ports:
    - port: 5201
      protocol: TCP
    to:
    - namespaceSelector:
        matchLabels:
          name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-b
  - ports:
    - port: 5201
      protocol: TCP
    to:
    - namespaceSelector:
        matchLabels:
          name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-c
  ingress: []
  podSelector:
    matchLabels:
      podname: perftest-a
  policyTypes:
  - Egress
""",

"""apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: recommend-k8s-np-y0cq6
  namespace: antrea-test
spec:
  egress: []
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-a
    ports:
    - port: 5201
      protocol: TCP
  podSelector:
    matchLabels:
      podname: perftest-b
  policyTypes:
  - Ingress
""",

"""apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: recommend-k8s-np-y0cq6
  namespace: antrea-test
spec:
  egress: []
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-a
    ports:
    - port: 5201
      protocol: TCP
  podSelector:
    matchLabels:
      podname: perftest-c
  policyTypes:
  - Ingress
""",
]

    expected_allow_antrea_policies = [
"""apiVersion: crd.antrea.io/v1alpha1
kind: NetworkPolicy
metadata:
  name: recommend-allow-anp-y0cq6
  namespace: antrea-test
spec:
  appliedTo:
  - podSelector:
      matchLabels:
        podname: perftest-a
  egress:
  - action: Allow
    ports:
    - port: 80
      protocol: TCP
    to:
    - ipBlock:
        cidr: 192.168.0.1/32
  - action: Allow
    toServices:
    - name: perftestsvc
      namespace: antrea-e2e
  - action: Allow
    ports:
    - port: 5201
      protocol: TCP
    to:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-b
  ingress: []
  priority: 5
  tier: Application
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: NetworkPolicy
metadata:
  name: recommend-allow-anp-y0cq6
  namespace: antrea-test
spec:
  appliedTo:
  - podSelector:
      matchLabels:
        podname: perftest-b
  egress: []
  ingress:
  - action: Allow
    from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-a
    ports:
    - port: 5201
      protocol: TCP
  priority: 5
  tier: Application
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: NetworkPolicy
metadata:
  name: recommend-allow-anp-y0cq6
  namespace: antrea-test
spec:
  appliedTo:
  - podSelector:
      matchLabels:
        podname: perftest-c
  egress: []
  ingress:
  - action: Allow
    from:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: antrea-test
      podSelector:
        matchLabels:
          podname: perftest-a
    ports:
    - port: 5201
      protocol: TCP
  priority: 5
  tier: Application
""",
]


    expected_reject_acnp = [
"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-reject-acnp-5zt4w
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: antrea-test
    podSelector:
      matchLabels:
        podname: perftest-a
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
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-reject-acnp-5zt4w
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: antrea-test
    podSelector:
      matchLabels:
        podname: perftest-b
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
""",

"""apiVersion: crd.antrea.io/v1alpha1
kind: ClusterNetworkPolicy
metadata:
  name: recommend-reject-acnp-5zt4w
spec:
  appliedTo:
  - namespaceSelector:
      matchLabels:
        kubernetes.io/metadata.name: antrea-test
    podSelector:
      matchLabels:
        podname: perftest-c
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
""",
]


    def setup(self):
        self.policy_recommendation = PolicyRecommendation()

    def process_flows(self,
                      flows,
                      job_type="initial",
                      isolation_method=3,
                      ns_allow_list="kube-system,flow-aggregator,flow-visibility"
                      ):
        for flow in flows:
            next(self.policy_recommendation.process(
                job_type=job_type,
                recommendation_id="",
                isolation_method=isolation_method,
                ns_allow_list=ns_allow_list,
                applied_to=flow[0],
                ingress=flow[1],
                egress=flow[2]
            ))

    def test_end_partition(self):
        for isolation_method, flows_processed, expected_policies in [
            (1, self.flows_processed[0], [self.expected_allow_antrea_policies[0]] + [self.expected_reject_acnp[0]]),
            (1, self.flows_processed[1], [self.expected_allow_antrea_policies[1]] + [self.expected_reject_acnp[1]]),
            (1, self.flows_processed[2], [self.expected_allow_antrea_policies[2]] + [self.expected_reject_acnp[2]]),
            (2, self.flows_processed[0], [self.expected_allow_antrea_policies[0]]),
            (2, self.flows_processed[1], [self.expected_allow_antrea_policies[1]]),
            (2, self.flows_processed[2], [self.expected_allow_antrea_policies[2]]),
            (3, self.flows_processed[3], [self.expected_k8s_policies[0]]),
            (3, self.flows_processed[1], [self.expected_k8s_policies[1]]),
            (3, self.flows_processed[2], [self.expected_k8s_policies[2]]),
        ]:
            self.setup()
            self.process_flows(isolation_method=isolation_method, flows=flows_processed)
            # Initialize the random number generator to get predictable generated policy names
            random.seed(0)
            for expected_policy, result in zip(expected_policies, self.policy_recommendation.end_partition()):
                job_type, _, _, yamls = result
                self.assertEqual(yamls, expected_policy)

if __name__ == "__main__":
    unittest.main()
