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

import logging
import os
import findspark
import time
import pyspark.sql.functions as f
import numpy as np
import warnings

from datetime import datetime
from pyspark.sql import SparkSession
from pyspark.sql.types import *
from scipy import stats
from scipy.special import boxcox, inv_boxcox
from statsmodels.tsa.arima.model import ARIMA
from sklearn.cluster import DBSCAN

findspark.init()
logger = logging.getLogger('anomaly_detection')
logger.setLevel(logging.INFO)
ch = logging.StreamHandler()
ch.setLevel(logging.INFO)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
logger.addHandler(ch)

url = "jdbc:clickhouse://localhost:30002" #This is a local instance. Changes based on port-forwarding, can be used on custom port.
table_name = "default.flows"


# Column names of flow record table in Clickhouse database used in anomaly detection job
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
GROUP_COLUMNS = [
	'sourceIP',
	'sourceTransportPort',
	'destinationIP',
	'destinationTransportPort',
	'protocolIdentifier',
	'flowStartSeconds',
	'flowEndSeconds'
]

def calculate_ewma(diff_secs_throughput):
	"""
	The function calculates Exponential Weighted Moving Average (EWMA) for a given list of throughput values of a connection
	Args:
		diff_secs_throughput: Column of a dataframe containing difference in seconds between connection start and current flow, along with its throughput as a tuple
	Returns:
		A list of EWMA values calculated for the set of throughput values for that specific connection.
	"""

	alpha = 0.5 #Can be changed and passed as UDF value later.
	prev_ewma_val = 0.0
	ewma_row = []
	for ele in diff_secs_throughput:
		ele_float = float(ele[1])
		curr_ewma_val = (1-alpha)*prev_ewma_val + alpha * ele_float
		prev_ewma_val = curr_ewma_val
		ewma_row.append(curr_ewma_val)

	return ewma_row

def calculate_ewma_anomaly(dataframe):
	"""
	The function categorizes whether a network flow is Anomalous or not based on the calculated EWMA value.
	A network flow record is anomalous if abs(throughput - ewma) > Standard Deviation. 
	True - Anomalous Traffic, False - Not Anomalous

	Args:
		dataframe : The row of a dataframe containing all data related to this network connection.
	Returns:
		A list of boolean values which signifies if a network flow record of that connection is anomalous or not.
	"""

	stddev = dataframe[6]
	ewma_arr = dataframe[7]
	throughput_arr = dataframe[9]
	anomaly_result = []

	if ewma_arr is None:
		logger.error("Error: EWMA values not calculated for this flow record")
		result = False
		anomaly_result.append(result)
	elif throughput_arr is None:
		logger.error("Error: Throughput values not in ideal format for this flow record")
		result = False
		anomaly_result.append(result)
	else:
		num_records = len(ewma_arr)

		for i in range(num_records):
			if stddev is None:
				logger.error("Error: Too Few Throughput Values for Standard Deviation to be calculated.")
				result = False
			elif throughput_arr[i] is None:
				logger.error("Error: Throughput values not in ideal format for this flow record")
				result = False
			else:
				result = True if (abs(float(throughput_arr[i]) - float(ewma_arr[i])) > float(stddev)) else False

			anomaly_result.append(result)

	return anomaly_result

def calculate_arima(diff_secs_throughput):
	"""
	The function calculates AutoRegressive Integrated Moving Average (ARIMA) for a given list of throughput values of a connection
	Assumption: Since ARIMA needs a handful of data to train and start prediction, any connection with less than 3 flow records will
				not be taken into account for calculation. We return empty value in that case.
	Args:
		diff_secs_throughput: Column of a dataframe containing difference in seconds between connection start and current flow, along with its throughput as a tuple
	Returns:
		A list of ARIMA values calculated for the set of throughput values for that specific connection.
	"""

	throughput_list = []
	for ele in diff_secs_throughput:
		throughput_list.append(float(ele[1]))
	
	if len(throughput_list) <=3:
		logger.error("Error: Too Few throughput values for ARIMA to work with")
		return None
	else:
		try:
			warnings.filterwarnings("ignore")
			throughput_list = [x+1 for x in throughput_list]
			throughput_list_bxcx, revvar = stats.boxcox(throughput_list)
			throughput_list_bxcx = throughput_list_bxcx.tolist()
			train, test = throughput_list_bxcx[0:3], throughput_list_bxcx[3:]
			if len(test) == 1:
				test = [test]
			history = [x for x in train]
			predictions = list()
			for t in range(len(test)):
				model = ARIMA(history, order=(1,1,1))
				model_fit = model.fit()
				output = model_fit.forecast()
				yhat = output[0]
				predictions.append(yhat)
				obs = test[t]
				history.append(obs)

			predictions_final = train + predictions
			predictions_final = inv_boxcox(predictions_final, revvar)
			predictions_final = predictions_final.tolist()

			predictions_final = [x-1 for x in predictions_final]

			return predictions_final
		except:
			logger.critical("Error: ARIMA encountered error with the current flow record.")
			return None
		

