#!/usr/bin/python3

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

import datetime
import getopt
import json
import logging
import os
import random
import string
import sys
import uuid

import kubernetes.client
from pyspark.sql import SparkSession
from pyspark.sql.functions import udf
from pyspark.sql.types import StringType
from urllib.parse import urlparse

import antrea_crd
from policy_recommendation_utils import *

# Column names of flow record table in Clickhouse database used in recommendation job
FLOW_TABLE_COLUMNS = [
    'sourcePodNamespace',
    'sourcePodLabels',
    'destinationIP',
    'destinationPodNamespace',
    'destinationPodLabels',
    'destinationServicePortName',
    'destinationTransportPort',
    'protocolIdentifier',
]

NAMESPACE_ALLOW_LIST = [
    'kube-system',
    'flow-aggregator',
    'flow-visibility'
]

ROW_DELIMITER = "#"
PEER_DELIMITER = "|"
DEFAULT_POLICY_PRIORITY = 5

MEANINGLESS_LABELS = [
    "pod-template-hash",
    "controller-revision-hash",
    "pod-template-generation"
]

logger = logging.getLogger('policy_recommendation')
logger.setLevel(logging.INFO)
ch = logging.StreamHandler()
ch.setLevel(logging.INFO)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
logger.addHandler(ch)

def get_flow_type(destinationServicePortName, destinationPodLabels):
    if destinationServicePortName != "":
        return "pod_to_svc"
    elif destinationPodLabels != "":
        return "pod_to_pod"
    else:
        return "pod_to_external"

def remove_meaningless_labels(podLabels):
    try:
        labels_dict = json.loads(podLabels)
    except Exception as e:
        logger.error("Error {}: labels {} are not in json format".format(e, podLabels))
        return ""
    labels_dict = {key:value for key,value in labels_dict.items() if key not in MEANINGLESS_LABELS}
    return json.dumps(labels_dict, sort_keys=True)

def get_protocol_string(protocolIdentifier):
    if protocolIdentifier == 6:
        return "TCP"
    elif protocolIdentifier == 17:
        return "UDP"
    else:
        return "UNKNOWN"

def map_flow_to_egress(flow, k8s=False):
    src = ROW_DELIMITER.join([flow.sourcePodNamespace, flow.sourcePodLabels])
    if flow.flowType == "pod_to_external":
        dst = ROW_DELIMITER.join([flow.destinationIP, str(flow.destinationTransportPort), get_protocol_string(flow.protocolIdentifier)])
    elif flow.flowType == "pod_to_svc" and not k8s:
        # K8s policies don't support Pod to Service rules
        svc_ns, svc_name = flow.destinationServicePortName.partition(':')[0].split('/')
        dst = ROW_DELIMITER.join([svc_ns, svc_name])
    else:
        dst = ROW_DELIMITER.join([flow.destinationPodNamespace, flow.destinationPodLabels, str(flow.destinationTransportPort), get_protocol_string(flow.protocolIdentifier)])
    return (src, ("", dst))

def map_flow_to_egress_svc(flow):
    src = ROW_DELIMITER.join([flow.sourcePodNamespace, flow.sourcePodLabels])
    dst = ROW_DELIMITER.join([flow.destinationServicePortName, str(flow.destinationTransportPort), get_protocol_string(flow.protocolIdentifier)])
    return (src, dst)

def map_flow_to_ingress(flow):
    src = ROW_DELIMITER.join([flow.sourcePodNamespace, flow.sourcePodLabels, str(flow.destinationTransportPort), get_protocol_string(flow.protocolIdentifier)])
    dst = ROW_DELIMITER.join([flow.destinationPodNamespace, flow.destinationPodLabels])
    return (dst, (src, ""))

# Combine the ingress row (src, "") with the egress row ("", dst) to a network peer row (src, dst)
def combine_network_peers(a, b):
    if a[0] != "":
        new_src = a[0]
    else:
        new_src = b[0]
    if a[1] != "":
        new_dst = a[1]
    else:
        new_dst = b[1]
    return (new_src, new_dst)

