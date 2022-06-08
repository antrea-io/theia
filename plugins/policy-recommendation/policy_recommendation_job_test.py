# Copyright 2022 Antrea Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import pytest
import random
import yaml
import kubernetes.client

from pyspark.sql import SparkSession, Row

import antrea_crd
import policy_recommendation_job as pr


@pytest.fixture(scope="session")
def spark_session(request):
    spark_session = (
        SparkSession.builder.master("local")
        .appName("policy_recommendation_job_test")
        .getOrCreate()
    )
    request.addfinalizer(lambda: spark_session.sparkContext.stop())
    return spark_session


@pytest.mark.parametrize(
    "test_input, expected_flow_type",
    [
        ((3, "", ""), "pod_to_external"),
        ((1, "", '{"podname":"perftest-c"}'), "pod_to_pod"),
        (
            (1, "antrea-e2e/perftestsvc:5201", '{"podname":"perftest-c"}'),
            "pod_to_svc",
        ),
    ],
)
def test_get_flow_type(test_input, expected_flow_type):
    flowType, destinationServicePortName, destinationPodLabels = test_input
    flow_type = pr.get_flow_type(flowType, destinationServicePortName,
                                 destinationPodLabels)
    assert flow_type == expected_flow_type


@pytest.mark.parametrize(
    "test_input, expected_labels",
    [
        ('{"podname": "perftest-b"}', '{"podname": "perftest-b"}'),
        (
            '{"podname": "perftest-b", "pod-template-hash": "abcde"}',
            '{"podname": "perftest-b"}',
        ),
        ("wrongformat-pod-labels", ""),
    ],
)
def test_remove_meaningless_labels(test_input, expected_labels):
    pod_labels = pr.remove_meaningless_labels(test_input)
    assert pod_labels == expected_labels


@pytest.mark.parametrize(
    "test_input, expected_proto_string",
    [
        (6, "TCP"),
        (17, "UDP"),
        (123, "UNKNOWN"),
    ],
)
def test_get_protocol_string(test_input, expected_proto_string):
    proto_string = pr.get_protocol_string(test_input)
    assert proto_string == expected_proto_string


table_name = "default.flows"


