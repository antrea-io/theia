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

import time
import logging
import findspark
import numpy as np
import warnings

from datetime import datetime
from pyspark.sql import SparkSession
import pyspark.sql.functions as f
from pyspark.sql.functions import udf
from pyspark.sql.types import *
from scipy import stats
from scipy.special import boxcox, inv_boxcox
from statsmodels.tsa.arima.model import ARIMA


findspark.init()
logger = logging.getLogger('sparkstreaming_anomaly_detection')
logger.setLevel(logging.INFO)
ch = logging.StreamHandler()
ch.setLevel(logging.INFO)
formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
logger.addHandler(ch)

url = "jdbc:clickhouse://localhost:30002" #Changes based on port-forwarding, can be used on custom port.
table_name = "default.flows"
driver = "com.clickhouse.jdbc.ClickHouseDriver"

# Column names of flow record table used in anomaly detection
FLOW_TABLE_COLUMNS = [
	'sourceIP',
	'sourceTransportPort',
	'destinationIP',
	'destinationTransportPort',
	'protocolIdentifier',
	'toUnixTimestamp(flowStartSeconds) as flowStartSecondsUnix',
	'toUnixTimestamp(flowEndSeconds) as flowEndSecondsUnix',
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


def calculate_ewma_anomaly(diff_secs_throughput, throughput_stddev):
	"""
	The function calculates Exponential Weighted Moving Average (EWMA) for a given list of throughput values of a connection
	After calculation, it uses a threshold parameter to identify if the record is anomalous or not
	A network flow record is anomalous if abs(throughput - ewma) > Standard Deviation. 
	True - Anomalous Traffic, False - Not Anomalous
	
	Args:
		diff_secs_throughput: Column of a dataframe containing difference in seconds between connection start and current flow, along with its throughput as a tuple
		throughput_stddev: Standard Deviation of all the throughput values for a specific connection
	Returns:
		anomaly_result: A list of boolean values which signifies if a network flow record of that connection is anomalous or not.
	"""

	throughput_list = []
	for ele in diff_secs_throughput:
		throughput_list.append(float(ele[1]))

	alpha = 0.5 #Can be changed and passed as UDF value later.
	prev_ewma_val = 0.0
	ewma_row = []
	for ele in throughput_list:
		ele_float = float(ele)
		curr_ewma_val = (1-alpha)*prev_ewma_val + alpha * ele_float
		prev_ewma_val = curr_ewma_val
		ewma_row.append(curr_ewma_val)

	anomaly_result = []

	if ewma_row is None:
		logger.error("Error: EWMA values not calculated for this flow record")
		result = False
		anomaly_result.append(result)
	elif throughput_list is None:
		logger.error("Error: Throughput values not in ideal format for this flow record")
		result = False
		anomaly_result.append(result)
	else:
		num_records = len(ewma_row)
		for i in range(num_records):
			if throughput_stddev is None:
				logger.error("Error: Too Few Throughput Values for Standard Deviation to be calculated.")
				result = False
			elif throughput_list[i] is None:
				logger.error("Error: Throughput values not in ideal format for this flow record")
				result = False
			else:
				result = True if (abs(float(throughput_list[i]) - float(ewma_row[i])) > float(throughput_stddev)) else False
			
			anomaly_result.append(result)

	return anomaly_result

#Converting calculate_ewma function into a UDF
UDF_calculate_ewma = udf(calculate_ewma_anomaly, ArrayType(BooleanType()))

def calculate_arima_anomaly(diff_secs_throughput, throughput_stddev):
	"""
	The function calculates AutoRegressive Integrated Moving Average (ARIMA) for a given list of throughput values of a connection
	After calculation, it uses a threshold parameter to identify if the record is anomalous or not
	A network flow record is anomalous if abs(throughput - arima) > Standard Deviation. 
	True - Anomalous Traffic, False - Not Anomalous
	Assumption: Since ARIMA needs a handful of data to train and start prediction, any connection with less than 3 flow records will
				not be taken into account for calculation. We return empty value in that case.

	Args:
		diff_secs_throughput: Column of a dataframe containing difference in seconds between connection start and current flow, along with its throughput as a tuple
		throughput_stddev: Standard Deviation of all the throughput values for a specific connection
	Returns:
		anomaly_result: A list of boolean values which signifies if a network flow record of that connection is anomalous or not.
	"""

	throughput_list = []
	for ele in diff_secs_throughput:
		throughput_list.append(float(ele[1]))
	
	anomaly_result = []
	
	if len(throughput_list) <=3:
		logger.error("Error: Very few Throughput values for ARIMA to work with")
		result = False
		anomaly_result.append(result)
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

			if predictions_final is None:
				logger.error("Error: ARIMA values not calculated for this flow record")
				result = False	
				anomaly_result.append(result)
			elif throughput_list is None:
				logger.error("Error: Throughput values not in ideal format for this flow record")
				result = False
				anomaly_result.append(result)
			else:
				num_records = len(predictions_final)

				for i in range(num_records):
					if throughput_stddev is None:
						logger.error("Error: Too Few Throughput Values for Standard Deviation to be calculated.")
						result = False
					elif throughput_list[i] is None:
						logger.error("Error: Throughput values not in ideal format for this flow record")
						result = False
					else:
						result = True if (abs(float(throughput_list[i]) - float(predictions_final[i])) > float(throughput_stddev)) else False
					
					anomaly_result.append(result)
		except:
			logger.critical("Error: ARIMA encountered error with the current flow record.")
			result = False
			anomaly_result.append(result)

	return anomaly_result

#Converting calculate_arima function into a UDF
UDF_calculate_arima = udf(calculate_arima_anomaly, ArrayType(BooleanType()))


def sparkStreaming():
	spark = SparkSession.builder.config("spark.jars","./clickhouse-jdbc-0.3.2-test3-all.jar, ./spark-streaming-jdbc-source-3.1.1_0.0.1.jar") \
			.appName("StreamingCount").getOrCreate()
	sc = spark.sparkContext
	sc.setLogLevel("ERROR")

	sql_query = "select {} from flows group by {} ORDER BY flowStartSeconds, flowEndSeconds".format(", ".join(FLOW_TABLE_COLUMNS), ", ".join(GROUP_COLUMNS))

	"""Creating a pre-defined Schema for the Dataframe so that dataframe can identify the data type for each column
	even when the number of data is very less for the spark session to identify automatically. Avoids Sampling ValueError"""

	streamSchema = StructType([
			StructField('sourceIP', StringType(), True),
			StructField('sourceTransportPort', LongType(), True),
			StructField('destinationIP', StringType(), True),
			StructField('destinationTransportPort', LongType(), True),
			StructField('protocolIdentifier', LongType(), True),
			StructField('flowStartSecondsUnix', DecimalType(38,18), True),
			StructField('flowEndSecondsUnix', DecimalType(38,18), True),
			StructField('Throughput Standard Deviation',DoubleType(), True),
			StructField('Diff_Secs', DecimalType(38,18), True),
			StructField('max(throughput)', DecimalType(38,18),True)
		])

	sql_stream = spark.readStream.format("jdbc-streaming") \
				.schema(streamSchema) \
				.options(url=url, 
						query = sql_query, 
						driver=driver, 
						offsetColumn='flowEndSecondsUnix', 
						startingOffsets= "earliest") \
				.load()

	logger.info("Is Spark Streaming? {}".format(sql_stream.isStreaming))

	sql_stream = sql_stream.withColumn("flowStartSecondsUnix", f.from_unixtime(f.col("flowStartSecondsUnix"))).withColumn("flowEndSecondsUnix", f.from_unixtime(f.col("flowEndSecondsUnix")))

	plotDF1 = sql_stream.groupby("sourceIP","sourceTransportPort","destinationIP","destinationTransportPort","protocolIdentifier","flowStartSecondsUnix") \
				.agg(f.stddev_samp("max(throughput)").alias("Throughput Standard Deviation"), f.collect_list(f.struct(["Diff_Secs","max(throughput)"])))
	plotDF1 = plotDF1.withColumnRenamed("collect_list(struct(Diff_Secs, max(throughput)))","Diff_Secs, Throughput")

	#Calculate EWMA and ARIMA Values on the DF
	plotDF1 = plotDF1.withColumn("EWMA", UDF_calculate_ewma(f.col("Diff_Secs, Throughput"), f.col("Throughput Standard Deviation")))
	plotDF1 = plotDF1.withColumn("ARIMA", UDF_calculate_arima(f.col("Diff_Secs, Throughput"), f.col("Throughput Standard Deviation")))
	
	try:
		query = plotDF1.writeStream \
			.format("console") \
			.outputMode("complete") \
			.trigger(processingTime='10 seconds') \
			.start()
		query.awaitTermination() #Timeout Parameter can be passed ([int] values in seconds)
	except Exception as e:
		logger.critical("Error: StreamingQueryException encountered. {}".format(e))
	
	#On some condition <To-Do>, Streaming Query can be stopped
	# query.stop()

def main():
	start_time = time.time()
	logger.info("Script started at {}".format(datetime.now().strftime("%a, %d %B %Y %H:%M:%S")))
	sparkStreaming()
	end_time = time.time()
	logger.info("It took {} seconds to finish the script".format(end_time-start_time))

if __name__ == '__main__':
	main()