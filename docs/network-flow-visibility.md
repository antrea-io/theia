# Theia: Network Flow Visibility for Antrea

## Table of Contents

<!-- toc -->
- [Overview](#overview)
  - [Grafana Flow Collector](#grafana-flow-collector)
    - [Purpose](#purpose)
    - [About Grafana and ClickHouse](#about-grafana-and-clickhouse)
    - [Deployment Steps](#deployment-steps)
      - [Credentials Configuration](#credentials-configuration)
      - [ClickHouse Configuration](#clickhouse-configuration)
    - [Pre-built Dashboards](#pre-built-dashboards)
      - [Flow Records Dashboard](#flow-records-dashboard)
      - [Pod-to-Pod Flows Dashboard](#pod-to-pod-flows-dashboard)
      - [Pod-to-External Flows Dashboard](#pod-to-external-flows-dashboard)
      - [Pod-to-Service Flows Dashboard](#pod-to-service-flows-dashboard)
      - [Node-to-Node Flows Dashboard](#node-to-node-flows-dashboard)
      - [Network-Policy Flows Dashboard](#network-policy-flows-dashboard)
    - [Dashboards Customization](#dashboards-customization)
<!-- /toc -->

## Overview

[Antrea](https://github.com/antrea-io/antrea/blob/main/docs/design/architecture.md)
is a Kubernetes network plugin that provides network connectivity and security
features for Pod workloads. Considering the scale and dynamism of Kubernetes
workloads in a cluster, Network Flow Visibility helps in the management and
configuration of Kubernetes resources such as Network Policy, Services, Pods
etc., and thereby provides opportunities to enhance the performance and security
aspects of Pod workloads.

For visualizing the network flows, Antrea monitors the flows in Linux conntrack
module. These flows are converted to flow records, and then flow records are post-processed
before they are sent to the configured external flow collector.

### Grafana Flow Collector

#### Purpose

Antrea supports sending IPFIX flow records through the Flow Exporter and Flow Aggregator
feature described in [doc](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md).
The Grafana Flow Collector works as the visualization tool for flow records and
flow-related information. We use ClickHouse as the data storage, which collects flow
records data from the Flow Aggregator and load the data to Grafana. This document
provides the guidelines for deploying the Grafana Flow Collector with support for
Antrea-specific IPFIX fields in a Kubernetes cluster.

#### About Grafana and ClickHouse

[Grafana](https://grafana.com/grafana/) is an open-source platform for monitoring
and observability. Grafana allows you to query, visualize, alert on and understand
your metrics. [ClickHouse](https://clickhouse.com/) is an open-source, high performance
columnar OLAP database management system for real-time analytics using SQL. We use
ClickHouse as the data storage, and use Grafana as the data visualization and monitoring tool.

#### Deployment Steps

To deploy the Grafana Flow Collector, the first step is to install the ClickHouse
Operator, which creates, configures and manages ClickHouse clusters. Check the [homepage](https://github.com/Altinity/clickhouse-operator)
for more information about the ClickHouse Operator. Current checked-in yaml is based on their
[v0.18.2](https://github.com/Altinity/clickhouse-operator/blob/refs/tags/0.18.2/deploy/operator/clickhouse-operator-install-bundle.yaml) released version. Running the following command
will install ClickHouse Operator into `kube-system` Namespace.

```bash
kubectl apply -f https://raw.githubusercontent.com/antrea-io/theia/main/build/yamls/clickhouse-operator-install-bundle.yaml
```

To deploy a released version of the Grafana Flow Collector, find a deployment manifest
from the [list of releases](https://github.com/antrea-io/theia/releases).
For any given release <TAG> (v0.1.0 or later version), run the following command:

```bash
kubectl apply -f https://github.com/antrea-io/theia/releases/download/<TAG>/flow-visibility.yml
```

To deploy the latest version of the Grafana Flow Collector (built from the main branch),
use the checked-in [deployment yaml](/build/yamls/flow-visibility.yml):

```bash
kubectl apply -f https://raw.githubusercontent.com/antrea-io/theia/main/build/yamls/flow-visibility.yml
```

Grafana is exposed through a NodePort Service by default in `flow-visibility.yml`.
If the given K8s cluster supports LoadBalancer Services, Grafana can be exposed
through a LoadBalancer Service by changing the `grafana` Service type in the manifest
like below.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: flow-visibility
spec:
  ports:
  - port: 3000
    protocol: TCP
    targetPort: http-grafana
  selector:
    app: grafana
  sessionAffinity: None
  type: LoadBalancer
```

Please refer to the [Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1)
to learn about the ClickHouse configuration options.

Run the following command to check if ClickHouse and Grafana are deployed properly:

```bash
kubectl get all -n flow-visibility                                                               
```

The expected results will be like:

```bash  
NAME                                  READY   STATUS    RESTARTS   AGE
pod/chi-clickhouse-clickhouse-0-0-0   2/2     Running   0          1m
pod/grafana-5c6c5b74f7-x4v5b          1/1     Running   0          1m

NAME                                    TYPE           CLUSTER-IP       EXTERNAL-IP   PORT(S)                         AGE
service/chi-clickhouse-clickhouse-0-0   ClusterIP      None             <none>        8123/TCP,9000/TCP,9009/TCP      1m
service/clickhouse-clickhouse           ClusterIP      10.102.124.56    <none>        8123/TCP,9000/TCP               1m
service/grafana                         NodePort       10.97.171.150    <none>        3000:31171/TCP                  1m

NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/grafana   1/1     1            1           1m

NAME                                 DESIRED   CURRENT   READY   AGE
replicaset.apps/grafana-5c6c5b74f7   1         1         1       1m

NAME                                             READY   AGE
statefulset.apps/chi-clickhouse-clickhouse-0-0   1/1     1m

```

Run the following commands to print the IP of the workder Node and the NodePort
that Grafana is listening on:

```bash
NODE_NAME=$(kubectl get pod -l app=grafana -n flow-visibility -o jsonpath='{.items[0].spec.nodeName}')
NODE_IP=$(kubectl get nodes ${NODE_NAME} -o jsonpath='{.status.addresses[0].address}')
GRAFANA_NODEPORT=$(kubectl get svc grafana -n flow-visibility -o jsonpath='{.spec.ports[*].nodePort}')
echo "=== Grafana Service is listening on ${NODE_IP}:${GRAFANA_NODEPORT} ==="
```

You can now open the Grafana dashboard in the browser using `http://[NodeIP]:[NodePort]`.
You should be able to see a Grafana login page. Login credentials:

- username: admin
- password: admin

To stop the Grafana Flow Collector, run the following commands:

```shell
kubectl delete -f flow-visibility.yml
kubectl delete -f https://raw.githubusercontent.com/antrea-io/theia/main/build/yamls/clickhouse-operator-install-bundle.yml -n kube-system
```

##### Credentials Configuration

ClickHouse credentials are specified in `flow-visibility.yml` as a Secret named
`clickhouse-secret`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: clickhouse-secret
  namespace: flow-visibility
stringData:
  password: clickhouse_operator_password
  username: clickhouse_operator
type: Opaque
```

If the username `clickhouse_operator` has been changed, please
update the following section accordingly.

```yaml
apiVersion: "clickhouse.altinity.com/v1"
kind: "ClickHouseInstallation"
metadata:
  name: clickhouse
  labels:
    app: clickhouse
spec:
  configuration:
    users:
      # replace clickhouse_operator by [new_username]
      clickhouse_operator/k8s_secret_password: flow-visibility/clickhouse-secret/password
      clickhouse_operator/networks/ip: "::/0"
```

ClickHouse credentials are also specified in `flow-aggregator.yml` as a Secret
named `clickhouse-secret` as shown below. Please also make the corresponding changes.

```yaml
apiVersion: v1
kind: Secret
metadata:
  labels:
    app: flow-aggregator
  name: clickhouse-secret
  namespace: flow-aggregator
stringData:
  password: clickhouse_operator_password
  username: clickhouse_operator
type: Opaque
```

Grafana login credentials are specified in `flow-visibility.yml` as a Secret named
`grafana-secret`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: grafana-secret
  namespace: flow-visibility
stringData:
  admin-password: admin
  admin-username: admin
type: Opaque
```

We recommend changing all the credentials above if you are going to run the Flow
Collector in production.

##### ClickHouse Configuration

The ClickHouse database can be accessed through the Service `clickhouse-clickhouse`.
The Pod exposes HTTP port at 8123 and TCP port at 9000 by default. The ports are
specified in `flow-visibility.yml` as `serviceTemplates` of a `ClickHouseInstallation`
resource. To use other ports, please update the following section.

```yaml
serviceTemplates:
  - name: service-template
    spec:
      ports:
        - name: http
          port: 8123
        - name: tcp
          port: 9000
```

This Service is used by the Flow Aggregator and Grafana.

- If you have changed the HTTP port, please update the `url` of a ConfigMap named
`grafana-datasource-provider` in `flow-visibility.yml`.

- If you have changed the TCP port, please update the `databaseURL` following
[Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1),
and also update the `jsonData.port` of the `grafana-datasource-provider` ConfigMap.

```yaml
apiVersion: v1
data:
  datasource_provider.yml: |
    apiVersion: 1
    datasources:
      - name: ClickHouse
        type: grafana-clickhouse-datasource
        access: proxy
        url: http://clickhouse-clickhouse.flow-visibility.svc:8123
        editable: true
        jsonData:
          server: clickhouse-clickhouse.flow-visibility.svc
          port: 9000
          username: $CLICKHOUSE_USERNAME
        secureJsonData:
          password: $CLICKHOUSE_PASSWORD
kind: ConfigMap
metadata:
  name: grafana-datasource-provider-h868k56k95
  namespace: flow-visibility
```

The ClickHouse throughput depends on two factors - the storage size of the ClickHouse
and the time interval between the batch commits to the ClickHouse. Larger storage
size and longer commit interval provide higher throughput.

Grafana flow collector supports the ClickHouse in-memory deployment with limited
storage size. This is specified in `flow-visibility.yml` under the `clickhouse`
resource of kind: `ClickHouseInstallation`. The default value of storage size for
the ClickHouse server is 8 GiB. Users can expect a linear growth in the ClickHouse
throughput when they enlarge the storage size. For development or testing environment,
you can decrease the storage size to 2GiB. To deploy the ClickHouse with a different
storage size, please modify the `sizeLimit` in the following section.

```yaml
- emptyDir:
    medium: Memory
    sizeLimit: 8Gi
  name: clickhouse-storage-volume
```

The time interval between the batch commits to the ClickHouse is specified in the
[Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1)
as `commitInterval`. The ClickHouse throughput grows sightly when the commit interval
grows from 1s to 8s. A commit interval larger than 8s provides little improvement on the throughput.

#### Pre-built Dashboards

The following dashboards are pre-built and are recommended for Antrea flow
visualization. They can be found in the Home page of Grafana, by clicking
the Magnifier button on the left menu bar.
<img src="https://downloads.antrea.io/static/02152022/flow-visibility-grafana-intro-1.png" width="900" alt="Grafana Search Dashboards Guide">

Note that all pre-built dashboards (except for the "Flow Records Dashboard")
filter out Pod traffic for which the source or destination Namespace is one of
`kube-system`, `flow-visibility`, or `flow-aggregator`. The primary motivation
for this is to avoid showing the connections between the Antrea Agents and the
Flow Aggregator, between the Flow Aggregator and ClickHouse, and between
ClickHouse and Grafana. If you want to stop filtering traffic like this, you
will need to [customize dashboards](#dashboards-customization) and edit the
ClickHouse SQL query for each individual panel.

##### Flow Records Dashboard

Flow Records Dashboard displays the number of flow records being captured in the
selected time range. The detailed metadata of each of the records can be found
in the table below.  

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-flow-records-1.png" width="900" alt="Flow Records Dashboard">

Flow Records Dashboard provides time-range control. The selected time-range will
be applied to all the panels in the dashboard. This feature is also available for
all the other pre-built dashboards.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-flow-records-3.png" width="900" alt="Flow Records Dashboard">

Flow Records Dashboard allows us to add key/value filters that automatically apply
to all the panels in the dashboard. This feature is also available for all the
other pre-built dashboards.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-flow-records-2.png" width="900" alt="Flow Records Dashboard">

Besides the dashboard-wide filter, Flow Records Dashboard also provides column-based
filters that apply to each table column.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-flow-records-4.png" width="900" alt="Flow Records Dashboard">

##### Pod-to-Pod Flows Dashboard

Pod-to-Pod Flows Dashboard shows cumulative bytes and reverse bytes of Pod-to-Pod
traffic in the selected time range, in the form of Sankey diagram. Corresponding
source or destination Pod throughput is visualized using the line graphs. Pie charts
visualize the cumulative traffic grouped by source or destination Pod Namespace.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-pod-1.png" width="900" alt="Pod-to-Pod Flows Dashboard">

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-pod-2.png" width="900" alt="Pod-to-Pod Flows Dashboard">

##### Pod-to-External Flows Dashboard

Pod-to-External Flows Dashboard has similar visualization to Pod-to-Pod Flows
Dashboard, visualizing the Pod-to-External flows. The destination of a traffic
flow is represented by the destination IP address.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-external-1.png" width="900" alt="Pod-to-External Flows Dashboard">

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-external-2.png" width="900" alt="Pod-to-External Flows Dashboard">

##### Pod-to-Service Flows Dashboard

Pod-to-Service Flows Dashboard shares the similar visualizations with Pod-to-Pod/External
Flows Dashboard, visualizing the Pod-to-Service flows. The destination of a traffic
is represented by the destination Service metadata.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-service-1.png" width="900" alt="Pod-to-Service Flows Dashboard">

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-service-2.png" width="900" alt="Pod-to-Service Flows Dashboard">

##### Node-to-Node Flows Dashboard

Node-to-Node Flows Dashboard visualizes the Node-to-Node traffic, including intra-Node
and inter-Node flows. Cumulative bytes are shown in the Sankey diagrams and pie charts,
and throughput is shown in the line graphs.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-node-to-node-1.png" width="900" alt="Node-to-Node Flows Dashboard">

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-node-to-node-2.png" width="900" alt="Node-to-Node Flows Dashboard">

##### Network-Policy Flows Dashboard

Network-Policy Flows Dashboard visualizes the traffic with NetworkPolicies enforced.
Currently we only support the visualization of NetworkPolicies with `Allow` action.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-np-1.png" width="900" alt="Network-Policy Flows Dashboard">

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-np-2.png" width="900" alt="Network-Policy Flows Dashboard">

#### Dashboards Customization

If you would like to make any changes to any of the pre-built dashboards, or build
a new dashboard, please follow this [doc](https://grafana.com/docs/grafana/latest/dashboards/)
on how to build a dashboard.

By clicking on the "Save dashboard" button in the Grafana UI, the changes to the
dashboards will be persisted in the Grafana database at runtime, but they will be
lost after restarting the Grafana deployment. To restore those changes after a restart,
as the first step, you will need to export the dashboard JSON file following the
[doc](https://grafana.com/docs/grafana/latest/dashboards/export-import/), then there
are two ways to import the dashboard depending on your needs:

- In the running Grafana UI, manually import the dashboard JSON files.
- If you want the changed dashboards to be automatically provisioned in Grafana
like our pre-built dashboards, generate a deployment manifest with the changes by
following the steps below:

1. Clone the repository. Exported dashboard JSON files should be placed under `antrea/build/yamls/base/provisioning/dashboards`.
1. If a new dashboard is added, edit [kustomization.yml][flow_visibility_kustomization_yaml]
by adding the file in the following section:

    ```yaml
    - name: grafana-dashboard-config
      files:
      - provisioning/dashboards/flow_records_dashboard.json
      - provisioning/dashboards/pod_to_pod_dashboard.json
      - provisioning/dashboards/pod_to_service_dashboard.json
      - provisioning/dashboards/pod_to_external_dashboard.json
      - provisioning/dashboards/node_to_node_dashboard.json
      - provisioning/dashboards/networkpolicy_allow_dashboard.json
      - provisioning/dashboards/[new_dashboard_name].json
    ```

1. Generate the new YAML manifest by running:

```bash
./hack/generate-manifest.sh > build/yamls/flow-visibility.yml
```

[flow_visibility_kustomization_yaml]: ../build/yamls/base/kustomization.yml