@pytest.mark.parametrize(
    "test_input, expected_sql_query",
    [
        (
            (0, "", "", True),
            "SELECT {} FROM {} WHERE ingressNetworkPolicyName == '' AND \
egressNetworkPolicyName == '' GROUP BY {}".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
        (
            (0, "", "", False),
            "SELECT {} FROM {} WHERE trusted == 1 GROUP BY {}".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
        (
            (100, "", "", True),
            "SELECT {} FROM {} WHERE ingressNetworkPolicyName == '' AND \
egressNetworkPolicyName == '' GROUP BY {} LIMIT 100".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
        (
            (100, "2022-01-01 00:00:00", "", True),
            "SELECT {} FROM {} WHERE ingressNetworkPolicyName == '' AND \
egressNetworkPolicyName == '' AND flowStartSeconds >= '2022-01-01 00:00:00' \
GROUP BY {} LIMIT 100".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
        (
            (100, "", "2022-01-01 23:59:59", True),
            "SELECT {} FROM {} WHERE ingressNetworkPolicyName == '' AND \
egressNetworkPolicyName == '' AND flowEndSeconds < '2022-01-01 23:59:59' \
GROUP BY {} LIMIT 100".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
        (
            (100, "2022-01-01 00:00:00", "2022-01-01 23:59:59", True),
            "SELECT {} FROM {} WHERE ingressNetworkPolicyName == '' AND \
egressNetworkPolicyName == '' AND flowStartSeconds >= '2022-01-01 00:00:00' \
AND flowEndSeconds < '2022-01-01 23:59:59' GROUP BY {} LIMIT 100".format(
                ", ".join(pr.FLOW_TABLE_COLUMNS),
                table_name,
                ", ".join(pr.FLOW_TABLE_COLUMNS),
            ),
        ),
    ],
)
def test_generate_sql_query(test_input, expected_sql_query):
    limit, start_time, end_time, unprotected = test_input
    sql_query = pr.generate_sql_query(
        table_name, limit, start_time, end_time, unprotected
    )
    assert sql_query == expected_sql_query


@pytest.mark.parametrize(
    "test_input, expected_policies",
    [
        (
            pr.NAMESPACE_ALLOW_LIST,
            {
                ns: {
                    "apiVersion": "crd.antrea.io/v1alpha1",
                    "kind": "ClusterNetworkPolicy",
                    "metadata": {
                        "name": "recommend-allow-acnp-{}-74D9G".format(ns)
                    },
                    "spec": {
                        "priority": 5,
                        "tier": "Platform",
                        "appliedTo": [
                            {
                                "namespaceSelector": {
                                    "matchLabels": {
                                        "kubernetes.io/metadata.name": "{}"
                                        .format(ns)
                                    }
                                }
                            }
                        ],
                        "egress": [
                            {"action": "Allow", "to": [{"podSelector": {}}]}
                        ],
                        "ingress": [
                            {"action": "Allow", "from": [{"podSelector": {}}]}
                        ],
                    },
                }
                for ns in pr.NAMESPACE_ALLOW_LIST
            },
        ),
    ],
)
def test_recommend_policies_for_ns_allow_list(test_input, expected_policies):
    recommend_policies_yamls = pr.recommend_policies_for_ns_allow_list(
        test_input)
    recommend_policies_dicts = [
        yaml.load(i, Loader=yaml.FullLoader) for i in recommend_policies_yamls
    ]
    assert len(recommend_policies_dicts) == len(expected_policies)
    for policy in recommend_policies_dicts:
        assert (
            "spec" in policy and
            "appliedTo" in policy["spec"] and
            len(policy["spec"]["appliedTo"]) == 1 and
            "namespaceSelector" in policy["spec"]["appliedTo"][0] and
            "matchLabels" in
            policy["spec"]["appliedTo"][0]["namespaceSelector"] and
            "kubernetes.io/metadata.name" in
            policy["spec"]["appliedTo"][0]["namespaceSelector"]["matchLabels"]
        )
        namespace = policy["spec"]["appliedTo"][0]["namespaceSelector"][
            "matchLabels"
        ]["kubernetes.io/metadata.name"]
        expect_policy = expected_policies[namespace]
        assert (
            "metadata" in policy and
            "name" in policy["metadata"] and
            policy["metadata"]["name"].startswith(
                "recommend-allow-acnp-{}".format(namespace)
            )
        )
        policy["metadata"]["name"] = expect_policy["metadata"]["name"]
        assert policy == expect_policy


flows_input = [
    (
        "antrea-test",
        '{"podname":"perftest-a"}',
        "10.10.0.5",
        "antrea-test",
        '{"podname":"perftest-b"}',
        "",
        5201,
        6,
        "pod_to_pod",
    ),
    (
        "antrea-test",
        '{"podname":"perftest-a"}',
        "10.10.0.6",
        "antrea-test",
        '{"podname":"perftest-c"}',
        "antrea-e2e/perftestsvc:5201",
        5201,
        6,
        "pod_to_svc",
    ),
    (
        "antrea-test",
        '{"podname":"perftest-a"}',
        "192.168.0.1",
        "",
        "",
        "",
        80,
        6,
        "pod_to_external",
    ),
]

flow_rows = [
    Row(
        sourcePodNamespace="antrea-test",
        sourcePodLabels='{"podname":"perftest-a"}',
        destinationIP="10.10.0.5",
        destinationPodNamespace="antrea-test",
        destinationPodLabels='{"podname":"perftest-b"}',
        destinationServicePortName="",
        destinationTransportPort=5201,
        protocolIdentifier=6,
        flowType="pod_to_pod",
    ),
    Row(
        sourcePodNamespace="antrea-test",
        sourcePodLabels='{"podname":"perftest-a"}',
        destinationIP="10.10.0.6",
        destinationPodNamespace="antrea-test",
        destinationPodLabels='{"podname":"perftest-c"}',
        destinationServicePortName="antrea-e2e/perftestsvc:5201",
        destinationTransportPort=5201,
        protocolIdentifier=6,
        flowType="pod_to_svc",
    ),
    Row(
        sourcePodNamespace="antrea-test",
        sourcePodLabels='{"podname":"perftest-a"}',
        destinationIP="192.168.0.1",
        destinationPodNamespace="",
        destinationPodLabels="",
        destinationServicePortName="",
        destinationTransportPort=80,
        protocolIdentifier=6,
        flowType="pod_to_external",
    ),
]


@pytest.mark.parametrize(
    "test_input, expected_egress",
    [
        (
            (flow_rows[0], False),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", 'antrea-test#{"podname":"perftest-b"}#5201#TCP'),
            ),
        ),
        (
            (flow_rows[1], False),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", "antrea-e2e#perftestsvc"),
            ),
        ),
        (
            (flow_rows[2], False),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", "192.168.0.1#80#TCP"),
            ),
        ),
        (
            (flow_rows[0], True),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", 'antrea-test#{"podname":"perftest-b"}#5201#TCP'),
            ),
        ),
        (
            (flow_rows[1], True),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", 'antrea-test#{"podname":"perftest-c"}#5201#TCP'),
            ),
        ),
        (
            (flow_rows[2], True),
            (
                'antrea-test#{"podname":"perftest-a"}',
                ("", "192.168.0.1#80#TCP"),
            ),
        ),
    ],
)
def test_map_flow_to_egress(test_input, expected_egress):
    flow, k8s = test_input
    egress = pr.map_flow_to_egress(flow, k8s)
    assert egress == expected_egress