def generate_k8s_egress_rule(egress):
    if len(egress.split(ROW_DELIMITER)) == 4:
        ns, labels, port, protocolIdentifier = egress.split(ROW_DELIMITER)
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
        destinationIP, port, protocolIdentifier = egress.split(ROW_DELIMITER)
        if get_IP_version(destinationIP) == "v4":
            cidr = destinationIP + "/32"
        else:
            cidr = destinationIP + "/128"
        egress_peer = kubernetes.client.V1NetworkPolicyPeer(
            ip_block = kubernetes.client.V1IPBlock(
                cidr = cidr,
            )
        )
    else:
        logger.fatal("Egress tuple {} has wrong format".format(egress))
        sys.exit(1)
    ports = kubernetes.client.V1NetworkPolicyPort(
        port = int(port),
        protocol = protocolIdentifier
    )
    egress_rule = kubernetes.client.V1NetworkPolicyEgressRule(
        to = [egress_peer],
        ports = [ports]
    )
    return egress_rule

def generate_k8s_ingress_rule(ingress):
    if len(ingress.split(ROW_DELIMITER)) != 4:
        logger.fatal("Ingress tuple {} has wrong format".format(ingress))
        sys.exit(1)
    ns, labels, port, protocolIdentifier = ingress.split(ROW_DELIMITER)
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
        protocol = protocolIdentifier
    )
    ingress_rule = kubernetes.client.V1NetworkPolicyIngressRule(
        _from = [ingress_peer],
        ports = [ports]
    )
    return ingress_rule

def generate_policy_name(info):
    return "-".join([info, "".join(random.sample(string.ascii_lowercase + string.digits, 5))])

def generate_k8s_np(x):
    applied_to, (ingresses, egresses) = x
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in NAMESPACE_ALLOW_LIST:
        return []
    ingress_list = list(set(ingresses.split(PEER_DELIMITER)))
    egress_list = list(set(egresses.split(PEER_DELIMITER)))
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
        return [dict_to_yaml(np.to_dict())]
    else:
        return []

