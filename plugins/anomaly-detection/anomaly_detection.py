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
import getopt
import json
import logging
import os
import sys
import uuid

import time
import pyspark.sql.functions as f
import numpy as np
import warnings

from datetime import datetime
from pyspark.sql import SparkSession
from pyspark.sql.types import (
    BooleanType, ArrayType, StructField, DecimalType, DoubleType,
    StringType, LongType, TimestampType, StructType)
from scipy import stats
from scipy.special import inv_boxcox
from statsmodels.tsa.arima.model import ARIMA
from sklearn.cluster import DBSCAN
from urllib.parse import urlparse

logger = logging.getLogger('anomaly_detection')
logger.setLevel(logging.INFO)
ch = logging.StreamHandler()
ch.setLevel(logging.INFO)
formatter = logging.Formatter(
    '%(asctime)s - %(name)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
logger.addHandler(ch)

table_name = "default.flows"

# Column names of flow record table in Clickhouse database used in anomaly
# detection job
FLOW_TABLE_COLUMNS = [
    'sourceIP',
    'sourceTransportPort',
    'destinationIP',
    'destinationTransportPort',
    'protocolIdentifier',
    'flowStartSeconds',
    'flowEndSeconds',
    'max(throughput)'
]

AGG_FLOW_TABLE_COLUMNS_EXTERNAL = [
    'destinationIP',
    'flowType',
    'flowEndSeconds',
    'sum(throughput)'
]

AGG_FLOW_TABLE_COLUMNS_POD_INBOUND = [
    "destinationPodNamespace AS podNamespace",
    "destinationPodLabels AS podLabels",
    "'inbound' AS direction",
    "flowEndSeconds",
    "sum(throughput)"
]

AGG_FLOW_TABLE_COLUMNS_POD_OUTBOUND = [
    "sourcePodNamespace AS podNamespace",
    "sourcePodLabels AS podLabels",
    "'outbound' AS direction",
    "flowEndSeconds",
    "sum(throughput)"
]

AGG_FLOW_TABLE_COLUMNS_PODNAME_INBOUND = [
    "destinationPodNamespace AS podNamespace",
    "destinationPodName AS podName",
    "'inbound' AS direction",
    "flowEndSeconds",
    "sum(throughput)"
]

AGG_FLOW_TABLE_COLUMNS_PODNAME_OUTBOUND = [
    "sourcePodNamespace AS podNamespace",
    "sourcePodName AS podName",
    "'outbound' AS direction",
    "flowEndSeconds",
    "sum(throughput)"
]

AGG_FLOW_TABLE_COLUMNS_SVC = [
    'destinationServicePortName',
    'flowEndSeconds',
    'sum(throughput)'
]

# Column names to be used to group and identify a connection uniquely
DF_GROUP_COLUMNS = [
    'sourceIP',
    'sourceTransportPort',
    'destinationIP',
    'destinationTransportPort',
    'protocolIdentifier',
    'flowStartSeconds'
]

DF_AGG_GRP_COLUMNS_POD = [
    'podNamespace',
    'podLabels',
    'direction',
]

DF_AGG_GRP_COLUMNS_PODNAME = [
    'podNamespace',
    'podName',
    'direction',
]

DF_AGG_GRP_COLUMNS_EXTERNAL = [
    'destinationIP',
    'flowType',
]

DF_AGG_GRP_COLUMNS_SVC = [
    'destinationServicePortName',
]

MEANINGLESS_LABELS = [
    "pod-template-hash",
    "controller-revision-hash",
    "pod-template-generation",
]


def calculate_ewma(throughput_list):
    """
    The function calculates Exponential Weighted Moving Average (EWMA) for
    a given list of throughput values of a connection
    Args:
        throughput_list: Column of a dataframe containing throughput
    Returns:
        A list of EWMA values calculated for the set of throughput values
        for that specific connection.
    """

    alpha = 0.5  # Can be changed and passed as UDF value later.
    prev_ewma_val = 0.0
    ewma_row = []
    for ele in throughput_list:
        ele_float = float(ele)
        curr_ewma_val = (1 - alpha) * prev_ewma_val + alpha * ele_float
        prev_ewma_val = curr_ewma_val
        ewma_row.append(float(curr_ewma_val))
    return ewma_row


def calculate_ewma_anomaly(throughput_row, stddev):
    """
    The function categorizes whether a network flow is Anomalous or not
    based on the calculated EWMA value.
    A network flow record is anomalous if abs(throughput - ewma) > Standard
    Deviation.
    True - Anomalous Traffic, False - Not Anomalous

    Args:
        throughput_row : The row of a dataframe containing all throughput data.
        stddev : The row of a dataframe containing standard Deviation
    Returns:
        A list of boolean values which signifies if a network flow record
        of that connection is anomalous or not.
    """
    ewma_arr = calculate_ewma(throughput_row)
    anomaly_result = []

    if ewma_arr is None:
        logger.error("Error: EWMA values not calculated for this flow record")
        result = False
        anomaly_result.append(result)
    elif throughput_row is None:
        logger.error("Error: Throughput values not in ideal format for this "
                     "flow record")
        result = False
        anomaly_result.append(result)
    else:
        num_records = len(ewma_arr)
        for i in range(num_records):
            if stddev is None:
                logger.error("Error: Too Few Throughput Values for Standard "
                             "Deviation to be calculated.")
                result = False
            elif throughput_row[i] is None:
                logger.error("Error: Throughput values not in ideal format "
                             "for this flow record")
                result = False
            else:
                float(stddev)
                result = True if (abs(float(throughput_row[i]) - float(
                    ewma_arr[i])) >
                                  float(stddev)) else False
            anomaly_result.append(result)
    return anomaly_result


def calculate_arima(throughputs):
    """
    The function calculates AutoRegressive Integrated Moving Average
    (ARIMA) for a given list of throughput values of a connection
    Assumption: Since ARIMA needs a handful of data to train and start
    prediction, any connection with less than 3 flow records will not be
    taken into account for calculation. We return empty value in that case.
    Args:
        throughputs: Column of a dataframe containing throughput
    Returns:
        A list of ARIMA values calculated for the set of throughput values
        for that specific connection.
    """

    throughput_list = []
    for ele in throughputs:
        throughput_list.append(float(ele))
    if len(throughput_list) <= 3:
        logger.error("Error: Too Few throughput values for ARIMA to work with")
        return None
    else:
        try:
            warnings.filterwarnings("ignore")
            throughput_list = [x for x in throughput_list]
            throughput_list_bxcx, revvar = stats.boxcox(throughput_list)
            throughput_list_bxcx = throughput_list_bxcx.tolist()
            train, test = throughput_list_bxcx[0:3], throughput_list_bxcx[3:]
            if len(test) == 1:
                test = [test]
            history = [x for x in train]
            predictions = list()
            for t in range(len(test)):
                model = ARIMA(history, order=(1, 1, 1))
                model_fit = model.fit()
                output = model_fit.forecast()
                yhat = output[0]
                predictions.append(yhat)
                obs = test[t]
                history.append(obs)

            predictions_final = train + predictions
            predictions_final = inv_boxcox(predictions_final, revvar)
            predictions_final = predictions_final.tolist()
            predictions_final = [float(x) for x in predictions_final]
            return predictions_final
        except Exception as e:
            logger.critical(
                "Error: ARIMA encountered error with the current flow "
                "record. err_msg: {}".format(e))
            return None


def calculate_arima_anomaly(throughput_row, stddev):
    """
    The function categorizes whether a network flow is Anomalous or not based
    on the calculated ARIMA value. A traffic is anomalous if abs(throughput
    - arima) > Standard Deviation. True - Anomalous Traffic, False - Not
    Anomalous

    Args:
        Throughput_list : The row of a dataframe containing all throughput
        data.
        stddev : The row of a dataframe containing standard Deviation
    Returns:
        A list of boolean values which signifies if a network flow record
        of that connection is anomalous or not.
    """
    arima_arr = calculate_arima(throughput_row)
    anomaly_result = []

    if arima_arr is None:
        logger.error("Error: ARIMA values not calculated for this flow record")
        result = False
        anomaly_result.append(result)
    elif throughput_row is None:
        logger.error("Error: Throughput values not in ideal format for this "
                     "flow record")
        result = False
        anomaly_result.append(result)
    else:
        num_records = len(arima_arr)
        for i in range(num_records):
            if stddev is None:
                logger.error("Error: Too Few Throughput Values for Standard "
                             "Deviation to be calculated.")
                result = False
            elif throughput_row[i] is None:
                logger.error("Error: Throughput values not in ideal format "
                             "for this flow record")
                result = False
            else:
                result = True if (abs(float(throughput_row[i]) - float(
                    arima_arr[i])) > float(stddev)) else False
            anomaly_result.append(result)
    return anomaly_result


def calculate_dbscan(throughput_list):
    """
    The function is a placeholder function as anomaly detection with
    DBSCAN only inputs the throughput values. However, in order to maintain
    the tadetector table in click house, a placeholder column is required
    """
    # Currently just a placeholder function
    placeholder_throughput_list = []
    for i in range(len(throughput_list)):
        placeholder_throughput_list.append(0.0)
    return placeholder_throughput_list


def calculate_dbscan_anomaly(throughput_row, stddev=None):
    """
    The function calculates Density-based spatial clustering of applications
    with Noise (DBSCAN) for a given list of throughput values of a connection
    Args:
        throughput_row : The row of a dataframe containing all throughput data.
        stddev : The row of a dataframe containing standard Deviation
        Assumption: Since DBSCAN needs only numeric value to train and start
        prediction, any connection with null values will not be taken
        into account for calculation. We return empty value in that case.
    Returns:
        A list of boolean values which signifies if a network flow records
        of the connection is anomalous or not based on DBSCAN
    """
    anomaly_result = []
    np_throughput_list = np.array(throughput_row)
    np_throughput_list = np_throughput_list.reshape(-1, 1)
    outlier_detection = DBSCAN(min_samples=4, eps=250000000)
    clusters = outlier_detection.fit_predict(np_throughput_list)
    for i in clusters:
        if i == -1:
            anomaly_result.append(True)
        else:
            anomaly_result.append(False)
    return anomaly_result


def filter_df_with_true_anomalies(
        spark, plotDF, algo_type, agg_flow=None, pod_label=None):
    newDF = plotDF.withColumn(
        "new", f.arrays_zip(
            "flowEndSeconds", "algoCalc", "throughputs",
            "anomaly")).withColumn(
        "new", f.explode("new"))
    if agg_flow == "pod":
        plotDF = newDF.select(
            "podNamespace", "podLabels" if pod_label else "podName",
            "direction", "aggType",
            f.col("new.flowEndSeconds").alias("flowEndSeconds"),
            "throughputStandardDeviation", "algoType",
            f.col("new.algoCalc").alias("algoCalc"),
            f.col("new.throughputs").alias("throughput"),
            f.col("new.anomaly").alias("anomaly"))
    elif agg_flow == "external":
        plotDF = newDF.select(
            "destinationIP", "aggType",
            f.col("new.flowEndSeconds").alias("flowEndSeconds"),
            "throughputStandardDeviation", "algoType",
            f.col("new.algoCalc").alias("algoCalc"),
            f.col("new.throughputs").alias("throughput"),
            f.col("new.anomaly").alias("anomaly"))
    elif agg_flow == "svc":
        plotDF = newDF.select(
            "destinationServicePortName", "aggType",
            f.col("new.flowEndSeconds").alias("flowEndSeconds"),
            "throughputStandardDeviation", "algoType",
            f.col("new.algoCalc").alias("algoCalc"),
            f.col("new.throughputs").alias("throughput"),
            f.col("new.anomaly").alias("anomaly"))
    else:
        plotDF = newDF.select(
            "sourceIP", "sourceTransportPort", "destinationIP",
            "destinationTransportPort",
            "protocolIdentifier", "flowStartSeconds", "aggType",
            f.col("new.flowEndSeconds").alias("flowEndSeconds"),
            "throughputStandardDeviation", "algoType",
            f.col("new.algoCalc").alias("algoCalc"),
            f.col("new.throughputs").alias("throughput"),
            f.col("new.anomaly").alias("anomaly"))
    ret_plot = plotDF.where(~plotDF.anomaly.isin([False]))
    if ret_plot.count() == 0:
        ret_plot = ret_plot.collect()
        if agg_flow == "":
            agg_type = "None"
        else:
            agg_type = agg_flow
        ret_plot.append({
            "sourceIP": 'None',
            "sourceTransportPort": 0,
            "destinationIP": 'None',
            "destinationTransportPort": 0,
            "protocolIdentifier": 0,
            "flowStartSeconds": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "podNamespace": 'None',
            "podLabels": 'None',
            "podName": 'None',
            "destinationServicePortName": 'None',
            "direction": 'None',
            "flowEndSeconds": 0,
            "throughputStandardDeviation": 0,
            "aggType": agg_type,
            "algoType": algo_type,
            "algoCalc": 0.0,
            "throughput": 0.0,
            "anomaly": "NO ANOMALY DETECTED"})
        ret_plot = spark.createDataFrame(ret_plot)
    return ret_plot


def plot_anomaly(spark, init_plot_df, algo_type, algo_func, anomaly_func,
                 tad_id_input, agg_flow=None, pod_label=None):
    # Insert the Algo currently in use
    init_plot_df = init_plot_df.withColumn('algoType', f.lit(algo_type))
    # Schema List
    schema_list = [
        StructField('sourceIP', StringType(), True),
        StructField('sourceTransportPort', LongType(), True),
        StructField('destinationIP', StringType(), True),
        StructField('destinationTransportPort', LongType(), True),
        StructField('protocolIdentifier', LongType(), True),
        StructField('flowStartSeconds', TimestampType(), True),
        StructField('flowEndSeconds', ArrayType(TimestampType(), True)),
        StructField('throughputStandardDeviation', DoubleType(), True)
    ]
    # Calculate anomaly Values on the DF
    algo_func_rdd = init_plot_df.rdd.map(
        lambda x: (x[0], x[1], x[2], x[3], x[4], x[5],
                   x[6], x[7], x[8], x[9], x[10], algo_func(x[8]),
                   anomaly_func(x[8], x[7])))
    if agg_flow == "pod":
        if pod_label:
            schema_list = [
                StructField('podNamespace', StringType(), True),
                StructField('podLabels', StringType(), True),
                StructField('direction', StringType(), True),
                StructField('flowEndSeconds', ArrayType(TimestampType(),
                                                        True)),
                StructField('throughputStandardDeviation', DoubleType(), True)
            ]
        else:
            schema_list = [
                StructField('podNamespace', StringType(), True),
                StructField('podName', StringType(), True),
                StructField('direction', StringType(), True),
                StructField('flowEndSeconds', ArrayType(TimestampType(),
                                                        True)),
                StructField('throughputStandardDeviation', DoubleType(), True)
            ]
        algo_func_rdd = init_plot_df.rdd.map(
            lambda x: (x[0], x[1], x[2], x[3], x[4], x[5],
                       x[6], x[7], algo_func(x[5]),
                       anomaly_func(x[5], x[4])))
    elif agg_flow == "svc":
        schema_list = [
            StructField('destinationServicePortName', StringType(), True),
            StructField('flowEndSeconds', ArrayType(TimestampType(), True)),
            StructField('throughputStandardDeviation', DoubleType(), True)
        ]
        algo_func_rdd = init_plot_df.rdd.map(
            lambda x: (x[0], x[1], x[2], x[3], x[4], x[5], algo_func(x[3]),
                       anomaly_func(x[3], x[2])))
    elif agg_flow == "external":
        schema_list = [
            StructField('destinationIP', StringType(), True),
            StructField('flowEndSeconds', ArrayType(TimestampType(), True)),
            StructField('throughputStandardDeviation', DoubleType(), True)
        ]
        algo_func_rdd = init_plot_df.rdd.map(
            lambda x: (x[0], x[1], x[2], x[3], x[4], x[5], algo_func(x[3]),
                       anomaly_func(x[3], x[2])))

    # Schema for the Dataframe to be created from the RDD
    algo_func_rdd_Schema = StructType(schema_list + [
        StructField('throughputs', ArrayType(DecimalType(38, 18), True)),
        StructField('aggType', StringType(), True),
        StructField('algoType', StringType(), True),
        StructField('algoCalc', ArrayType(DoubleType(), True)),
        StructField('anomaly', ArrayType(BooleanType(), True))
    ])

    anomalyDF = spark.createDataFrame(algo_func_rdd, algo_func_rdd_Schema)
    ret_plotDF = filter_df_with_true_anomalies(spark, anomalyDF, algo_type,
                                               agg_flow, pod_label)
    # Write anomalous records to DB/CSV - Module WIP
    # Module to write to CSV. Optional.
    ret_plotDF = ret_plotDF.withColumn(
        "anomaly", f.col("anomaly").cast(
            "string"))
    ret_plotDF = ret_plotDF.withColumn('id', f.lit(str(tad_id_input)))
    return ret_plotDF


def generate_tad_sql_query(start_time, end_time, ns_ignore_list,
                           agg_flow=None, pod_label=None, external_ip=None,
                           svc_port_name=None, pod_name=None,
                           pod_namespace=None):
    if agg_flow == "pod":
        agg_flow_table_columns_pod_inbound = (
            AGG_FLOW_TABLE_COLUMNS_POD_INBOUND)
        agg_flow_table_columns_pod_outbound = (
            AGG_FLOW_TABLE_COLUMNS_POD_OUTBOUND)
        df_agg_grp_columns_pod = DF_AGG_GRP_COLUMNS_POD
        if pod_label:
            inbound_condition = (
                "ilike(destinationPodLabels, '%{}%') ".format(pod_label))
            outbound_condition = (
                "ilike(sourcePodLabels, '%{}%')".format(pod_label))
            if pod_namespace:
                inbound_condition += (
                    " AND destinationPodNamespace = '{}'".format(
                        pod_namespace))
                outbound_condition += (
                    " AND sourcePodNamespace = '{}'".format(pod_namespace))
        elif pod_name:
            inbound_condition = (
                "destinationPodName = '{}'".format(pod_name))
            outbound_condition = (
                "sourcePodName = '{}'".format(pod_name))
            if pod_namespace:
                inbound_condition += (
                    " AND destinationPodNamespace = '{}'".format(
                        pod_namespace))
                outbound_condition += (
                    " AND sourcePodNamespace = '{}'".format(pod_namespace))
            agg_flow_table_columns_pod_inbound = (
                AGG_FLOW_TABLE_COLUMNS_PODNAME_INBOUND)
            agg_flow_table_columns_pod_outbound = (
                AGG_FLOW_TABLE_COLUMNS_PODNAME_OUTBOUND)
            df_agg_grp_columns_pod = DF_AGG_GRP_COLUMNS_PODNAME
        else:
            inbound_condition = (
                "destinationPodLabels <> '' ")
            outbound_condition = (
                "sourcePodLabels <> ''")
        if ns_ignore_list:
            sql_query_extension = (
                "AND sourcePodNamespace NOT IN ({0}) AND "
                "destinationPodNamespace NOT IN ({0})".format(
                    ", ".join("'{}'".format(x) for x in ns_ignore_list)))
        else:
            sql_query_extension = ""
        sql_query = (
            "SELECT * FROM "
            "(SELECT {0} FROM {1} WHERE {2} {6} GROUP BY {3}) "
            "UNION ALL "
            "(SELECT {4} FROM {1} WHERE {5} {6} GROUP BY {3}) ".format(
                ", ".join(agg_flow_table_columns_pod_inbound),
                table_name, inbound_condition, ", ".join(
                    df_agg_grp_columns_pod + ['flowEndSeconds']),
                ", ".join(agg_flow_table_columns_pod_outbound),
                outbound_condition, sql_query_extension))
    else:
        common_flow_table_columns = FLOW_TABLE_COLUMNS
        if agg_flow == "external":
            common_flow_table_columns = AGG_FLOW_TABLE_COLUMNS_EXTERNAL
        elif agg_flow == "svc":
            common_flow_table_columns = AGG_FLOW_TABLE_COLUMNS_SVC

        sql_query = ("SELECT {} FROM {} ".format(
            ", ".join(common_flow_table_columns), table_name))
        sql_query_extension = []
        if ns_ignore_list:
            sql_query_extension.append(
                "sourcePodNamespace NOT IN ({0}) AND "
                "destinationPodNamespace NOT IN ({0})".format(
                    ", ".join("'{}'".format(x) for x in ns_ignore_list)))
        if start_time:
            sql_query_extension.append(
                "flowStartSeconds >= '{}'".format(start_time))
        if end_time:
            sql_query_extension.append(
                "flowEndSeconds < '{}'".format(end_time))
        if agg_flow:
            if agg_flow == "external":
                # TODO agg=destination IP, change the name to external
                sql_query_extension.append("flowType = 3")
                if external_ip:
                    sql_query_extension.append("destinationIP = '{}'".format(
                        external_ip))
            elif agg_flow == "svc":
                if svc_port_name:
                    sql_query_extension.append(
                        "destinationServicePortName = '{}'".format(
                            svc_port_name))
                else:
                    sql_query_extension.append(
                        "destinationServicePortName <> ''")

        if sql_query_extension:
            sql_query += "WHERE " + " AND ".join(sql_query_extension) + " "
        df_group_columns = DF_GROUP_COLUMNS
        if agg_flow == "external":
            df_group_columns = DF_AGG_GRP_COLUMNS_EXTERNAL
        elif agg_flow == "svc":
            df_group_columns = DF_AGG_GRP_COLUMNS_SVC

        sql_query += "GROUP BY {} ".format(
            ", ".join(df_group_columns + [
                "flowEndSeconds"]))
    return sql_query


def assign_flow_type(prepared_DF, agg_flow=None, direction=None):
    if agg_flow == "external":
        prepared_DF = prepared_DF.withColumn('aggType', f.lit(
            "external"))
        prepared_DF = prepared_DF.drop('flowType')
    elif agg_flow == "svc":
        prepared_DF = prepared_DF.withColumn('aggType', f.lit("svc"))
    elif agg_flow == "pod":
        prepared_DF = prepared_DF.withColumn('aggType', f.lit("pod"))
    else:
        prepared_DF = prepared_DF.withColumn('aggType', f.lit("None"))
    return prepared_DF


def remove_meaningless_labels(podLabels):
    try:
        labels_dict = json.loads(podLabels)
    except Exception as e:
        logger.error(
            "Error {}: labels {} are not in json format".format(e, podLabels)
        )
        return ""
    labels_dict = {
        key: value
        for key, value in labels_dict.items()
        if key not in MEANINGLESS_LABELS
    }
    return json.dumps(labels_dict, sort_keys=True)


def anomaly_detection(algo_type, db_jdbc_address, start_time, end_time,
                      tad_id_input, ns_ignore_list, agg_flow=None,
                      pod_label=None, external_ip=None, svc_port_name=None,
                      pod_name=None, pod_namespace=None):
    spark = SparkSession.builder.getOrCreate()
    sql_query = generate_tad_sql_query(
        start_time, end_time, ns_ignore_list, agg_flow, pod_label,
        external_ip, svc_port_name, pod_name, pod_namespace)
    initDF = (
        spark.read.format("jdbc").option(
            'driver', "ru.yandex.clickhouse.ClickHouseDriver").option(
            "url", db_jdbc_address).option(
            "user", os.getenv("CH_USERNAME")).option(
            "password", os.getenv("CH_PASSWORD")).option(
            "query", sql_query).load()
    )

    if agg_flow:
        if agg_flow == "pod":
            if pod_name:
                df_agg_grp_columns = DF_AGG_GRP_COLUMNS_PODNAME
            else:
                df_agg_grp_columns = DF_AGG_GRP_COLUMNS_POD
        elif agg_flow == "external":
            df_agg_grp_columns = DF_AGG_GRP_COLUMNS_EXTERNAL
        elif agg_flow == "svc":
            df_agg_grp_columns = DF_AGG_GRP_COLUMNS_SVC
        prepared_DF = initDF.groupby(df_agg_grp_columns).agg(
            f.collect_list("flowEndSeconds").alias("flowEndSeconds"),
            f.stddev_samp("sum(throughput)").alias(
                "throughputStandardDeviation"),
            f.collect_list("sum(throughput)").alias("throughputs"))
    else:
        prepared_DF = initDF.groupby(DF_GROUP_COLUMNS).agg(
            f.collect_list("flowEndSeconds").alias("flowEndSeconds"),
            f.stddev_samp("max(throughput)").alias(
                "throughputStandardDeviation"),
            f.collect_list("max(throughput)").alias("throughputs"))

    prepared_DF = assign_flow_type(prepared_DF, agg_flow)
    if agg_flow == "pod" and not pod_name:
        prepared_DF = (
            prepared_DF.withColumn(
                "PodLabels",
                f.udf(remove_meaningless_labels, StringType())(
                    "PodLabels"
                ),
            )
        )

    if algo_type == "EWMA":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type, calculate_ewma,
                                calculate_ewma_anomaly, tad_id_input, agg_flow,
                                pod_label)
    elif algo_type == "ARIMA":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type, calculate_arima,
                                calculate_arima_anomaly, tad_id_input,
                                agg_flow, pod_label)
    elif algo_type == "DBSCAN":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type,
                                calculate_dbscan,
                                calculate_dbscan_anomaly, tad_id_input,
                                agg_flow, pod_label)
    return spark, ret_plot