@pytest.mark.parametrize(
    "test_input, expected_egress",
    [
        (
            flow_rows[1],
            (
                'antrea-test#{"podname":"perftest-a"}',
                "antrea-e2e/perftestsvc:5201#5201#TCP",
            ),
        )
    ],
)
def test_map_flow_to_egress_svc(test_input, expected_egress):
    egress = pr.map_flow_to_egress_svc(test_input)
    assert egress == expected_egress


@pytest.mark.parametrize(
    "test_input, expected_ingress",
    [
        (
            flow_rows[0],
            (
                'antrea-test#{"podname":"perftest-b"}',
                ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
            ),
        ),
        (
            flow_rows[1],
            (
                'antrea-test#{"podname":"perftest-c"}',
                ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
            ),
        ),
        (
            flow_rows[2],
            ("#", ('antrea-test#{"podname":"perftest-a"}#80#TCP', "")),
        ),
    ],
)
def test_map_flow_to_ingress(test_input, expected_ingress):
    ingress = pr.map_flow_to_ingress(test_input)
    assert ingress == expected_ingress


@pytest.mark.parametrize(
    "test_input, expected_peer",
    [
        (
            (
                ("", 'antrea-test#{"podname":"perftest-a"}#5201#TCP'),
                ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
            ),
            (
                'antrea-test#{"podname":"perftest-a"}#5201#TCP',
                'antrea-test#{"podname":"perftest-a"}#5201#TCP',
            ),
        ),
        (
            (
                ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
                ("", ""),
            ),
            ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
        ),
        (
            (
                ('antrea-test#{"podname":"perftest-a"}#5201#TCP', ""),
                ("", 'antrea-test#{"podname":"perftest-b"}#5201#TCP'),
            ),
            (
                'antrea-test#{"podname":"perftest-a"}#5201#TCP',
                'antrea-test#{"podname":"perftest-b"}#5201#TCP',
            ),
        ),
    ],
)
def test_combine_network_peers(test_input, expected_peer):
    peer1, peer2 = test_input
    network_peer = pr.combine_network_peers(peer1, peer2)
    assert network_peer == expected_peer


