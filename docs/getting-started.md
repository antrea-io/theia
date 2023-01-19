# Getting Started with Theia

## Table of Contents

<!-- toc -->
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Theia Installation](#theia-installation)
- [Features](#features)
  - [Network Flow Visualization and Monitoring](#network-flow-visualization-and-monitoring)
  - [NetworkPolicy Recommendation](#networkpolicy-recommendation)
  - [Throughput Anomaly Detection](#throughput-anomaly-detection)
- [Additional Information](#additional-information)
<!-- /toc -->

## Overview

Theia is a network observability and analytics platform for Kubernetes, built
on top of [Antrea](https://github.com/antrea-io/antrea). Theia consumes
[network flows exported by Antrea](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md)
to provide fine-grained visibility into the communication and NetworkPolicies
among Pods and Services in a Kubernetes cluster.

Theia supports network flow visualization and monitoring with Grafana, and can
recommend appropriate NetworkPolicy configuration to secure Kubernetes network
and applications. This guide describes how to install and get started with
Theia.

## Prerequisites

Theia requires that Antrea v1.7.0 or later is installed in the Kubernetes
cluster.

For Antrea v1.7, please ensure the Flow Exporter feature of Antrea
Agent is enabled in the Antrea deployment manifest:

```yaml
  antrea-agent.conf: |
    ...
    featureGates:
      ...
      FlowExporter: true
```

From Antrea v1.8, you can deploy Antrea through Helm by running the following
commands:

```bash
helm repo add antrea https://charts.antrea.io
helm install antrea antrea/antrea -n kube-system --set featureGates.FlowExporter=true
```

This will install the latest available version of Antrea with the Flow Exporter
feature enabled. You can also install a specific version of Antrea (>= v1.8.0)
with `--version <TAG>`.

For more information about Antrea Helm chart, please refer to
[Antrea Helm chart installation instructions](https://github.com/antrea-io/antrea/blob/main/docs/helm.md).

## Theia Installation

Please install Flow Aggregator and Theia through Helm.

For Theia v0.1, please clone the repository and checkout branch `release-0.1`.
Both Helm charts are located under the folder `build/charts`.

From Theia v0.2 and Antrea v1.8, the Flow Aggregator Helm chart is moved from
Theia repository to Antrea repository; and the Helm charts are added to Antrea
Helm repo. Please add the repo by running the following command:

```bash
helm repo add antrea https://charts.antrea.io
helm repo update
```

To install Flow Aggregator, please run the following command:

```bash
helm install flow-aggregator antrea/flow-aggregator --set clickHouse.enable=true,recordContents.podLabels=true -n flow-aggregator --create-namespace
```

To enable [Grafana Flow Collector](network-flow-visibility.md),
[NetworkPolicy Recommendation](networkpolicy-recommendation.md) and
[Throughput Anomaly Detection](throughput-anomaly-detection.md), please install
Theia by running the following commands:

```bash
helm install theia antrea/theia --set sparkOperator.enable=true -n flow-visibility --create-namespace
```

From Theia v0.3, [Theia Command-line Tool](theia-cli.md) uses Theia Manager
as a layer to connect and manage other resources like ClickHouse and Spark
Operator. To enable Theia Manager, please install Theia by running the
following commands:

```bash
helm install theia antrea/theia --set sparkOperator.enable=true,theiaManager.enable=true -n flow-visibility --create-namespace
```

To enable only Grafana Flow Collector, please install Theia by running the
following commands:

```bash
helm install theia antrea/theia -n flow-visibility --create-namespace
```

These will install the latest available versions of Flow Aggregator and Theia.
You can also install specific versions of Flow Aggregator (>= v1.8.0) and
Theia (>= v0.2.0) with `--version <TAG>`. Please ensure that you use the same
released version for the Flow Aggregator chart as for the Antrea chart.

## Features

### Network Flow Visualization and Monitoring

Theia uses Grafana to visualize network flows in the Kubernetes cluster. After
the installation, you can run the following commands to get the Grafana Service
address:

```bash
NODE_NAME=$(kubectl get pod -l app=grafana -n flow-visibility -o jsonpath='{.items[0].spec.nodeName}')
NODE_IP=$(kubectl get nodes ${NODE_NAME} -o jsonpath='{.status.addresses[0].address}')
GRAFANA_NODEPORT=$(kubectl get svc grafana -n flow-visibility -o jsonpath='{.spec.ports[*].nodePort}')
echo "=== Grafana Service is listening on ${NODE_IP}:${GRAFANA_NODEPORT} ==="
```

You can access Grafana in your browser at: `http://[NodeIP]:[NodePort]`,
and log in with username: `admin` and password: `admin`. Navigate to the [Theia
dashboards](network-flow-visibility.md#grafana-dashboards) to view the network
flows in the cluster.

### NetworkPolicy Recommendation

Please follow the instructions in the [NetworkPolicy Recommendation](networkpolicy-recommendation.md)
user guide.

### Throughput Anomaly Detection

Please follow the instructions in the [Throughput Anomaly Detection](throughput-anomaly-detection.md)
user guide.

## Additional Information

Refer to Antrea documentation to learn more about
[Flow Exporter](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#flow-exporter),
[Flow Aggregator](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#flow-aggregator),
and their advanced configurations.

For more information about Grafana Flow Collector installation and
customization, please refer to Grafana Flow Collector [Deployment Steps](network-flow-visibility.md#deployment-steps),
and [Configuration](network-flow-visibility.md#configuration).