def generate_anp_egress_rule(egress):
    if len(egress.split(ROW_DELIMITER)) == 4:
        # Pod-to-Pod flow
        ns, labels, port, protocolIdentifier = egress.split(ROW_DELIMITER)
        try:
            labels_dict = json.loads(labels)
        except Exception as e:
            logger.error("Error {}: labels {} in egress {} are not in json format".format(e, labels, egress))
            return ""
        egress_peer = antrea_crd.NetworkPolicyPeer(
            namespace_selector = kubernetes.client.V1LabelSelector(
                match_labels = {
                    "kubernetes.io/metadata.name":ns
                }
            ),
            pod_selector = kubernetes.client.V1LabelSelector(
                match_labels = labels_dict
            ),
        )
        ports = antrea_crd.NetworkPolicyPort(
            protocol = protocolIdentifier,
            port = int(port)
        )
        egress_rule = antrea_crd.Rule(
            action = "Allow",
            to = [egress_peer],
            ports = [ports]
        )
    elif len(egress.split(ROW_DELIMITER)) == 3:
        # Pod-to-External flow
        destinationIP, port, protocolIdentifier = egress.split(ROW_DELIMITER)
        if get_IP_version(destinationIP) == "v4":
            cidr = destinationIP + "/32"
        else:
            cidr = destinationIP + "/128"
        egress_peer = antrea_crd.NetworkPolicyPeer(
            ip_block = antrea_crd.IPBlock(
                CIDR = cidr,
            )
        )
        ports = antrea_crd.NetworkPolicyPort(
            protocol = protocolIdentifier,
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
        logger.fatal("Egress tuple {} has wrong format".format(egress))
        sys.exit(1)
    return egress_rule

def generate_anp_ingress_rule(ingress):
    if len(ingress.split(ROW_DELIMITER)) != 4:
        logger.fatal("Ingress tuple {} has wrong format".format(ingress))
        sys.exit(1)
    ns, labels, port, protocolIdentifier = ingress.split(ROW_DELIMITER)
    try:
        labels_dict = json.loads(labels)
    except Exception as e:
        logger.error("Error {}: labels {} in ingress {} are not in json format".format(e, labels, ingress))
        return ""
    ingress_peer = antrea_crd.NetworkPolicyPeer(
        namespace_selector = kubernetes.client.V1LabelSelector(
            match_labels = {
                "kubernetes.io/metadata.name":ns
            }
        ),
        pod_selector = kubernetes.client.V1LabelSelector(
            match_labels = labels_dict
        ),
    )
    ports = antrea_crd.NetworkPolicyPort(
        protocol = protocolIdentifier,
        port = int(port)
    )
    ingress_rule = antrea_crd.Rule(
        action = "Allow",
        _from = [ingress_peer],
        ports = [ports]
    )
    return ingress_rule

def generate_anp(network_peers):
    applied_to, (ingresses, egresses) = network_peers
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in NAMESPACE_ALLOW_LIST:
        return []
    try:
        labels_dict = json.loads(labels)
    except Exception as e:
        logger.error("Error {}: labels {} in applied_to {} are not in json format".format(e, labels, applied_to))
        return []
    ingress_list = list(set(ingresses.split(PEER_DELIMITER)))
    egress_list = list(set(egresses.split(PEER_DELIMITER)))
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
            kind = "NetworkPolicy",
            api_version = "crd.antrea.io/v1alpha1",
            metadata = kubernetes.client.V1ObjectMeta(
                name = np_name,
                namespace = ns,
            ),
            spec = antrea_crd.NetworkPolicySpec(
                tier = "Application",
                priority = DEFAULT_POLICY_PRIORITY,
                applied_to = [antrea_crd.NetworkPolicyPeer(
                    pod_selector = kubernetes.client.V1LabelSelector(
                        match_labels = labels_dict
                    ),
                )],
                egress = egressRules,
                ingress = ingressRules,
            )
        )
        return [dict_to_yaml(np.to_dict())]
    return []

def get_svc_cg_name(namespace, name):
    return "-".join(["cg", namespace, name])

def generate_svc_cg(destinationServicePortNameRow):
    namespace, name = destinationServicePortNameRow.destinationServicePortName.partition(':')[0].split('/')
    if namespace in NAMESPACE_ALLOW_LIST:
        return []
    svc_cg = antrea_crd.ClusterGroup(
        kind = "ClusterGroup",
        api_version = "crd.antrea.io/v1alpha2",
        metadata = kubernetes.client.V1ObjectMeta(
            name = get_svc_cg_name(namespace, name)
        ),
        spec = antrea_crd.GroupSpec(
            service_reference = antrea_crd.ServiceReference(
                name = name,
                namespace = namespace
            )
        )
    )
    return [dict_to_yaml(svc_cg.to_dict())]

def generate_acnp_svc_egress_rule(egress):
    svcPortName, port, protocolIdentifier = egress.split(ROW_DELIMITER)
    ns, svc = svcPortName.partition(':')[0].split('/')
    egress_peer = antrea_crd.NetworkPolicyPeer(
        group = get_svc_cg_name(ns, svc)
    )
    ports = antrea_crd.NetworkPolicyPort(
        protocol = protocolIdentifier,
        port = int(port)
    )
    egress_rule = antrea_crd.Rule(
        action = "Allow",
        to = [egress_peer],
        ports = [ports]
    )
    return egress_rule

