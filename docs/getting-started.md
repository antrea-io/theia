# Getting Started with Theia

## Table of Contents

<!-- toc -->
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Theia Installation](#theia-installation)
- [Features](#features)
  - [Network Flow Visualization and Monitoring](#network-flow-visualization-and-monitoring)
  - [NetworkPolicy Recommendation](#networkpolicy-recommendation)
- [Additional Information](#additional-information)
<!-- /toc -->

## Overview

Theia is a network observability and analytics platform for Kubernetes, built
on top of [Antrea](https://github.com/antrea-io/antrea). Theia consumes [network
flows exported by Antrea](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md)
to provide fine-grained visibility into the communication and NetworkPolicies
among Pods and Services in a Kubernetes cluster.

Theia supports network flow visualization and monitoring with Grafana, and can
recommend appropriate NetworkPolicy configuration to secure Kubernetes network
and applications. This guide describes how to install and get started with
Theia.

## Prerequisites

Theia requires that Antrea v1.7.0 or later is installed in the Kubernetes
cluster. Ensure the Flow Exporter feature of Antrea Agent is enabled in the
Antrea deployment manifest:

```yaml
  antrea-agent.conf: |
    ...
    featureGates:
      ...
      FlowExporter: true
```

## Theia Installation

To enable both [Grafana Flow Collector](network-flow-visibility.md) and
[NetworkPolicy Recommendation](networkpolicy-recommendation.md), please install
Flow Aggregator and Theia by runnning the following commands:

```bash
git clone https://github.com/antrea-io/theia.git
helm install flow-aggregator theia/build/charts/flow-aggregator --set clickHouse.enable=true,recordContents.podLabels=true -n flow-aggregator --create-namespace
helm install theia theia/build/charts/theia --set sparkOperator.enable=true -n flow-visibility --create-namespace
```

To enable only Grafana Flow Collector, please install Flow Aggregator and Theia
by runnning the following commands:

```bash
git clone https://github.com/antrea-io/theia.git
helm install flow-aggregator theia/build/charts/flow-aggregator --set clickHouse.enable=true -n flow-aggregator --create-namespace
helm install theia theia/build/charts/theia -n flow-visibility --create-namespace
```

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

You can access Grafana in your browser at: `http://[NodeIP]:[NodePort]`, and log
in with username: `admin` and password: `admin`. Navigate to the [Theia
dashboards](network-flow-visibility.md#grafana-dashboards) to view the network
flows in the cluster.

### NetworkPolicy Recommendation

Please follow the instructions in the [NetworkPolicy Recommendation](networkpolicy-recommendation.md)
user guide.

## Additional Information

Refer to Antrea documentation to learn more about
[Flow Exporter](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#flow-exporter),
[Flow Aggregator](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#flow-aggregator),
and their advanced configurations.

For more information about Grafana Flow Collector installation and
customization, please refer to Grafana Flow Collector [Deployment Steps](network-flow-visibility.md#deployment-steps),
and [Configuration](network-flow-visibility.md#configuration).