def calculate_arima_anomaly(dataframe):
	"""
	The function categorizes whether a network flow is Anomalous or not based on the calculated ARIMA value.
	A traffic is anomalous if abs(throughput - arima) > Standard Deviation. 
	True - Anomalous Traffic, False - Not Anomalous

	Args:
		dataframe : The row of a dataframe containing all data related to this network connection.
	Returns:
		A list of boolean values which signifies if a network flow record of that connection is anomalous or not.
	"""	

	stddev = dataframe[6]
	arima_arr = dataframe[8]
	throughput_arr = dataframe[9]
	anomaly_result = []

	if arima_arr is None:
		logger.error("Error: ARIMA values not calculated for this flow record")
		result = False
		anomaly_result.append(result)
	elif throughput_arr is None:
		logger.error("Error: Throughput values not in ideal format for this flow record")
		result = False
		anomaly_result.append(result)
	else:
		num_records = len(arima_arr)

		for i in range(num_records):
			if stddev is None:
				logger.error("Error: Too Few Throughput Values for Standard Deviation to be calculated.")
				result = False
			elif throughput_arr[i] is None:
				logger.error("Error: Throughput values not in ideal format for this flow record")
				result = False
			else:
				result = True if (abs(float(throughput_arr[i]) - float(arima_arr[i])) > float(stddev)) else False
			
			anomaly_result.append(result)
	
	return anomaly_result


def calculate_dbscan_anomaly(dataframe):
	"""
	The function calculates Density-based spatial clustering of applications with Noise (DBSCAN) for a given list of throughput values of a connection
	Args:
		dataframe: The row of a dataframe containing all data related to this network connection.
		Assumption: Since DBSCAN needs only numeric value to train and start prediction, any connection with null values will not be taken into account
					for calculation. We return empty value in that case.
	Returns:
		A list of boolean values which signifies if a network flow records of the connection is anomalou or not based on DBSCAN
	"""
	
	throughput_list = dataframe[9]
	anomaly_result = []

	np_throughput_list = np.array(throughput_list)
	np_throughput_list = np_throughput_list.reshape(-1,1)

	outlier_detection = DBSCAN(min_samples = 4, eps=250000000)
	clusters = outlier_detection.fit_predict(np_throughput_list)

	for i in clusters:
		if i == -1:
			anomaly_result.append(True)
		else:
			anomaly_result.append(False)

	return anomaly_result

