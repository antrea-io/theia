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
        SparkSession.builder.master("local").appName(
            "anomaly_detection_job_test").getOrCreate()
    )
    request.addfinalizer(lambda: spark_session.sparkContext.stop())
    return spark_session


table_name = "default.flows"
inbound_condition = (
    "ilike(destinationPodLabels, '%\"app\":\"clickhouse\"%') ")
outbound_condition = ("ilike(sourcePodLabels, '%\"app\":\"clickhouse\"%')")
inbound_condition_podname = (
    "destinationPodName = 'TestPodName'")
outbound_condition_podname = (
    "sourcePodName = 'TestPodName'")
inbound_condition_podnamespace = (
    " AND destinationPodNamespace = 'TestPodNamespace'")
outbound_condition_podnamespace = (
    " AND sourcePodNamespace = 'TestPodNamespace'")


@pytest.mark.parametrize(
    "test_input, expected_sql_query",
    [
        (
                ("", "", [], "", "", "", "", "", ""),
                "SELECT {} FROM {} GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("2022-01-01 00:00:00", "", [], "", "", "", "", "", ""),
                "SELECT {} FROM {} WHERE "
                "flowStartSeconds >= '2022-01-01 00:00:00' "
                "GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("", "2022-01-01 23:59:59", [], "", "", "", "", "", ""),
                "SELECT {} FROM {} WHERE "
                "flowEndSeconds < '2022-01-01 23:59:59' GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("2022-01-01 00:00:00", "2022-01-01 23:59:59", [], "", "", "",
                 "", "", ""),
                "SELECT {} FROM {} WHERE "
                "flowStartSeconds >= '2022-01-01 00:00:00' AND "
                "flowEndSeconds < '2022-01-01 23:59:59' "
                "GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("", "", ["mock_ns", "mock_ns2"], "", "", "", "", "", ""),
                "SELECT {} FROM {} WHERE "
                "sourcePodNamespace NOT IN ('mock_ns', 'mock_ns2') AND "
                "destinationPodNamespace NOT IN ('mock_ns', 'mock_ns2') "
                "GROUP BY {} ".format(
                    ", ".join(ad.FLOW_TABLE_COLUMNS),
                    table_name,
                    ", ".join(ad.DF_GROUP_COLUMNS + ['flowEndSeconds']),
                ),
        ),
        (
                ("", "", [], "external", "", "10.0.0.1", "", "", ""),
                "SELECT {} FROM {} WHERE "
                "flowType = 3 AND destinationIP = '10.0.0.1' "
                "GROUP BY {} ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_EXTERNAL),
                    table_name,
                    ", ".join(
                        ad.DF_AGG_GRP_COLUMNS_EXTERNAL + ['flowEndSeconds']),
                )
        ),
        (
                ("", "", [], "pod", "\"app\":\"clickhouse\"", "", "", "",
                ""),
                "SELECT * FROM "
                "(SELECT {0} FROM {1} WHERE {2}  GROUP BY {3}) "
                "UNION ALL "
                "(SELECT {4} FROM {1} WHERE {5}  GROUP BY {3}) ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_POD_INBOUND),
                    table_name, inbound_condition,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_POD + ['flowEndSeconds']),
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_POD_OUTBOUND),
                    outbound_condition)
        ),
        (
                ("", "", [], "pod", "", "", "", "TestPodName", ""),
                "SELECT * FROM "
                "(SELECT {0} FROM {1} WHERE {2}  GROUP BY {3}) "
                "UNION ALL "
                "(SELECT {4} FROM {1} WHERE {5}  GROUP BY {3}) ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_PODNAME_INBOUND),
                    table_name, inbound_condition_podname,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_PODNAME +
                    ['flowEndSeconds']),
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_PODNAME_OUTBOUND),
                    outbound_condition_podname)
        ),
        (
                ("", "", [], "pod", "", "", "", "TestPodName",
                "TestPodNamespace"),
                "SELECT * FROM "
                "(SELECT {0} FROM {1} WHERE {2}  GROUP BY {3}) "
                "UNION ALL "
                "(SELECT {4} FROM {1} WHERE {5}  GROUP BY {3}) ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_PODNAME_INBOUND),
                    table_name, inbound_condition_podname +
                    inbound_condition_podnamespace,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_PODNAME +
                    ['flowEndSeconds']),
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_PODNAME_OUTBOUND),
                    outbound_condition_podname +
                    outbound_condition_podnamespace)
        ),
        (
                ("", "", ["mock_ns", "mock_ns2"], "pod",
                 "\"app\":\"clickhouse\"", "", "", "", ""),
                "SELECT * FROM "
                "(SELECT {0} FROM {1} WHERE {2} {6} GROUP BY {3}) "
                "UNION ALL "
                "(SELECT {4} FROM {1} WHERE {5} {6} GROUP BY {3}) ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_POD_INBOUND),
                    table_name, inbound_condition,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_POD + ['flowEndSeconds']),
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_POD_OUTBOUND),
                    outbound_condition,
                    "AND sourcePodNamespace NOT IN ('mock_ns', 'mock_ns2') AND"
                    " destinationPodNamespace NOT IN ('mock_ns', 'mock_ns2')")
        ),
        (
                ("", "", [], "svc", "", "", "", "", ""),
                "SELECT {} FROM {} WHERE "
                "destinationServicePortName <> '' "
                "GROUP BY {} ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_SVC),
                    table_name,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_SVC + ['flowEndSeconds']),
                )
        ),
        (
                ("", "", [], "svc", "", "", "test-service-port-name", "", ""),
                "SELECT {} FROM {} WHERE "
                "destinationServicePortName = 'test-service-port-name' "
                "GROUP BY {} ".format(
                    ", ".join(ad.AGG_FLOW_TABLE_COLUMNS_SVC),
                    table_name,
                    ", ".join(ad.DF_AGG_GRP_COLUMNS_SVC + ['flowEndSeconds']),
                )
        ),
    ],
)
def test_generate_sql_query(test_input, expected_sql_query):
    (start_time, end_time, ns_ignore_list, agg_flow, pod_label, external_ip,
     svc_port_name, pod_name, pod_namespace) = test_input
    sql_query = ad.generate_tad_sql_query(
        start_time, end_time, ns_ignore_list, agg_flow, pod_label,
        external_ip, svc_port_name, pod_name, pod_namespace)
    assert sql_query == expected_sql_query


