import unittest
import random

from traffic_drop_udf import *
    
class TestTrafficDrop(unittest.TestCase):
    detectionID = "88d98d56-a3a4-43ea-b4f8-8a231077c6a9"

    flows_aggregated = [
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-01',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-02',
            2
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-03',
            5
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-04',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-05',
            100
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-06',
            4
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-07',
            2
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-08',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-09',
            6
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-10',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-11',
            4
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-12',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-13',
            2
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-14',
            5
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-15',
            3
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-16',
            0
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-17',
            2
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-18',
            4
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-19',
            1
        ],
        [
            'antrea-test#"Pod-A"',
            'ingress',
            '2022-01-20',
            5
        ],
    ]

    expected_result = [
        [
            'antrea-test#"Pod-A"',
            'ingress',
            8.0,
            21.7037469479108,
            '2022-01-05',
            100
        ]
    ]


    def setup(self):
        self.traffic_drop = TrafficDrop()

    def process_flows(self,
                      jobType="initial",
                      detectionID=detectionID,
                      flows=flows_aggregated):
        for flow in flows:
            next(self.traffic_drop.process(
                jobType=jobType,
                detectionID=detectionID,
                endpoint=flow[0],
                direction=flow[1],
                date=flow[2],
                dropNumber=flow[3]
            ))

    def test_end_partition(self):
        self.setup()
        self.process_flows()
        results = list(self.traffic_drop.end_partition())
        self.assertEqual(len(self.expected_result), len(results))
        for expected_result, result in zip(self.expected_result, results):
            _, detectionID, _, endpoint, direction, avgDrop, stdevDrop, anomalyDropDate, anomalyDropNumber = result
            self.assertEqual(self.detectionID, detectionID)
            self.assertEqual(expected_result[0], endpoint)
            self.assertEqual(expected_result[1], direction)
            self.assertEqual(expected_result[2], avgDrop)
            self.assertEqual(expected_result[3], stdevDrop)
            self.assertEqual(expected_result[4], anomalyDropDate)
            self.assertEqual(expected_result[5], anomalyDropNumber)

if __name__ == "__main__":
    unittest.main()
