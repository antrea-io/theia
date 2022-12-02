import unittest

from preprocessing_udf import *

class TestStaticPolicyRecommendation(unittest.TestCase):
    flows_input = [
        (
            "antrea-test",
            "{\"podname\":\"perftest-a\"}",
            "10.10.0.5",
            "antrea-test",
            "{\"podname\":\"perftest-b\"}",
            "",
            5201,
            6,
            1
        ),
        (
            "antrea-test",
            "{\"podname\":\"perftest-a\"}",
            "10.10.0.6",
            "antrea-test",
            "{\"podname\":\"perftest-c\"}",
            "antrea-e2e/perftestsvc:5201",
            5201,
            6,
            1
        ),
        (
            "antrea-test",
            "{\"podname\":\"perftest-a\"}",
            "192.168.0.1",
            "",
            "",
            "",
            80,
            6,
            3
        )
    ]

    flows_processed = [
        [
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-test#{"podname": "perftest-b"}#5201#TCP'
            ],
            [
                'antrea-test#{"podname": "perftest-b"}',
                'antrea-test#{"podname": "perftest-a"}#5201#TCP',
                ''
            ]
        ],
        [
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                'antrea-e2e#perftestsvc'
            ],
            [
                'antrea-test#{"podname": "perftest-c"}',
                'antrea-test#{"podname": "perftest-a"}#5201#TCP',
                ''
            ]
        ],
        [
            [
                'antrea-test#{"podname": "perftest-a"}',
                '',
                '192.168.0.1#80#TCP'
            ],
        ],
    ]

    def setup(self):
        self.preprocessing = PreProcessing()

    def test_process(self):
        self.setup()
        for flow_input, expected_flows_processed in zip(self.flows_input, self.flows_processed):
            process_result = self.preprocessing.process(
                    jobType="initial",
                    isolationMethod=1,
                    nsAllowList="kube-system,flow-aggregator,flow-visibility",
                    labelIgnoreList="pod-template-hash,controller-revision-hash,pod-template-generation",
                    sourcePodNamespace=flow_input[0],
                    sourcePodLabels=flow_input[1],
                    destinationIP=flow_input[2],
                    destinationPodNamespace=flow_input[3],
                    destinationPodLabels=flow_input[4],
                    destinationServicePortName=flow_input[5],
                    destinationTransportPort=flow_input[6],
                    protocolIdentifier=flow_input[7],
                    flowType=flow_input[8]
            )
            for flow_processed, expected_flow_processed in zip(process_result, expected_flows_processed):
                applied_to, ingress, egress = flow_processed
                self.assertEqual(applied_to, expected_flow_processed[0])
                self.assertEqual(ingress, expected_flow_processed[1])
                self.assertEqual(egress, expected_flow_processed[2])

if __name__ == "__main__":
    unittest.main()