# Introduced 2 anomalies in between the lists
throughput_list = [
    4007380032, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4006917952, 4004471308, 4005277827, 4005486294,
    4005435632, 4004465468, 4005336400, 4006201196, 4005546675,
    4005703059, 4004631769, 4006915708, 4004834307, 4005943619,
    4005760579, 4006503308, 4006580124, 4006524102, 4005521494,
    4004706899, 4006355667, 4006373555, 4005542681, 4006120227,
    4003599734, 4005561673, 4005682768, 10004969097, 4005517222,
    1005533779, 4005370905, 4005589772, 4005328806, 4004926121,
    4004496934, 4005615814, 4005798822, 50007861276, 4005396697,
    4005148294, 4006448435, 4005355097, 4004335558, 4005389043,
    4004839744, 4005556492, 4005796992, 4004497248, 4005988134,
    205881027, 4004638304, 4006191046, 4004723289, 4006172825,
    4005561235, 4005658636, 4006005936, 3260272025, 4005589772]

expected_ewma_row_list = [
    2003690016.0, 3005303984.0, 3504887646.0,
    3755082736.5, 3880284515.25, 3942860073.625,
    3974889012.8125, 3989680160.40625, 3997478993.703125,
    4001482643.8515625, 4003459137.9257812, 4005188544.9628906,
    4004829926.4814453, 4005053876.7407227, 4005270085.3703613,
    4005352858.6851807, 4006135405.3425903, 4005303356.671295,
    4005290591.8356476, 4005388442.917824, 4005412037.458912,
    4006164994.729456, 4005318151.364728, 4005297989.182364,
    4005392141.5911818, 4005413886.795591, 4006165919.3977957,
    4005318613.698898, 4005298220.349449, 4005392257.1747246,
    4005413944.5873623, 4006165948.293681, 4005318628.1468406,
    4005298227.5734205, 4005392260.7867103, 4005413946.3933554,
    4004939707.1966777, 4005138053.598339, 4005669624.7991695,
    4005608149.899585, 4005655604.4497924, 4005143686.7248964,
    4006029697.362448, 4005432002.181224, 4005687810.590612,
    4005724194.795306, 4006113751.397653, 4006346937.698827,
    4006435519.8494134, 4005978506.9247065, 4005342702.962353,
    4005849184.9811764, 4006111369.990588, 4005827025.495294,
    4005973626.2476473, 4004786680.1238236, 4005174176.5619116,
    4005428472.280956, 7005198784.640478, 5505358003.320239,
    3255445891.1601195, 3630408398.08006, 3817999085.04003,
    3911663945.520015, 3958295033.2600074, 3981395983.630004,
    3993505898.815002, 3999652360.407501, 27003756818.20375,
    15504576757.601875, 9754862525.800938, 6880655480.400469,
    5443005288.700234, 4723670423.350117, 4364529733.175058,
    4184684738.587529, 4095120615.2937646, 4050458803.646882,
    4027478025.823441, 4016733079.9117203, 2111307053.4558601,
    3057972678.72793, 3532081862.363965, 3768402575.6819825,
    3887287700.340991, 3946424467.6704955, 3976041551.835248,
    3991023743.917624, 3625647884.4588118, 3815618828.229406]


@pytest.mark.parametrize(
    "test_input, expected_output",
    [(throughput_list, expected_ewma_row_list), ],
)
def test_calculate_ewma(test_input, expected_output):
    ewma_row_list = ad.calculate_ewma(test_input)
    assert ewma_row_list == expected_output


