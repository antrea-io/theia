# Theia

Theia is a network observability and analytics platform for Kubernetes. It is
built on top of [Antrea](https://github.com/antrea-io/antrea), and consumes
[network flows exported by Antrea](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md)
to provide fine-grained visibility into the communication and NetworkPolicies
among Pods and Services in a Kubernetes cluster.

## Getting Started

Getting started with Theia is simple. You can follow the [Getting Started](docs/getting-started.md)
guide to install Theia and start rocking!

Theia supports network flow visualization and monitoring with Grafana. Check the
[Network Flow Visibility](docs/network-flow-visibility.md) document to learn
more.

Based on the collected network flow information, Theia can recommend appropriate
NetworkPolicy configuration to secure Kubernetes network and applications.
Please refer to the [NetworkPolicy Recommendation](docs/networkpolicy-recommendation.md)
user guide to learn more.

Theia also provides throughput anomaly detection, it can find the anomalies
in the network, and report them to the user.
Please refer to the
[Throughput Anomaly Detection](docs/throughput-anomaly-detection.md) user
guide to learn more.

## Contributing

The Antrea community welcomes new contributors. We are waiting for your PRs!

* Before contributing, please get familiar with our
[Code of Conduct](CODE_OF_CONDUCT.md).
* Check out the Antrea [Contributor Guide](CONTRIBUTING.md) for information
about setting up your development environment and our contribution workflow.
* Learn about Antrea's [Architecture and Design](https://github.com/antrea-io/antrea/blob/main/docs/design/architecture.md).
Your feedback is more than welcome!
* Check out [Open Issues](https://github.com/antrea-io/theia/issues).
* Join the Antrea [community](#community) and ask us any question you may have.

## Community

Please refer to the [Antrea community](https://github.com/antrea-io/antrea/blob/main/README.md#community)
information.

## License

Theia is licensed under the [Apache License, version 2.0](LICENSE)
