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
    'flowEndSeconds - flowStartSeconds as Diff_Secs',
    'max(throughput)'
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


def calculate_ewma(diff_secs_throughput):
    """
    The function calculates Exponential Weighted Moving Average (EWMA) for
    a given list of throughput values of a connection
    Args:
        diff_secs_throughput: Column of a dataframe containing difference in
        seconds between connection start and current flow, along with its
        throughput as a tuple
    Returns:
        A list of EWMA values calculated for the set of throughput values
        for that specific connection.
    """

    alpha = 0.5  # Can be changed and passed as UDF value later.
    prev_ewma_val = 0.0
    ewma_row = []
    for ele in diff_secs_throughput:
        ele_float = float(ele[1])
        curr_ewma_val = (1 - alpha) * prev_ewma_val + alpha * ele_float
        prev_ewma_val = curr_ewma_val
        ewma_row.append(float(curr_ewma_val))

    return ewma_row


def calculate_ewma_anomaly(dataframe):
    """
    The function categorizes whether a network flow is Anomalous or not
    based on the calculated EWMA value.
    A network flow record is anomalous if abs(throughput - ewma) > Standard
    Deviation.
    True - Anomalous Traffic, False - Not Anomalous

    Args:
        dataframe : The row of a dataframe containing all data related to
        this network connection.
    Returns:
        A list of boolean values which signifies if a network flow record
        of that connection is anomalous or not.
    """
    stddev = dataframe[7]
    ewma_arr = dataframe[9]
    throughput_arr = dataframe[10]
    anomaly_result = []

    if ewma_arr is None:
        logger.error("Error: EWMA values not calculated for this flow record")
        result = False
        anomaly_result.append(result)
    elif throughput_arr is None:
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
            elif throughput_arr[i] is None:
                logger.error("Error: Throughput values not in ideal format "
                             "for this flow record")
                result = False
            else:
                float(stddev)
                result = True if (abs(float(throughput_arr[i]) - float(
                    ewma_arr[i])) >
                                  float(stddev)) else False
            anomaly_result.append(result)
    return anomaly_result


def calculate_arima(diff_secs_throughput):
    """
    The function calculates AutoRegressive Integrated Moving Average
    (ARIMA) for a given list of throughput values of a connection
    Assumption: Since ARIMA needs a handful of data to train and start
    prediction, any connection with less than 3 flow records will not be
    taken into account for calculation. We return empty value in that case.
    Args:
        diff_secs_throughput: Column of a dataframe containing difference
        in seconds between connection start and current flow,
        along with its throughput as a tuple
    Returns:
        A list of ARIMA values calculated for the set of throughput values
        for that specific connection.
    """

    throughput_list = []
    for ele in diff_secs_throughput:
        throughput_list.append(float(ele[1]))
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
            predictions_final = [x for x in predictions_final]
            return predictions_final
        except Exception as e:
            logger.critical(
                "Error: ARIMA encountered error with the current flow "
                "record. err_msg: {}".format(e))
            return None


def calculate_arima_anomaly(dataframe):
    """
    The function categorizes whether a network flow is Anomalous or not based
    on the calculated ARIMA value. A traffic is anomalous if abs(throughput
    - arima) > Standard Deviation. True - Anomalous Traffic, False - Not
    Anomalous

    Args:
        dataframe : The row of a dataframe containing all data related to
        this network connection.
    Returns:
        A list of boolean values which signifies if a network flow record
        of that connection is anomalous or not.
    """

    stddev = dataframe[7]
    arima_arr = dataframe[9]
    throughput_arr = dataframe[10]
    anomaly_result = []

    if arima_arr is None:
        logger.error("Error: ARIMA values not calculated for this flow record")
        result = False
        anomaly_result.append(result)
    elif throughput_arr is None:
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
            elif throughput_arr[i] is None:
                logger.error("Error: Throughput values not in ideal format "
                             "for this flow record")
                result = False
            else:
                result = True if (abs(float(throughput_arr[i]) - float(
                    arima_arr[i])) > float(stddev)) else False
            anomaly_result.append(result)
    return anomaly_result