def anomaly_detection():
	spark = SparkSession.builder.config("spark.jars","./clickhouse-jdbc-0.3.2-test3-all.jar").getOrCreate()
	sc = spark.sparkContext
	sc.setLogLevel("ERROR")
	sql_query = "select {} from default.flows group by {} ORDER BY flowStartSeconds, flowEndSeconds".format(", ".join(FLOW_TABLE_COLUMNS), ", ".join(GROUP_COLUMNS))
	
	plotDF = spark.read \
		.format("jdbc") \
		.option('driver', "com.clickhouse.jdbc.ClickHouseDriver") \
		.option("url", url) \
		.option("user", os.getenv("CH_USERNAME")) \
		.option("password",os.getenv("CH_PASSWORD")) \
		.option("query",sql_query) \
		.load()

	plotDF1 = plotDF.groupby("sourceIP","sourceTransportPort","destinationIP","destinationTransportPort","protocolIdentifier","flowStartSeconds").agg(f.stddev_samp("max(throughput)").alias("Throughput Standard Deviation"))
	plotDF2 = plotDF.groupby("sourceIP","sourceTransportPort","destinationIP","destinationTransportPort","protocolIdentifier","flowStartSeconds").agg(f.collect_list(f.struct(["Diff_Secs","max(throughput)"])))
	plotDF2 = plotDF2.withColumnRenamed("collect_list(struct(Diff_Secs, max(throughput)))","Diff_Secs, Throughput")

	# Joining Two different Groupby DFs
	plotDF3 = plotDF1.join(plotDF2, ["sourceIP","sourceTransportPort","destinationIP","destinationTransportPort","protocolIdentifier","flowStartSeconds"])

	#Calculate EWMA and ARIMA Values on the DF
	rdd2 = plotDF3.rdd.map(lambda x: (x[0], x[1], x[2], x[3], x[4], x[5], x[6], x[7], calculate_ewma(x[7]), calculate_arima(x[7])))

	#Schema for the Dataframe to ve created from the RDD
	rdd2Schema = StructType([
		StructField('sourceIP', StringType(), True),
		StructField('sourceTransportPort', LongType(), True),
		StructField('destinationIP', StringType(), True),
		StructField('destinationTransportPort', LongType(), True),
		StructField('protocolIdentifier', LongType(), True),
		StructField('flowStartSeconds', TimestampType(), True),
		StructField('Throughput Standard Deviation',DoubleType(), True),
		StructField('Diff_Secs, Throughput', ArrayType(StructType([
														StructField("Diff_Secs", LongType(), True), 
														StructField("max(throughput)",DecimalType(38,18),True)
														]))),
		StructField('EWMA',ArrayType(DoubleType(),True)),
		StructField('ARIMA', ArrayType(DoubleType(),True))
	])

	plotDF4 = spark.createDataFrame(rdd2, rdd2Schema)
	plotDF4 = plotDF4.withColumn("Throughputs", f.col("Diff_Secs, Throughput.max(throughput)").cast(ArrayType(DecimalType(38,18)))).drop("Diff_Secs, Throughput")

	#DF to RDD to calculate anomaly using EWMA, ARIMA and DBSCAN
	rdd3 = plotDF4.rdd.map(lambda x: (x[0], x[1], x[2], x[3], x[4], x[5], x[6], x[7], x[8], x[9], calculate_ewma_anomaly(x), calculate_arima_anomaly(x), calculate_dbscan_anomaly(x)))	

	#Schema for the Dataframe to ve created from the RDD
	rdd3Schema = StructType([
		StructField('sourceIP', StringType(), True),
		StructField('sourceTransportPort', LongType(), True),
		StructField('destinationIP', StringType(), True),
		StructField('destinationTransportPort', LongType(), True),
		StructField('protocolIdentifier', LongType(), True),
		StructField('flowStartSeconds', TimestampType(), True),
		StructField('Throughput Standard Deviation', DoubleType(), True),
		StructField('EWMA', ArrayType(DoubleType(),True)),
		StructField('ARIMA', ArrayType(DoubleType(),True)),
		StructField('Throughputs', ArrayType(DecimalType(38,18),True)),
		StructField('EWMA Anomaly(T/F)', ArrayType(BooleanType(),True)),
		StructField('ARIMA Anomaly(T/F)', ArrayType(BooleanType(),True)),
		StructField('DBSCAN Anomaly(T/F)', ArrayType(BooleanType(),True))
	])

	plotDF5 = spark.createDataFrame(rdd3, rdd3Schema)
	#Write anomalous records to DB/CSV - Module WIP
	#Module to write to CSV. Optional.
	plotDF5 = plotDF5.withColumn("Throughput Standard Deviation", f.col("Throughput Standard Deviation").cast("string")).withColumn("EWMA", f.col("EWMA").cast("string")).withColumn("ARIMA", f.col("ARIMA").cast("string")).withColumn("Throughputs", f.col("Throughputs").cast("string")).withColumn("EWMA Anomaly(T/F)", f.col("EWMA Anomaly(T/F)").cast("string")).withColumn("ARIMA Anomaly(T/F)", f.col("ARIMA Anomaly(T/F)").cast("string")).withColumn("DBSCAN Anomaly(T/F)", f.col("DBSCAN Anomaly(T/F)").cast("string"))
	plotDF5.write.option("header",True).csv("./csv-outputs/anomaly-detection-results-{}".format(datetime.now().strftime("%d-%m-%Y-%H-%M-%S")))


def main():
	start_time = time.time()
	logger.info("Script started at {}".format(datetime.now().strftime("%a, %d %B %Y %H:%M:%S")))
	anomaly_detection()
	end_time = time.time()
	logger.info("It took {} seconds to finish the script".format(end_time-start_time))

if __name__ == '__main__':
	main()