def write_anomaly_detection_result(
        result_df, db_jdbc_address, result_table_name, tad_id_input):
    if not tad_id_input:
        tad_id = str(uuid.uuid4())
    else:
        tad_id = tad_id_input

    result_df.write.mode("append").format("jdbc").option(
        "driver", "ru.yandex.clickhouse.ClickHouseDriver").option(
        "url", db_jdbc_address).option(
        "user", os.getenv("CH_USERNAME")).option(
        "password", os.getenv("CH_PASSWORD")).option(
        "dbtable", result_table_name).save()
    return tad_id


def main():
    db_jdbc_address = (
        "jdbc:clickhouse://clickhouse-clickhouse.flow-visibility.svc:8123")
    result_table_name = "default.tadetector"
    algo_type = ""
    start_time = ""
    end_time = ""
    tad_id_input = None
    ns_ignore_list = []
    agg_flow = ""
    pod_label = ""
    external_ip = ""
    svc_port_name = ""
    pod_name = ""
    pod_namespace = ""
    help_message = """
    Start the Throughput Anomaly Detection spark job.
        Options:
        -h, --help: Show help message.
        -a, --algo=EWMA: Type argument describes the anomaly detection Algo
            to use. Currently Supported Algos are EWMA, ARIMA and DBSCAN
        -d, --db_jdbc_url=None: The JDBC URL used by Spark jobs connect to
            the ClickHouse database for reading flow records and writing
            result. jdbc:clickhouse://clickhouse-clickhouse.flow-visibility
            .svc:8123 is the ClickHouse JDBC URL used by default.
        -s, --start_time=None: The start time of the flow records
            considered for the Throughput Anomaly Detection. Format is
            YYYY-MM-DD hh:mm:ss in UTC timezone. Default value is None,
            which means no limit of the start time of flow records.
        -e, --end_time=None: The end time of the flow records considered
            for the Throughput Anomaly Detection.
            Format is YYYY-MM-DD hh:mm:ss in UTC timezone.
            Default value is None, which means no limit of the end time
            of flow records.
        -i, --id=None: Throughput Anomaly Detection job ID in UUID format.
            If not specified, it will be generated automatically.
        -n, --ns-ignore-list=[]: List of namespaces to ignore in anomaly
            calculation.
        -f, --agg-flow=None: Aggregated Flow Throughput Anomaly Detection.
        -l, --pod-label=None: Aggregated Flow Throughput Anomaly Detection
            to/from Pod using pod labels
        -N, --pod-name=None: Aggregated Flow Throughput Anomaly Detection
            to/from Pod using pod Name
        -P, --pod-namespace=None: Aggregated Flow Throughput Anomaly Detection
            to/from Pod using pod namespace
        -x, --external-ip=None: Aggregated Flow Throughput Anomaly Detection
            to Destination IP
        -p, --svc-port-name=None: Aggregated Flow Throughput Anomaly Detection
            to Destination Service Port
        """

    # TODO: change to use argparse instead of getopt for options
    try:
        opts, _ = getopt.getopt(
            sys.argv[1:],
            "ht:d:s:e:i:n:f:l:d:x:p:N:P",
            [
                "help",
                "algo=",
                "db_jdbc_url=",
                "start_time=",
                "end_time=",
                "id=",
                "ns_ignore_list=",
                "agg-flow=",
                "pod-label=",
                "external-ip=",
                "svc-port-name=",
                "pod-name=",
                "pod-namespace=",
            ],
        )
    except getopt.GetoptError as e:
        logger.error("ERROR of getopt.getopt: {}".format(e))
        logger.info(help_message)
        sys.exit(2)
    option_keys = [option[0] for option in opts]
    if "-h" in option_keys or "--help" in option_keys:
        logger.info(help_message)
        sys.exit()
    for opt, arg in opts:
        if opt in ("-a", "--algo"):
            valid_algos = ['EWMA', 'ARIMA', 'DBSCAN']
            if arg not in valid_algos:
                logger.error(
                    "Algorithm should be in {}".format(
                        " or ".join(valid_algos)
                    )
                )
                logger.info(help_message)
                sys.exit(2)
            algo_type = arg
        elif opt in ("-d", "--db_jdbc_url"):
            parse_url = urlparse("arg")
            if parse_url.scheme != "jdbc":
                logger.error(
                    "Please provide a valid JDBC url for ClickHouse database"
                )
                logger.info(help_message)
                sys.exit(2)
            db_jdbc_address = arg
        elif opt in ("-s", "--start_time"):
            try:
                datetime.strptime(arg, "%Y-%m-%d %H:%M:%S")
            except ValueError:
                logger.error(
                    "start_time should be in 'YYYY-MM-DD hh:mm:ss' format."
                )
                logger.info(help_message)
                sys.exit(2)
            start_time = arg
        elif opt in ("-e", "--end_time"):
            try:
                datetime.strptime(arg, "%Y-%m-%d %H:%M:%S")
            except ValueError:
                logger.error(
                    "end_time should be in 'YYYY-MM-DD hh:mm:ss' format."
                )
                logger.info(help_message)
                sys.exit(2)
            end_time = arg
        elif opt in ("-n", "--ns_ignore_list"):
            arg_list = json.loads(arg)
            if not isinstance(arg_list, list):
                logger.error("ns_ignore_list should be a list.")
                logger.info(help_message)
                sys.exit(2)
            ns_ignore_list = arg_list
        elif opt in ("-i", "--id"):
            tad_id_input = arg
        elif opt in ("-f", "--agg-flow"):
            agg_flow = arg
        elif opt in ("-l", "--pod-label"):
            pod_label = arg
        elif opt in ("-N", "--pod-name"):
            pod_name = arg
        elif opt in ("-P", "--pod-namespace"):
            pod_namespace = arg
        elif opt in ("-x", "--external-ip"):
            external_ip = arg
        elif opt in ("-", "--svc-port-name"):
            svc_port_name = arg

    func_start_time = time.time()
    logger.info("Script started at {}".format(
        datetime.now().strftime("%a, %d %B %Y %H:%M:%S")))
    spark, result_df = anomaly_detection(
        algo_type,
        db_jdbc_address,
        start_time,
        end_time,
        tad_id_input,
        ns_ignore_list,
        agg_flow,
        pod_label,
        external_ip,
        svc_port_name,
        pod_name,
        pod_namespace
    )
    func_end_time = time.time()
    tad_id = write_anomaly_detection_result(
        result_df,
        db_jdbc_address,
        result_table_name,
        tad_id_input,
    )
    logger.info(
        "Anomaly Detection completed, id: {}, in {} "
        "seconds ".format(tad_id,
                          func_end_time - func_start_time))
    spark.stop()


if __name__ == '__main__':
    main()