def calculate_dbscan(diff_secs_throughput):
    """
    The function is a placeholder function as anomaly detection with
    DBSCAN only inputs the throughput values. However, in order to maintain
    the tadetector table in click house, a placeholder column is required
    """
    # Currently just a placeholder function
    placeholder_throughput_list = []
    for i in range(len(diff_secs_throughput)):
        placeholder_throughput_list.append(0.0)
    return placeholder_throughput_list


def calculate_dbscan_anomaly(dataframe):
    """
    The function calculates Density-based spatial clustering of applications
    with Noise (DBSCAN) for a given list of throughput values of a connection
    Args:
        dataframe: The row of a dataframe containing all data related to this
        network connection.
        Assumption: Since DBSCAN needs only numeric value to train and start
        prediction, any connection with null values will not be taken
        into account for calculation. We return empty value in that case.
    Returns:
        A list of boolean values which signifies if a network flow records
        of the connection is anomalous or not based on DBSCAN
    """

    throughput_list = dataframe[10]
    anomaly_result = []
    np_throughput_list = np.array(throughput_list)
    np_throughput_list = np_throughput_list.reshape(-1, 1)
    outlier_detection = DBSCAN(min_samples=4, eps=250000000)
    clusters = outlier_detection.fit_predict(np_throughput_list)
    for i in clusters:
        if i == -1:
            anomaly_result.append(True)
        else:
            anomaly_result.append(False)
    return anomaly_result


def filter_df_with_true_anomalies(spark, plotDF, algo_type):
    plotDF = plotDF.withColumn(
        "new", f.arrays_zip(
            "flowEndSeconds", "algoCalc", "throughputs",
            "anomaly")).withColumn(
        "new", f.explode("new")).select(
        "sourceIP", "sourceTransportPort", "destinationIP",
        "destinationTransportPort",
        "protocolIdentifier", "flowStartSeconds",
        f.col("new.flowEndSeconds").alias("flowEndSeconds"),
        "throughputStandardDeviation", "algoType",
        f.col("new.algoCalc").alias("algoCalc"),
        f.col("new.throughputs").alias("throughput"),
        f.col("new.anomaly").alias("anomaly"))
    ret_plot = plotDF.where(~plotDF.anomaly.isin([False]))
    if ret_plot.count() == 0:
        ret_plot.append({
            "sourceIP": 'None',
            "sourceTransportPort": 0,
            "destinationIP": 'None',
            "destinationTransportPort": 0,
            "protocolIdentifier": 0,
            "flowStartSeconds": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "flowEndSeconds": 0,
            "throughputStandardDeviation": 0,
            "algoType": algo_type,
            "algoCalc": 0.0,
            "throughput": 0.0,
            "anomaly": "NO ANOMALY DETECTED"})
        ret_plot = spark.createDataFrame(ret_plot)
    return ret_plot