@pytest.mark.parametrize(
    "test_input, expected_egress_rule",
    [
        (
            "wrongformat",
            "",
        ),
        (
            'antrea-test#{"podname":"perftest-a"}#5201#TCP',
            kubernetes.client.V1NetworkPolicyEgressRule(
                to=[
                    kubernetes.client.V1NetworkPolicyPeer(
                        namespace_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"name": "antrea-test"}
                        ),
                        pod_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"podname": "perftest-a"}
                        ),
                    )
                ],
                ports=[
                    kubernetes.client.V1NetworkPolicyPort(
                        port=5201, protocol="TCP"
                    )
                ],
            ),
        ),
        (
            "192.168.0.1#80#TCP",
            kubernetes.client.V1NetworkPolicyEgressRule(
                to=[
                    kubernetes.client.V1NetworkPolicyPeer(
                        ip_block=kubernetes.client.V1IPBlock(
                            cidr="192.168.0.1/32",
                        )
                    )
                ],
                ports=[
                    kubernetes.client.V1NetworkPolicyPort(
                        port=80, protocol="TCP"
                    )
                ],
            ),
        ),
        (
            "fd12:3456:789a:1::1#80#TCP",
            kubernetes.client.V1NetworkPolicyEgressRule(
                to=[
                    kubernetes.client.V1NetworkPolicyPeer(
                        ip_block=kubernetes.client.V1IPBlock(
                            cidr="fd12:3456:789a:1::1/128",
                        )
                    )
                ],
                ports=[
                    kubernetes.client.V1NetworkPolicyPort(
                        port=80, protocol="TCP"
                    )
                ],
            ),
        ),
    ],
)
def test_generate_k8s_egress_rule(test_input, expected_egress_rule):
    if test_input == "wrongformat":
        with pytest.raises(SystemExit) as pytest_wrapped_e:
            pr.generate_k8s_egress_rule(test_input)
        assert pytest_wrapped_e.type == SystemExit
        assert pytest_wrapped_e.value.code == 1
    else:
        egress_rule = pr.generate_k8s_egress_rule(test_input)
        assert egress_rule == expected_egress_rule


@pytest.mark.parametrize(
    "test_input, expected_ingress_rule",
    [
        (
            "wrongformat",
            "",
        ),
        (
            'antrea-test#{"podname":"perftest-a"}#5201#TCP',
            kubernetes.client.V1NetworkPolicyIngressRule(
                _from=[
                    kubernetes.client.V1NetworkPolicyPeer(
                        namespace_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"name": "antrea-test"}
                        ),
                        pod_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"podname": "perftest-a"}
                        ),
                    )
                ],
                ports=[
                    kubernetes.client.V1NetworkPolicyPort(
                        port=5201, protocol="TCP"
                    )
                ],
            ),
        ),
    ],
)
def test_generate_k8s_ingress_rule(test_input, expected_ingress_rule):
    if test_input == "wrongformat":
        with pytest.raises(SystemExit) as pytest_wrapped_e:
            pr.generate_k8s_ingress_rule(test_input)
        assert pytest_wrapped_e.type == SystemExit
        assert pytest_wrapped_e.value.code == 1
    else:
        ingress_rule = pr.generate_k8s_ingress_rule(test_input)
        assert ingress_rule == expected_ingress_rule


@pytest.mark.parametrize(
    "test_input, expected_policy_name",
    [
        ("recommend-k8s-np", "recommend-k8s-np-y0cq6"),
        ("recommend-allow-anp", "recommend-allow-anp-y0cq6"),
    ],
)
def test_generate_policy_name(test_input, expected_policy_name):
    random.seed(0)
    policy_name = pr.generate_policy_name(test_input)
    assert policy_name == expected_policy_name


@pytest.mark.parametrize(
    "test_input, expected_cg_name",
    [(("antrea-e2e", "perfsvc"), "cg-antrea-e2e-perfsvc")],
)
def test_get_svc_cg_name(test_input, expected_cg_name):
    ns, svc_name = test_input
    cg_name = pr.get_svc_cg_name(ns, svc_name)
    assert cg_name == expected_cg_name