def generate_svc_acnp(x):
    applied_to, egresses = x
    ns, labels = applied_to.split(ROW_DELIMITER)
    if ns in NAMESPACE_ALLOW_LIST:
        return []
    try:
        labels_dict = json.loads(labels)
    except Exception as e:
        logger.error("Error {}: labels {} in applied_to {} are not in json format".format(e, labels, applied_to))
        return []
    egress_list = egresses.split(PEER_DELIMITER)
    egressRules = []
    for egress in egress_list:
        egressRules.append(generate_acnp_svc_egress_rule(egress))
    if egressRules:
        np_name = generate_policy_name("recommend-svc-allow-acnp")
        np = antrea_crd.ClusterNetworkPolicy(
            kind = "ClusterNetworkPolicy",
            api_version = "crd.antrea.io/v1alpha1",
            metadata = kubernetes.client.V1ObjectMeta(
                name = np_name,
            ),
            spec = antrea_crd.NetworkPolicySpec(
                tier = "Application",
                priority = DEFAULT_POLICY_PRIORITY,
                applied_to = [antrea_crd.NetworkPolicyPeer(
                    pod_selector = kubernetes.client.V1LabelSelector(
                        match_labels = labels_dict
                    ),
                    namespace_selector = kubernetes.client.V1LabelSelector(
                        match_labels = {
                            "kubernetes.io/metadata.name":ns
                        }
                    )
                )],
                egress = egressRules,
            )
        )
        return [dict_to_yaml(np.to_dict())]
    else:
        return []

