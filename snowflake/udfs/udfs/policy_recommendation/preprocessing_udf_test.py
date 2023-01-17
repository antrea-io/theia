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
                    job_type="initial",
                    isolation_method=1,
                    ns_allow_list="kube-system,flow-aggregator,flow-visibility",
                    label_ignore_list="pod-template-hash,controller-revision-hash,pod-template-generation",
                    source_pod_namespace=flow_input[0],
                    source_pod_labels=flow_input[1],
                    destination_ip=flow_input[2],
                    destination_pod_namespace=flow_input[3],
                    destination_pod_labels=flow_input[4],
                    destination_service_port_name=flow_input[5],
                    destination_transport_port=flow_input[6],
                    protocol_identifier=flow_input[7],
                    flow_type=flow_input[8]
            )
            for flow_processed, expected_flow_processed in zip(process_result, expected_flows_processed):
                applied_to, ingress, egress = flow_processed
                self.assertEqual(applied_to, expected_flow_processed[0])
                self.assertEqual(ingress, expected_flow_processed[1])
                self.assertEqual(egress, expected_flow_processed[2])

if __name__ == "__main__":
    unittest.main()