expected_arima_row_list = [
    40073, 40069, 40044, 40044, 40052, 40055, 40054, 40065,
    40067, 40055, 40055, 40055, 40056, 40060, 40054, 40054,
    40054, 40056, 40059, 40054, 40053, 40054, 40056, 40059,
    40054, 40053, 40054, 40056, 40059, 40054, 40053, 40054,
    40055, 40059, 40054, 40053, 40054, 40053, 40051, 40052,
    40055, 40055, 40054, 40053, 40057, 40055, 40056, 40057,
    40059, 40062, 40062, 40058, 40056, 40059, 40059, 40058,
    40057, 40051, 40053, 98892, 63727, 11685, 56532, 39834,
    39838, 39841, 39844, 39847, 39850, 51939, 42449, 41960,
    41622, 41418, 41398, 41380, 41362, 41344, 41328, 41311,
    41296, 37786, 39971, 39973, 39973, 39974, 39975, 39976,
    39977, 39795]


@pytest.mark.parametrize(
    "test_input, expected_output",
    [(throughput_list, expected_arima_row_list), ],
)
def test_calculate_arima(test_input, expected_output):
    arima_list = ad.calculate_arima(test_input)
    arima_list = [int(str(x)[:5]) for x in arima_list]
    assert arima_list == expected_output


stddev = 4.9198515356827E9

expanded_arima_row_list = [
    4007380031.999998, 4006917951.999995, 4004471307.9999986,
    4004471338.531294, 4005277824.246516, 4005540171.967446,
    4005417708.3450212, 4006596903.605313, 4006766397.4873962,
    4005464712.562591, 4005505914.47936, 4005509545.4651246,
    4005696322.736301, 4006033558.7986755, 4005433015.8854957,
    4005412742.398469, 4005435189.6809897, 4005639605.435888,
    4005955240.943543, 4005417006.2521853, 4005394611.0058765,
    4005423107.5286036, 4005615008.6452837, 4005930744.817921,
    4005411345.290995, 4005387458.632538, 4005419217.319681,
    4005600201.8535633, 4005919351.798048, 4005407929.6769786,
    4005383100.809166, 4005417053.9240017, 4005590088.792332,
    4005912966.4215846, 4005405453.2374043, 4005380049.8736587,
    4005415584.314455, 4005347223.1615577, 4005118865.3445063,
    4005284593.2700768, 4005512211.463756, 4005544131.8972206,
    4005486224.1983933, 4005378548.2363796, 4005763254.8659606,
    4005506442.62549, 4005639283.9637246, 4005700488.7304854,
    4005994680.054991, 4006202832.9615264, 4006238860.0935726,
    4005890090.9782805, 4005645200.299684, 4005901661.0918307,
    4005961401.6530385, 4005890634.274731, 4005710152.5932317,
    4005117957.4774227, 4005335916.072919, 9889541964.746168,
    6372659702.031328, 1168584960.8614435, 5653370820.402384,
    3983485999.737595, 3983817981.649843, 3984128774.440398,
    3984428874.23501, 3984757281.599942, 3985066198.7098646,
    5193940800.99889, 4244915044.8262258, 4196033928.4701886,
    4163392386.852055, 4141770112.8968925, 4139754955.6538,
    4137999352.749037, 4136188331.593653, 4134572893.2860427,
    4132811885.9396014, 4131164397.6772537, 4129639059.839085,
    3778811317.851506, 3997193748.126457, 3997318079.4362803,
    3997388443.8144045, 3997506294.672869, 3997592427.864033,
    3997686533.833508, 3997782900.975419, 3979560170.4104342]

expected_anomaly_list_arima = [
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, True, True,
    True, False, False, False, False, False,
    False, False, True, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False]


@pytest.mark.parametrize(
    "test_input, expected_arima_anomaly",
    [((throughput_list, stddev), expected_anomaly_list_arima), ],
)
def test_calculate_arima_anomaly(test_input, expected_arima_anomaly):
    throughput_list, stddev = test_input
    anomaly_list = ad.calculate_arima_anomaly(throughput_list, stddev)
    assert anomaly_list == expected_arima_anomaly


expected_anomaly_list_ewma = [
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, True, True, True, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False]


@pytest.mark.parametrize(
    "test_input, expected_ewma_anomaly",
    [((throughput_list, stddev), expected_anomaly_list_ewma), ],
)
def test_calculate_ewma_anomaly(test_input, expected_ewma_anomaly):
    throughput_list, stddev = test_input
    anomaly_list = ad.calculate_ewma_anomaly(throughput_list, stddev)
    assert anomaly_list == expected_ewma_anomaly


expected_dbscan_anomaly_list = [
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, False, False,
    False, False, False, False, True, False,
    True, False, False, False, False, False,
    False, False, True, False, False, False,
    False, False, False, False, False, False,
    False, False, True, False, False, False,
    False, False, False, False, True, False]


@pytest.mark.parametrize(
    "test_input, expected_dbscan_anomaly",
    [((throughput_list, stddev),
      expected_dbscan_anomaly_list), ],
)
def test_calculate_dbscan_anomaly(test_input, expected_dbscan_anomaly):
    throughput_list, stddev = test_input
    anomaly_list = ad.calculate_dbscan_anomaly(throughput_list, stddev)
    assert anomaly_list == expected_dbscan_anomaly