def plot_anomaly(spark, init_plot_df, algo_type, algo_func, anomaly_func,
                 tad_id_input):
    # Insert the Algo currently in use
    init_plot_df = init_plot_df.withColumn('algoType', f.lit(algo_type))
    # Common schema List
    common_schema_list = [
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
                   x[6], x[7], x[8], x[9], algo_func(x[8])))

    # Schema for the Dataframe to be created from the RDD
    algo_func_rdd_Schema = StructType(common_schema_list + [
        StructField('Diff_Secs, Throughput', ArrayType(StructType([
            StructField("Diff_Secs", LongType(), True),
            StructField("max(throughput)", DecimalType(38, 18), True)
        ]))),
        StructField('algoType', StringType(), True),
        StructField('algoCalc', ArrayType(DoubleType(), True))
    ])

    algo_DF = spark.createDataFrame(algo_func_rdd, algo_func_rdd_Schema)
    algo_DF = algo_DF.withColumn("throughputs", f.col(
        "Diff_Secs, Throughput.max(throughput)").cast(
        ArrayType(DecimalType(38, 18)))).drop("Diff_Secs, Throughput")

    # DF to RDD to calculate anomaly using EWMA, ARIMA and DBSCAN
    anomaly_func_rdd = algo_DF.rdd.map(
        lambda x: (x[0], x[1], x[2], x[3], x[4], x[5],
                   x[6], x[7], x[8], x[9], x[10],
                   anomaly_func(x)))

    # Schema for the Dataframe to be created from the RDD
    anomaly_func_rdd_Schema = StructType(common_schema_list + [
        StructField('algoType', StringType(), True),
        StructField('algoCalc', ArrayType(DoubleType(), True)),
        StructField('throughputs', ArrayType(DecimalType(38, 18), True)),
        StructField('anomaly', ArrayType(BooleanType(), True))
    ])
    anomalyDF = spark.createDataFrame(anomaly_func_rdd,
                                      anomaly_func_rdd_Schema)
    ret_plotDF = filter_df_with_true_anomalies(spark, anomalyDF, algo_type)
    # Write anomalous records to DB/CSV - Module WIP
    # Module to write to CSV. Optional.
    ret_plotDF = ret_plotDF.withColumn(
        "anomaly", f.col("anomaly").cast(
            "string"))
    ret_plotDF = ret_plotDF.withColumn('id', f.lit(str(tad_id_input)))
    return ret_plotDF


def generate_tad_sql_query(start_time, end_time):
    sql_query = ("SELECT {} FROM {} "
                 .format(", ".join(FLOW_TABLE_COLUMNS), table_name))
    if start_time:
        sql_query += "WHERE flowStartSeconds >= '{}' ".format(start_time)
    if end_time:
        if start_time:
            sql_query += "AND flowEndSeconds < '{}' ".format(end_time)
        else:
            sql_query += "WHERE flowEndSeconds < '{}' ".format(end_time)
    sql_query += "GROUP BY {} ".format(
        ", ".join(DF_GROUP_COLUMNS + ["flowEndSeconds"]))
    return sql_query


def anomaly_detection(algo_type, db_jdbc_address, start_time, end_time,
                      tad_id_input):
    spark = SparkSession.builder.getOrCreate()
    sql_query = generate_tad_sql_query(start_time, end_time)
    initDF = (
        spark.read.format("jdbc").option(
            'driver', "ru.yandex.clickhouse.ClickHouseDriver").option(
            "url", db_jdbc_address).option(
            "user", os.getenv("CH_USERNAME")).option(
            "password", os.getenv("CH_PASSWORD")).option(
            "query", sql_query).load()
    )

    prepared_DF = initDF.groupby(DF_GROUP_COLUMNS).agg(
        f.collect_list("flowEndSeconds").alias("flowEndSeconds"),
        f.stddev_samp("max(throughput)").alias("throughputStandardDeviation"),
        f.collect_list(f.struct(["Diff_Secs", "max(throughput)"])).alias(
            "Diff_Secs, Throughput"))

    if algo_type == "EWMA":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type, calculate_ewma,
                                calculate_ewma_anomaly, tad_id_input)
    elif algo_type == "ARIMA":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type, calculate_arima,
                                calculate_arima_anomaly, tad_id_input)
    elif algo_type == "DBSCAN":
        ret_plot = plot_anomaly(spark, prepared_DF, algo_type,
                                calculate_dbscan,
                                calculate_dbscan_anomaly, tad_id_input)
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
        """

    # TODO: change to use argparse instead of getopt for options
    try:
        opts, _ = getopt.getopt(
            sys.argv[1:],
            "ht:d:s:e:i:",
            [
                "help",
                "algo=",
                "db_jdbc_url=",
                "start_time=",
                "end_time=",
                "id=",
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
        elif opt in ("-i", "--id"):
            tad_id_input = arg

    func_start_time = time.time()
    logger.info("Script started at {}".format(
        datetime.now().strftime("%a, %d %B %Y %H:%M:%S")))
    spark, result_df = anomaly_detection(
        algo_type,
        db_jdbc_address,
        start_time,
        end_time,
        tad_id_input
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