def generate_reject_acnp(applied_to):
    if not applied_to:
        np_name = "recommend-reject-all-acnp"
        applied_to = antrea_crd.NetworkPolicyPeer(
            pod_selector = kubernetes.client.V1LabelSelector(),
            namespace_selector = kubernetes.client.V1LabelSelector()
        )
    else:
        np_name = generate_policy_name("recommend-reject-acnp")
        ns, labels = applied_to.split(ROW_DELIMITER)
        if ns in NAMESPACE_ALLOW_LIST:
            return []
        try:
            labels_dict = json.loads(labels)
        except Exception as e:
            logger.error("Error {}: labels {} in applied_to {} are not in json format".format(e, labels, applied_to))
            return []
        applied_to = antrea_crd.NetworkPolicyPeer(
            pod_selector = kubernetes.client.V1LabelSelector(
                match_labels = labels_dict
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
    return [dict_to_yaml(np.to_dict())]

def recommend_k8s_policies(flows_df):
    egress_rdd = flows_df.rdd.map(lambda flow:map_flow_to_egress(flow, k8s=True))\
        .reduceByKey(lambda a, b: ("", a[1]+PEER_DELIMITER+b[1]))
    ingress_rdd = flows_df.filter(flows_df.flowType != "pod_to_external")\
        .rdd.map(map_flow_to_ingress)\
        .reduceByKey(lambda a, b: (a[0]+PEER_DELIMITER+b[0], ""))
    network_peers_rdd = ingress_rdd.union(egress_rdd)\
                    .reduceByKey(combine_network_peers)
    k8s_np_rdd = network_peers_rdd.flatMap(generate_k8s_np)
    k8s_np_list = k8s_np_rdd.collect()
    return k8s_np_list

def recommend_antrea_policies(flows_df, option=1, deny_rules=True, to_services=True):
    ingress_rdd = flows_df.filter(flows_df.flowType != "pod_to_external")\
        .rdd.map(map_flow_to_ingress)\
        .reduceByKey(lambda a, b: (a[0]+PEER_DELIMITER+b[0], ""))
    if not to_services:
        # If toServices feature is not enabled, only recommend egress rules for 
        # unprotected Pod-to-Pod & Pod-to-External flows here
        unprotected_flows_df = flows_df.filter(flows_df.flowType != "pod_to_svc")
    else:
        unprotected_flows_df = flows_df
    egress_rdd = unprotected_flows_df.rdd.map(map_flow_to_egress)\
        .reduceByKey(lambda a, b: ("", a[1]+PEER_DELIMITER+b[1]))
    network_peers_rdd = ingress_rdd.union(egress_rdd)\
                    .reduceByKey(combine_network_peers)
    anp_rdd = network_peers_rdd.flatMap(generate_anp)
    anp_list = anp_rdd.collect()
    # If toServices feature is not enabled, recommend clusterGroup for Services and
    # allow Antrea Cluster Network Policies for unprotected Pod-to-Svc flows
    svc_cg_list = []
    svc_acnp_list = []
    if not to_services:
        unprotected_svc_flows_df = flows_df.filter(flows_df.flowType == "pod_to_svc")
        svc_df = unprotected_svc_flows_df.groupBy(["destinationServicePortName"]).agg({})
        svc_cg_list = svc_df.rdd.flatMap(generate_svc_cg).collect()
        egress_svc_rdd = unprotected_svc_flows_df.rdd.map(map_flow_to_egress_svc)\
            .reduceByKey(lambda a, b: a+PEER_DELIMITER+b)
        svc_acnp_rdd = egress_svc_rdd.flatMap(generate_svc_acnp)
        svc_acnp_list = svc_acnp_rdd.collect()
    if deny_rules:
        if option == 1:
            # Recommend deny ANPs for the appliedTo groups of allow policies
            if not to_services:
                applied_groups_rdd = network_peers_rdd.map(lambda x: x[0])\
                    .union(egress_svc_rdd.map(lambda x: x[0]))\
                    .distinct()
            else:
                applied_groups_rdd = network_peers_rdd.map(lambda x: x[0])
            deny_anp_rdd = applied_groups_rdd.flatMap(generate_reject_acnp)
            deny_anp_list = deny_anp_rdd.collect()
            return anp_list + svc_cg_list + svc_acnp_list + deny_anp_list
        else:
            # Recommend deny ACNP for whole cluster
            deny_all_policy = generate_reject_acnp("")
            return anp_list + svc_cg_list + svc_acnp_list + [deny_all_policy]
    else:
        return anp_list + svc_cg_list + svc_acnp_list

def recommend_policies_for_unprotected_flows(unprotected_flows_df, option=1, to_services=True):
    if option not in [1, 2, 3]:
        logger.error("Error: option {} is not valid".format(option))
        return []
    if option == 3:
        # Recommend K8s native NetworkPolicies for unprotected flows
        return recommend_k8s_policies(unprotected_flows_df)
    else:
        return recommend_antrea_policies(unprotected_flows_df, option, True, to_services)

def recommend_policies_for_trusted_denied_flows(trusted_denied_flows_df, to_services):
    return recommend_antrea_policies(trusted_denied_flows_df, deny_rules=False, to_services=to_services)

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

def generate_sql_query(table_name, limit, start_time, end_time, unprotected):
    sql_query = "SELECT {} FROM {}".format(", ".join(FLOW_TABLE_COLUMNS), table_name)
    if unprotected:
        sql_query += " WHERE ingressNetworkPolicyName == '' AND egressNetworkPolicyName == ''"
    else:
        # Select user trusted denied flows when unprotected equals False
        sql_query += " WHERE trusted == 1"
    if start_time:
        sql_query += " AND flowStartSeconds >= '{}'".format(start_time)
    if end_time:
        sql_query += " AND flowEndSeconds < '{}'".format(end_time)
    sql_query += " GROUP BY {}".format(", ".join(FLOW_TABLE_COLUMNS))
    if limit:
        sql_query += " LIMIT {}".format(limit)
    return sql_query

def read_flow_df(spark, db_jdbc_address, sql_query, rm_labels):
    flow_df = spark.read \
        .format("jdbc") \
        .option("driver", "ru.yandex.clickhouse.ClickHouseDriver") \
        .option("url", db_jdbc_address) \
        .option("user", os.getenv("CH_USERNAME")) \
        .option("password", os.getenv("CH_PASSWORD")) \
        .option("query",  sql_query)\
        .load()
    if rm_labels:
        flow_df = flow_df.withColumn('sourcePodLabels', udf(remove_meaningless_labels, StringType())("sourcePodLabels"))\
            .withColumn('destinationPodLabels', udf(remove_meaningless_labels, StringType())("destinationPodLabels"))\
            .dropDuplicates(['sourcePodLabels', 'destinationPodLabels'])
    flow_df = flow_df.withColumn('flowType', udf(get_flow_type, StringType())("destinationServicePortName", "destinationPodLabels"))
    return flow_df

def write_recommendation_result(spark, result, recommendation_type, db_jdbc_address, table_name, recommendation_id_input):
    if not recommendation_id_input:
        recommendation_id = str(uuid.uuid4())
    else:
        recommendation_id = recommendation_id_input
    result_dict = {
        'id': recommendation_id,
        'type': recommendation_type,
        'timeCreated': datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        'yamls': '---\n'.join(filter(None, result))
    }
    result_df = spark.createDataFrame([result_dict])
    result_df.write\
        .mode("append") \
        .format("jdbc") \
        .option("driver", "ru.yandex.clickhouse.ClickHouseDriver") \
        .option("url", db_jdbc_address) \
        .option("user", os.getenv("CH_USERNAME")) \
        .option("password", os.getenv("CH_PASSWORD")) \
        .option("dbtable", table_name) \
        .save()
    return recommendation_id

def initial_recommendation_job(spark, db_jdbc_address, table_name, limit=0, option=1, start_time=None, end_time=None, ns_allow_list=NAMESPACE_ALLOW_LIST, rm_labels=False, to_services=True):
    """
    Start an initial policy recommendation Spark job on a cluster having no recommendation before.

    Args:
        spark: Current SparkSession.
        db_jdbc_address: Database address to fetch the flow records.
        table_name: Name of the table storing flow records in database.
        limit: Limit on the number of flow records fetched in database. Default value is 100, setting to 0 means unlimited.
        option: Option of network isolation preference in policy recommendation. Currently we have 3 options and default value is 1:
            1: Recommending allow ANP/ACNP policies, with default deny rules only on appliedTo Pod labels which have allow rules recommended.
            2: Recommending allow ANP/ACNP policies, with default deny rules for whole cluster.
            3: Recommending allow K8s NetworkPolicies, with no deny rules at all.
        start_time: The start time of the flow records considered for the policy recommendation. Default value is None, which means no limit of the start time of flow records.
        end_time: The end time of the flow records considered for the policy recommendation. Default value is None, which means no limit of the end time of flow records.
        ns_allow_list: List of default traffic allow namespaces. Default value is Antrea CNI related namespaces.
        rm_labels: Remove automatically generated Pod labels including 'pod-template-hash', 'controller-revision-hash', 'pod-template-generation'.
        to_services: Use the toServices feature in ANP, only works when option is 1 or 2.

    Returns:
        A list of recommended policies, each recommended policy is a string of YAML format.
    """
    sql_query = generate_sql_query(table_name, limit, start_time, end_time, True)
    unprotected_flows_df = read_flow_df(spark, db_jdbc_address, sql_query, rm_labels)
    return recommend_policies_for_ns_allow_list(ns_allow_list) + recommend_policies_for_unprotected_flows(unprotected_flows_df, option, to_services)

def subsequent_recommendation_job(spark, db_jdbc_address, table_name, limit=0, option=1, start_time=None, end_time=None, rm_labels=False, to_services=True):
    """
    Start a subsequent policy recommendation Spark job on a cluster having recommendation before.

    Args:
        spark: Current SparkSession.
        db_jdbc_address: Database address to fetch the flow records.
        table_name: Name of the table storing flow records in database.
        limit: Limit on the number of flow records fetched in database. Default value is 100, setting to 0 means unlimited.
        option: Option of network isolation preference in policy recommendation. Currently we have 3 options and default value is 1:
            1: Recommending allow ANP/ACNP policies, with default deny rules only on appliedTo Pod labels which have allow rules recommended.
            2: Recommending allow ANP/ACNP policies, with default deny rules for whole cluster.
            3: Recommending allow K8s NetworkPolicies, with no deny rules at all.
        start_time: The start time of the flow records considered for the policy recommendation. Default value is None, which means no limit of the start time of flow records.
        end_time: The end time of the flow records considered for the policy recommendation. Default value is None, which means no limit of the end time of flow records.
        rm_labels: Remove automatically generated Pod labels including 'pod-template-hash', 'controller-revision-hash', 'pod-template-generation'.
        to_services: Use the toServices feature in ANP, only works when option is 1 or 2.

    Returns:
        A list of recommended policies, each recommended policy is a string of YAML format.
    """
    recommend_policies = []
    sql_query = generate_sql_query(table_name, limit, start_time, end_time, True)
    unprotected_flows_df = read_flow_df(spark, db_jdbc_address, sql_query, rm_labels)
    recommend_policies += recommend_policies_for_unprotected_flows(unprotected_flows_df, option, to_services)
    if option in [1, 2]:
        sql_query = generate_sql_query(table_name, limit, start_time, end_time, False)
        trusted_denied_flows_df = read_flow_df(spark, db_jdbc_address, sql_query, rm_labels)
        recommend_policies += recommend_policies_for_trusted_denied_flows(trusted_denied_flows_df, to_services)
    return recommend_policies

def main(argv):
    db_jdbc_address = "jdbc:clickhouse://clickhouse-clickhouse.flow-visibility.svc:8123"
    flow_table_name = "default.flows"
    result_table_name = "default.recommendations"
    recommendation_type = 'initial'
    limit = 0
    option = 1
    start_time = ""
    end_time = ""
    ns_allow_list = NAMESPACE_ALLOW_LIST
    recommendation_id_input = ""
    rm_labels = True
    to_services = True
    help_message = """
    Start the policy recommendation spark job.

    Options:
    -h, --help: Show help message.
    -t, --type=initial: {initial|subsequent} Indicates this recommendation is an initial recommendion or a subsequent recommendation job.
    -d, --db_jdbc_url=None: The JDBC URL used by Spark jobs connect to the ClickHouse database for reading flow records and writing result.
        jdbc:clickhouse://clickhouse-clickhouse.flow-visibility.svc:8123 is the ClickHouse JDBC URL used by default.
    -l, --limit=0: The limit on the number of flow records read from the database. 0 means no limit.
    -o, --option=1: Option of network isolation preference in policy recommendation.
        Currently we have 3 options:
        1: Recommending allow ANP/ACNP policies, with default deny rules only on appliedTo Pod labels which have allow rules recommended.
        2: Recommending allow ANP/ACNP policies, with default deny rules for whole cluster.
        3: Recommending allow K8s NetworkPolicies, with no deny rules at all.
    -s, --start_time=None: The start time of the flow records considered for the policy recommendation. 
        Format is YYYY-MM-DD hh:mm:ss in UTC timezone. Default value is None, which means no limit of the start time of flow records.
    -e, --end_time=None: The end time of the flow records considered for the policy recommendation.
        Format is YYYY-MM-DD hh:mm:ss in UTC timezone. Default value is None, which means no limit of the end time of flow records.
    -n, --ns_allow_list=[]: List of default traffic allow namespaces.
        Default value is a list of Antrea CNI related namespaces: ['kube-system', 'flow-aggregator', 'flow-visibility'].
    -i, --id=None: Recommendation job ID in UUID format. If not specified, it will be generated automatically.
    --rm_labels=None: Remove automatically generated Pod labels including 'pod-template-hash', 'controller-revision-hash', 'pod-template-generation'.
        This feature is enabled by default, provide false to disable this feature.
    --to_services=None: Use the toServices feature in ANP and recommendation toServices rules for Pod-to-Service flows, only works when option is 1 or 2.
        This feature is enabled by default, provide false to disable this feature.
    
    Usage Example:
    python3 policy_recommendation_job.py -t initial -l 1000 -o 1 -s '2021-01-01 00:00:00' -e '2021-12-31 00:00:00' -n '["kube-system","flow-aggregator","flow-visibility"]'
    """

    # TODO: change to use argparse instead of getopt for options
    try:
        opts, _ = getopt.getopt(argv, "ht:d:l:o:s:e:n:i:", ["help", "type=", "db_jdbc_url=", "limit=", "option=", "start_time=", "end_time=", "ns_allow_list=", "id=", "rm_labels=", "to_services="])
    except getopt.GetoptError as e:
        logger.error("ERROR of getopt.getopt: {}".format(e))
        logger.info(help_message)
        sys.exit(2)
    option_keys = [option[0] for option in opts]
    if "-h" in option_keys or "--help" in option_keys:
        logger.info(help_message)
        sys.exit()
    for opt, arg in opts:
        if opt in ("-t", "--type"):
            valid_types = ["initial", "subsequent"]
            if arg not in valid_types:
                logger.error("Recommendation type should be in {}".format("or".join(valid_types)))
                logger.info(help_message)
                sys.exit(2)
            recommendation_type = arg
        elif opt in ("-d", "--db_jdbc_url"):
            parse_url = urlparse("arg")
            if parse_url.scheme != "jdbc":
                logger.error("Please provide a valid JDBC url for ClickHouse database")
                logger.info(help_message)
                sys.exit(2)
            db_jdbc_address = arg
        elif opt in ("-l", "--limit"):
            if not is_intstring(arg) or int(arg) < 0:
                logger.error("Limit should be an integer >= 0.")
                logger.info(help_message)
                sys.exit(2)
            limit = int(arg)
        elif opt in ("-o", "--option"):
            if not is_intstring(arg) or int(arg) not in [1, 2, 3]:
                logger.error("Option of network isolation preference should be 1 or 2 or 3.")
                logger.info(help_message)
                sys.exit(2)
            option = int(arg)
        elif opt in ("-s", "--start_time"):
            try:
                datetime.datetime.strptime(arg, "%Y-%m-%d %H:%M:%S")
            except ValueError:
                logger.error("start_time should be in 'YYYY-MM-DD hh:mm:ss' format.")
                logger.info(help_message)
                sys.exit(2)
            start_time = arg
        elif opt in ("-e", "--end_time"):
            try:
                datetime.datetime.strptime(arg, "%Y-%m-%d %H:%M:%S")
            except ValueError:
                logger.error("end_time should be in 'YYYY-MM-DD hh:mm:ss' format.")
                logger.info(help_message)
                sys.exit(2)
            end_time = arg
        elif opt in ("-n", "--ns_allow_list"):
            arg_list = json.loads(arg)
            if not isinstance(arg_list, list):
                logger.error("ns_allow_list should be a list.")
                logger.info(help_message)
                sys.exit(2)
            ns_allow_list = arg_list
        elif opt in ("-i", "--id"):
            recommendation_id_input = arg
        elif opt in ("--rm_labels"):
            if arg == "false":
                rm_labels = False
        elif opt in ("--to_services"):
            if arg == "false":
                to_services = False

    spark = SparkSession.builder.getOrCreate()
    if recommendation_type == 'initial':
        result = initial_recommendation_job(spark, db_jdbc_address, flow_table_name, limit, option, start_time, end_time, ns_allow_list, rm_labels, to_services)
        recommendation_id = write_recommendation_result(spark, result, 'initial', db_jdbc_address, result_table_name, recommendation_id_input)
        logger.info("Initial policy recommendation completed, id: {}, policy number: {}".format(recommendation_id, len(result)))
    else:
        result = subsequent_recommendation_job(spark, db_jdbc_address, flow_table_name, limit, option, start_time, end_time, rm_labels, to_services)
        recommendation_id = write_recommendation_result(spark, result, 'subsequent', db_jdbc_address, result_table_name, recommendation_id_input)
        logger.info("Subsequent policy recommendation completed, id: {}, policy number: {}".format(recommendation_id, len(result)))
    spark.stop()

if __name__ == '__main__':
    main(sys.argv[1:])