@pytest.mark.parametrize(
    "test_input, expected_egress_rule",
    [
        (
            "wrongformat",
            "",
        ),
        (
            'antrea-test#{"podname":"perftest-a"}#5201#TCP',
            antrea_crd.Rule(
                action="Allow",
                to=[
                    antrea_crd.NetworkPolicyPeer(
                        namespace_selector=kubernetes.client.V1LabelSelector(
                            match_labels={
                                "kubernetes.io/metadata.name": "antrea-test"
                            }
                        ),
                        pod_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"podname": "perftest-a"}
                        ),
                    )
                ],
                ports=[
                    antrea_crd.NetworkPolicyPort(port=5201, protocol="TCP")
                ],
            ),
        ),
        (
            "192.168.0.1#80#TCP",
            antrea_crd.Rule(
                action="Allow",
                to=[
                    antrea_crd.NetworkPolicyPeer(
                        ip_block=antrea_crd.IPBlock(
                            CIDR="192.168.0.1/32",
                        )
                    )
                ],
                ports=[antrea_crd.NetworkPolicyPort(port=80, protocol="TCP")],
            ),
        ),
        (
            "fd12:3456:789a:1::1#80#TCP",
            antrea_crd.Rule(
                action="Allow",
                to=[
                    antrea_crd.NetworkPolicyPeer(
                        ip_block=antrea_crd.IPBlock(
                            CIDR="fd12:3456:789a:1::1/128",
                        )
                    )
                ],
                ports=[antrea_crd.NetworkPolicyPort(port=80, protocol="TCP")],
            ),
        ),
        (
            "antrea-e2e#perftestsvc",
            antrea_crd.Rule(
                action="Allow",
                to_services=[
                    antrea_crd.NamespacedName(
                        namespace="antrea-e2e", name="perftestsvc"
                    )
                ],
            ),
        ),
    ],
)
def test_generate_anp_egress_rule(test_input, expected_egress_rule):
    if test_input == "wrongformat":
        with pytest.raises(SystemExit) as pytest_wrapped_e:
            pr.generate_anp_egress_rule(test_input)
        assert pytest_wrapped_e.type == SystemExit
        assert pytest_wrapped_e.value.code == 1
    else:
        egress_rule = pr.generate_anp_egress_rule(test_input)
        assert egress_rule == expected_egress_rule


@pytest.mark.parametrize(
    "test_input, expected_ingress_rule",
    [
        (
            "wrongformat",
            "",
        ),
        (
            'antrea-test#{"podname":"perftest-a"}#5201#TCP',
            antrea_crd.Rule(
                action="Allow",
                _from=[
                    antrea_crd.NetworkPolicyPeer(
                        namespace_selector=kubernetes.client.V1LabelSelector(
                            match_labels={
                                "kubernetes.io/metadata.name": "antrea-test"
                            }
                        ),
                        pod_selector=kubernetes.client.V1LabelSelector(
                            match_labels={"podname": "perftest-a"}
                        ),
                    )
                ],
                ports=[
                    antrea_crd.NetworkPolicyPort(port=5201, protocol="TCP")
                ],
            ),
        ),
    ],
)
def test_generate_anp_ingress_rule(test_input, expected_ingress_rule):
    if test_input == "wrongformat":
        with pytest.raises(SystemExit) as pytest_wrapped_e:
            pr.generate_anp_ingress_rule(test_input)
        assert pytest_wrapped_e.type == SystemExit
        assert pytest_wrapped_e.value.code == 1
    else:
        ingress_rule = pr.generate_anp_ingress_rule(test_input)
        assert ingress_rule == expected_ingress_rule


