# Copyright 2023 Antrea Authors
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

from pyspark.sql import SparkSession

import anomaly_detection as ad


@pytest.fixture(scope="session")
def spark_session(request):
    spark_session = (
        SparkSession.builder.master("local")
        .appName("anomlay_detection_job_test")
        .getOrCreate()
    )
    request.addfinalizer(lambda: spark_session.sparkContext.stop())
    return spark_session


table_name = "default.flows"


@pytest.mark.parametrize(
    "test_input, expected_sql_query",
    [
        (
                ("", ""),
                "SELECT {} FROM {} GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("2022-01-01 00:00:00", ""),
                "SELECT {} FROM {} WHERE "
                "flowStartSeconds >= '2022-01-01 00:00:00' "
                "GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("", "2022-01-01 23:59:59"),
                "SELECT {} FROM {} WHERE "
                "flowEndSeconds < '2022-01-01 23:59:59' GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("2022-01-01 00:00:00", "2022-01-01 23:59:59"),
                "SELECT {} FROM {} WHERE "
                "flowStartSeconds >= '2022-01-01 00:00:00' AND "
                "flowEndSeconds < '2022-01-01 23:59:59' "
                "GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
    ],
)
def test_generate_sql_query(test_input, expected_sql_query):
    start_time, end_time = test_input
    sql_query = ad.generate_tad_sql_query(start_time, end_time)
    assert sql_query == expected_sql_query


diff_secs_throughput_list = [
    [5220, 4004471308], [4860, 4006917952], [6720, 4006373555],
    [7080, 10004969097], [6000, 4005703059], [7140, 4005517222],
    [3600, 4007380032], [3780, 4005277827], [4800, 4005435632],
    [8580, 4004723289], [6300, 4005760579], [4740, 4005486294],
    [8640, 4006172825], [5640, 4005486294], [8700, 4005561235],
    [7200, 1005533779], [5340, 4005486294], [6600, 4004706899],
    [6660, 4006355667], [5280, 4005277827], [4380, 4005277827],
    [7920, 4005355097], [7560, 4005615814], [7500, 4004496934],
    [8100, 4004839744], [4440, 4005486294], [7260, 4005370905],
    [5580, 4005277827], [4680, 4005277827], [6360, 4006503308],
    [8520, 4006191046], [6180, 4004834307], [5880, 4006201196],
    [5760, 4004465468], [7860, 4006448435], [6780, 4005542681]]

expected_ewma_row_list = [
    2002235654.0, 3004576803.0, 3505475179.0,
    6755222138.0, 5380462598.5, 4692989910.25,
    4350184971.125, 4177731399.0625, 4091583515.53125,
    4048153402.265625, 4026956990.6328125, 4016221642.3164062,
    4011197233.658203, 4008341763.8291016, 4006951499.414551,
    2506242639.2072754, 3255864466.6036377, 3630285682.801819,
    3818320674.9009094, 3911799250.9504547, 3958538538.9752274,
    3981946817.9876137, 3993781315.993807, 3999139124.9969034,
    4001989434.4984517, 4003737864.2492256, 4004554384.624613,
    4004916105.8123064, 4005096966.406153, 4005800137.2030764,
    4005995591.601538, 4005414949.300769, 4005808072.6503844,
    4005136770.3251925, 4005792602.662596, 4005667641.831298]


@pytest.mark.parametrize(
    "test_input, expected_output",
    [(diff_secs_throughput_list, expected_ewma_row_list), ],
)
def test_calculate_ewma(test_input, expected_output):
    ewma_row_list = ad.calculate_ewma(test_input)
    assert ewma_row_list == expected_output


expected_arima_row_list = [
    40044, 40069, 40063, 40055,
    10006, 49890, 40055, 40106,
    40058, 40054, 42977, 40055,
    37671, 40060, 42326, 37999,
    13902, 36050, 36215, 36385,
    40014, 40047, 40052, 40055,
    40047, 40047, 37220, 37309,
    37394, 37474, 37556, 37626,
    37686, 37758, 37809, 37878]


@pytest.mark.parametrize(
    "test_input, expected_output",
    [(diff_secs_throughput_list, expected_arima_row_list), ],
)
def test_calculate_arima(test_input, expected_output):
    arima_list = ad.calculate_arima(test_input)
    arima_list = [int(str(x)[:5]) for x in arima_list]
    assert arima_list == expected_output


stddev = 4.9198515356827E9

# Introduced 2 anomalies in between the lists
throughput_list = [
    4.0044713079999986E9, 4.006917951999995E9, 4.006373555E9,
    4.0064328864065647E9, 1.0001208441920076E10, 4.991089312943837E9,
    4.0055171211742964E9, 4.626073573776036E9, 4.534010184967284E9,
    4.466799561945112E9, 4.415333279500873E9, 4.374398755270068E9,
    4.34118839784096E9, 4.313543957535702E9, 4.2901708280305667E9,
    4.2701589854351115E9, 2.8325782582807236E9, 2.878340351423177E9,
    3.385947522781177E9, 3.5646004395330224E9, 3.6667007752395616E9,
    3.734521818953146E9, 3.783516303056411E9, 3.820958451443989E9,
    3.8506855053810143E10, 3.8756093718031974E9, 3.8975599768541164E9,
    3.9181189080811553E10, 3.9389864233827744E9, 3.958470156413464E9,
    3.9601061005316563E9, 3.9615580705907497E9, 3.9628158138629975E9,
    3.9641264728285837E9, 3.9652109214908457E9, 3.9664069982877073E9]

expanded_arima_row_list = [
    4004471307.9999986, 4006917951.999995, 4006373555.0,
    4005589532.039936, 10006702026.604738, 4989043846.332678,
    4005517137.571196, 4010659163.493726, 4005892608.887371,
    4005450688.4167933, 4297790490.967786, 4005582738.5384636,
    3767119154.3932557, 4006051692.1707573, 4232602969.5832963,
    3799968523.9055543, 1390254377.7501612, 3605038207.7053514,
    3621518502.8757067, 3638555962.676026, 4001438213.3433123,
    4004721638.3185096, 4005245177.0466213, 4005527131.198336,
    4004788075.4534545, 4004792510.4156027, 3722002449.8679004,
    3730981405.6425004, 3739427992.656704, 3747434614.2705755,
    3755687741.4894, 3762658349.287123, 3768651030.8630514,
    3775820305.253907, 3780919888.648877, 3787800077.0179124
]

expected_anomaly_list = [
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    True, False, False, True, False, False,
    False, False, False, False, False, False]


@pytest.mark.parametrize(
    "test_input, expected_arima_anomaly",
    [(["", "", "", "", "", "", "", stddev, "", expanded_arima_row_list,
       throughput_list], expected_anomaly_list), ],
)
def test_calculate_arima_anomaly(test_input, expected_arima_anomaly):
    anomaly_list = ad.calculate_arima_anomaly(test_input)
    assert anomaly_list == expected_arima_anomaly


@pytest.mark.parametrize(
    "test_input, expected_ewma_anomaly",
    [(["", "", "", "", "", "", "", stddev, "", expected_ewma_row_list,
       throughput_list], expected_anomaly_list), ],
)
def test_calculate_ewma_anomaly(test_input, expected_ewma_anomaly):
    anomaly_list = ad.calculate_ewma_anomaly(test_input)
    assert anomaly_list == expected_ewma_anomaly


expected_dbscan_anomaly_list = [
    False, False, False, False, True,
    True, False, False, False, False,
    False, False, False, False, False,
    False, True, True, False, False,
    False, False, False, False, True,
    False, False, True, False, False,
    False, False, False, False, False, False]


@pytest.mark.parametrize(
    "test_input, expected_dbscan_anomaly",
    [(["", "", "", "", "", "", "", "", "", "", throughput_list],
      expected_dbscan_anomaly_list), ],
)
def test_calculate_dbscan_anomaly(test_input, expected_dbscan_anomaly):
    anomaly_list = ad.calculate_dbscan_anomaly(test_input)
    assert anomaly_list == expected_dbscan_anomaly
