import unittest

from drop_detection_udf import *
    
class TestDropDetection(unittest.TestCase):
    detection_id = "88d98d56-a3a4-43ea-b4f8-8a231077c6a9"

    aggregated_flows = [
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-01',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-02',
            2
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-03',
            5
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-04',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-05',
            100
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-06',
            4
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-07',
            2
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-08',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-09',
            6
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-10',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-11',
            4
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-12',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-13',
            2
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-14',
            5
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-15',
            3
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-16',
            0
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-17',
            2
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-18',
            4
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-19',
            1
        ],
        [
            'antrea-test/Pod-A',
            'ingress',
            '2022-01-20',
            5
        ],
    ]

    expected_result = [
        [
            'antrea-test/Pod-A',
            'ingress',
            8.0,
            21.7037469479108,
            '2022-01-05',
            100
        ]
    ]


    def setup(self):
        self.drop_detection = DropDetection()

    def process_flows(self,
                      job_type="initial",
                      detection_id=detection_id,
                      flows=aggregated_flows):
        for flow in flows:
            next(self.drop_detection.process(
                job_type=job_type,
                detection_id=detection_id,
                endpoint=flow[0],
                direction=flow[1],
                date=flow[2],
                drop_number=flow[3]
            ))

    def test_end_partition(self):
        self.setup()
        self.process_flows()
        results = list(self.drop_detection.end_partition())
        self.assertEqual(len(self.expected_result), len(results))
        for expected_result, result in zip(self.expected_result, results):
            _, detection_id, _, endpoint, direction, avg_drop, stdev_drop, anomaly_drop_date, anomaly_drop_number = result
            self.assertEqual(self.detection_id, detection_id)
            self.assertEqual(expected_result[0], endpoint)
            self.assertEqual(expected_result[1], direction)
            self.assertEqual(expected_result[2], avg_drop)
            self.assertEqual(expected_result[3], stdev_drop)
            self.assertEqual(expected_result[4], anomaly_drop_date)
            self.assertEqual(expected_result[5], anomaly_drop_number)

if __name__ == "__main__":
    unittest.main()