@pytest.mark.parametrize(
    "flows_input, expected_policies",
    [
        (
            flows_input,
            {
                "perftest-b": {
                    "apiVersion": "networking.k8s.io/v1",
                    "kind": "NetworkPolicy",
                    "metadata": {
                        "name": "recommend-k8s-np-BzeCA",
                        "namespace": "antrea-test",
                    },
                    "spec": {
                        "egress": [],
                        "ingress": [
                            {
                                "from": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-a"
                                            }
                                        },
                                    }
                                ],
                                "ports": [{"port": 5201, "protocol": "TCP"}],
                            }
                        ],
                        "podSelector": {
                            "matchLabels": {"podname": "perftest-b"}
                        },
                        "policyTypes": ["Ingress"],
                    },
                },
                "perftest-c": {
                    "apiVersion": "networking.k8s.io/v1",
                    "kind": "NetworkPolicy",
                    "metadata": {
                        "name": "recommend-k8s-np-j5c1d",
                        "namespace": "antrea-test",
                    },
                    "spec": {
                        "egress": [],
                        "ingress": [
                            {
                                "from": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-a"
                                            }
                                        },
                                    }
                                ],
                                "ports": [{"port": 5201, "protocol": "TCP"}],
                            },
                        ],
                        "podSelector": {
                            "matchLabels": {"podname": "perftest-c"}
                        },
                        "policyTypes": ["Ingress"],
                    },
                },
                "perftest-a": {
                    "apiVersion": "networking.k8s.io/v1",
                    "kind": "NetworkPolicy",
                    "metadata": {
                        "name": "recommend-k8s-np-OiBQn",
                        "namespace": "antrea-test",
                    },
                    "spec": {
                        "egress": [
                            {
                                "ports": [{"port": 5201, "protocol": "TCP"}],
                                "to": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-c"
                                            }
                                        },
                                    }
                                ],
                            },
                            {
                                "ports": [{"port": 5201, "protocol": "TCP"}],
                                "to": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-b"
                                            }
                                        },
                                    }
                                ],
                            },
                            {
                                "ports": [{"port": 80, "protocol": "TCP"}],
                                "to": [
                                    {"ipBlock": {"cidr": "192.168.0.1/32"}}
                                ],
                            },
                        ],
                        "ingress": [],
                        "podSelector": {
                            "matchLabels": {"podname": "perftest-a"}
                        },
                        "policyTypes": ["Egress"],
                    },
                },
            },
        )
    ],
)
def test_recommend_k8s_policies(spark_session, flows_input, expected_policies):
    test_df = spark_session.createDataFrame(
        flows_input, pr.FLOW_TABLE_COLUMNS
    )
    recommend_k8s_policies_yamls = pr.recommend_k8s_policies(test_df)
    recommend_k8s_policies_dicts = [
        yaml.load(i, Loader=yaml.FullLoader)
        for i in recommend_k8s_policies_yamls
    ]
    assert len(recommend_k8s_policies_dicts) == len(expected_policies)
    for policy in recommend_k8s_policies_dicts:
        assert (
            "spec" in policy and
            "podSelector" in policy["spec"] and
            "matchLabels" in policy["spec"]["podSelector"] and
            "podname" in policy["spec"]["podSelector"]["matchLabels"]
        )
        podname = policy["spec"]["podSelector"]["matchLabels"]["podname"]
        expect_policy = expected_policies[podname]
        assert (
            "metadata" in policy and
            "name" in policy["metadata"] and
            policy["metadata"]["name"].startswith("recommend-k8s-np-")
        )
        policy["metadata"]["name"] = expect_policy["metadata"]["name"]
        assert policy == expect_policy


@pytest.mark.parametrize(
    "test_input, expected_policies",
    [
        (
            (flows_input, 1, True, False),
            {
                "NetworkPolicy": {
                    "perftest-b": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-53JbG",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-b"
                                        }
                                    }
                                }
                            ],
                            "egress": [],
                            "ingress": [
                                {
                                    "action": "Allow",
                                    "from": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-a"
                                                }
                                            },
                                        }
                                    ],
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                }
                            ],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                    "perftest-a": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-eDJzR",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-a"
                                        }
                                    }
                                }
                            ],
                            "egress": [
                                {
                                    "action": "Allow",
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                    "to": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-b"
                                                }
                                            },
                                        }
                                    ],
                                },
                                {
                                    "action": "Allow",
                                    "ports": [{"port": 80, "protocol": "TCP"}],
                                    "to": [
                                        {"ipBlock": {"cidr": "192.168.0.1/32"}}
                                    ],
                                },
                            ],
                            "ingress": [],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                    "perftest-c": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-ax8fj",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-c"
                                        }
                                    }
                                }
                            ],
                            "egress": [],
                            "ingress": [
                                {
                                    "action": "Allow",
                                    "from": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-a"
                                                }
                                            },
                                        }
                                    ],
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                }
                            ],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                },
                "ClusterGroup": {
                    "apiVersion": "crd.antrea.io/v1alpha2",
                    "kind": "ClusterGroup",
                    "metadata": {"name": "cg-antrea-e2e-perftestsvc"},
                    "spec": {
                        "serviceReference": {
                            "name": "perftestsvc",
                            "namespace": "antrea-e2e",
                        }
                    },
                },
                "ClusterNetworkPolicy": {
                    "Application": {
                        "perftest-a": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-svc-allow-acnp-sGSj9"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-a"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Allow",
                                        "ports": [
                                            {"port": 5201, "protocol": "TCP"}
                                        ],
                                        "to": [
                                            {
                                                "group":
                                                "cg-antrea-e2e-perftestsvc"
                                            }
                                        ],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Application",
                            },
                        },
                    },
                    "Baseline": {
                        "perftest-a": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-OpeDq"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-a"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                        "perftest-b": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-trSQN"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-b"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                        "perftest-c": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-trSQN"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-c"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                    },
                },
            },
        ),
        (
            (flows_input, 1, True, True),
            {
                "NetworkPolicy": {
                    "perftest-b": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-53JbG",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-b"
                                        }
                                    }
                                }
                            ],
                            "egress": [],
                            "ingress": [
                                {
                                    "action": "Allow",
                                    "from": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-a"
                                                }
                                            },
                                        }
                                    ],
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                }
                            ],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                    "perftest-a": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-eDJzR",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-a"
                                        }
                                    }
                                }
                            ],
                            "egress": [
                                {
                                    "action": "Allow",
                                    "toServices": [
                                        {
                                            "name": "perftestsvc",
                                            "namespace": "antrea-e2e",
                                        }
                                    ],
                                },
                                {
                                    "action": "Allow",
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                    "to": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-b"
                                                }
                                            },
                                        }
                                    ],
                                },
                                {
                                    "action": "Allow",
                                    "ports": [{"port": 80, "protocol": "TCP"}],
                                    "to": [
                                        {"ipBlock": {"cidr": "192.168.0.1/32"}}
                                    ],
                                },
                            ],
                            "ingress": [],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                    "perftest-c": {
                        "apiVersion": "crd.antrea.io/v1alpha1",
                        "kind": "NetworkPolicy",
                        "metadata": {
                            "name": "recommend-allow-anp-ax8fj",
                            "namespace": "antrea-test",
                        },
                        "spec": {
                            "appliedTo": [
                                {
                                    "podSelector": {
                                        "matchLabels": {
                                            "podname": "perftest-c"
                                        }
                                    }
                                }
                            ],
                            "egress": [],
                            "ingress": [
                                {
                                    "action": "Allow",
                                    "from": [
                                        {
                                            "namespaceSelector": {
                                                "matchLabels": {
                                                    "kubernetes.io\
/metadata.name": "antrea-test"
                                                }
                                            },
                                            "podSelector": {
                                                "matchLabels": {
                                                    "podname": "perftest-a"
                                                }
                                            },
                                        }
                                    ],
                                    "ports": [
                                        {"port": 5201, "protocol": "TCP"}
                                    ],
                                }
                            ],
                            "priority": 5,
                            "tier": "Application",
                        },
                    },
                },
                "ClusterNetworkPolicy": {
                    "Baseline": {
                        "perftest-a": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-OpeDq"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-a"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                        "perftest-b": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-trSQN"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-b"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                        "perftest-c": {
                            "apiVersion": "crd.antrea.io/v1alpha1",
                            "kind": "ClusterNetworkPolicy",
                            "metadata": {
                                "name": "recommend-reject-acnp-trSQN"
                            },
                            "spec": {
                                "appliedTo": [
                                    {
                                        "namespaceSelector": {
                                            "matchLabels": {
                                                "kubernetes.io\
/metadata.name": "antrea-test"
                                            }
                                        },
                                        "podSelector": {
                                            "matchLabels": {
                                                "podname": "perftest-c"
                                            }
                                        },
                                    }
                                ],
                                "egress": [
                                    {
                                        "action": "Reject",
                                        "to": [{"podSelector": {}}],
                                    }
                                ],
                                "ingress": [
                                    {
                                        "action": "Reject",
                                        "from": [{"podSelector": {}}],
                                    }
                                ],
                                "priority": 5,
                                "tier": "Baseline",
                            },
                        },
                    },
                },
            },
        ),
    ],
)
def test_recommend_antrea_policies(
    spark_session, test_input, expected_policies
):
    flows_input, option, deny_rules, to_services = test_input
    test_df = spark_session.createDataFrame(
        flows_input, pr.FLOW_TABLE_COLUMNS
    )
    recommend_policies_yamls = pr.recommend_antrea_policies(
        test_df, option, deny_rules, to_services
    )
    recommend_policies_dicts = [
        yaml.load(i, Loader=yaml.FullLoader) for i in recommend_policies_yamls
    ]
    for policy in recommend_policies_dicts:
        assert "kind" in policy and policy["kind"] in expected_policies
        if policy["kind"] == "NetworkPolicy":
            assert (
                "spec" in policy and
                "appliedTo" in policy["spec"] and
                len(policy["spec"]["appliedTo"]) == 1 and
                "podSelector" in policy["spec"]["appliedTo"][0] and
                "matchLabels"
                in policy["spec"]["appliedTo"][0]["podSelector"] and
                "podname"
                in policy["spec"]["appliedTo"][0]["podSelector"]["matchLabels"]
            )
            podname = policy["spec"]["appliedTo"][0]["podSelector"][
                "matchLabels"
            ]["podname"]
            expect_policy = expected_policies[policy["kind"]][podname]
            assert (
                "metadata" in policy and
                "name" in policy["metadata"] and
                policy["metadata"]["name"].startswith(
                    "recommend-allow-anp-"
                )
            )
            policy["metadata"]["name"] = expect_policy["metadata"]["name"]
            assert policy == expect_policy
        elif policy["kind"] == "ClusterGroup":
            assert policy == expected_policies[policy["kind"]]
        else:
            assert "spec" in policy and "tier" in policy["spec"]
            policy_tier = policy["spec"]["tier"]
            assert policy_tier in expected_policies[policy["kind"]]
            assert (
                "appliedTo" in policy["spec"] and
                len(policy["spec"]["appliedTo"]) == 1 and
                "podSelector" in policy["spec"]["appliedTo"][0] and
                "matchLabels" in
                policy["spec"]["appliedTo"][0]["podSelector"] and
                "podname" in
                policy["spec"]["appliedTo"][0]["podSelector"]["matchLabels"]
            )
            podname = policy["spec"]["appliedTo"][0]["podSelector"][
                "matchLabels"
            ]["podname"]
            expect_policy = expected_policies[policy["kind"]][policy_tier][
                podname
            ]
            assert "metadata" in policy and "name" in policy["metadata"]
            if policy_tier == "Application":
                assert (
                    policy["metadata"]["name"].startswith(
                        "recommend-svc-allow-acnp-"
                    )
                )
            else:
                assert (
                    policy["metadata"]["name"].startswith(
                        "recommend-reject-acnp-"
                    )
                )
            policy["metadata"]["name"] = expect_policy["metadata"]["name"]
            assert policy == expect_policy
