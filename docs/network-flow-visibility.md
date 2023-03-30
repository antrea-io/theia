# Network Flow Visibility with Theia

## Table of Contents

<!-- toc -->
- [Overview](#overview)
- [Grafana Flow Collector](#grafana-flow-collector)
  - [Purpose](#purpose)
  - [About Grafana and ClickHouse](#about-grafana-and-clickhouse)
  - [Deployment Steps](#deployment-steps)
  - [Configuration](#configuration)
    - [With Helm](#with-helm)
      - [ClickHouse Cluster](#clickhouse-cluster)
    - [With Standalone Manifest](#with-standalone-manifest)
      - [Grafana Configuration](#grafana-configuration)
        - [Service Customization](#service-customization)
        - [Credentials Configuration](#credentials-configuration)
      - [ClickHouse Configuration](#clickhouse-configuration)
        - [Credentials Configuration](#credentials-configuration-1)
        - [Service Customization](#service-customization-1)
        - [Performance Configuration](#performance-configuration)
        - [Persistent Volumes](#persistent-volumes)
- [Grafana Dashboards](#grafana-dashboards)
  - [Home Dashboard](#home-dashboard)
  - [Pre-built Dashboards](#pre-built-dashboards)
    - [Flow Records Dashboard](#flow-records-dashboard)
    - [Pod-to-Pod Flows Dashboard](#pod-to-pod-flows-dashboard)
    - [Pod-to-External Flows Dashboard](#pod-to-external-flows-dashboard)
    - [Pod-to-Service Flows Dashboard](#pod-to-service-flows-dashboard)
    - [Node-to-Node Flows Dashboard](#node-to-node-flows-dashboard)
    - [Network-Policy Flows Dashboard](#network-policy-flows-dashboard)
    - [Network Topology Dashboard](#network-topology-dashboard)
  - [Dashboard Customization](#dashboard-customization)
<!-- /toc -->

## Overview

Theia is a network observability and analytics platform for Kubernetes, built
on top of [Antrea](https://github.com/antrea-io/antrea). It supports network
flow visualization and monitoring with Grafana. This document describes the
Grafana Flow Collector and network flow visualization functionality of Theia.

## Grafana Flow Collector

### Purpose

Antrea supports exporting IPFIX flow records with Kubernetes workload and
NetworkPolicy information and sending them to a flow collector, through the Flow
Exporter and Flow Aggregator features as described in the [Antrea Network Flow
Visibility](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md)
document. The Grafana Flow Collector works as the visualization and monitoring
tool for flow records and flow-related information. It uses ClickHouse as the
data storage, which collects flow records data from the Flow Aggregator and
loads the data to Grafana. This document provides the guidelines for deploying
the Grafana Flow Collector with support for Antrea-specific IPFIX fields in a
Kubernetes cluster.

### About Grafana and ClickHouse

[Grafana](https://grafana.com/grafana/) is an open-source platform for monitoring
and observability. Grafana allows you to query, visualize, alert on and understand
your metrics. [ClickHouse](https://clickhouse.com/) is an open-source, high performance
columnar OLAP database management system for real-time analytics using SQL. We use
ClickHouse as the data storage, and use Grafana as the data visualization and monitoring tool.

### Deployment Steps

This section talks about detailed steps to deploy the Grafana Flow Collector
using Helm charts or YAML manifests. For quick steps of installing Theia
including the Grafana Flow Collector, please refer to the [Getting Started](getting-started.md)
guide.

We support deploying the Grafana Flow Collector with Helm. Here is the
[Helm chart](../build/charts/theia/) for the Grafana Flow Collector. Please follow
the instructions from the Helm chart [README](../build/charts/theia/README.md)
to customize the installation.

You can clone the repository and run the following command to install the Grafana
Flow Collector into Namespace `flow-visibility`. See [helm install](https://helm.sh/docs/helm/helm_install/)
for command documentation.

```bash
helm install -f values.yaml theia build/charts/theia -n flow-visibility --create-namespace
```

We recommend using Helm to deploy the Grafana Flow Collector. But if you prefer
not to clone the repository, you can mannually deploy it. The first step is to
install the ClickHouse Operator, which creates, configures and manages ClickHouse
clusters. Check the [homepage](https://github.com/Altinity/clickhouse-operator)
for more information about the ClickHouse Operator. Current checked-in yaml is
based on their [v0.18.2](https://github.com/Altinity/clickhouse-operator/blob/refs/tags/0.18.2/deploy/operator/clickhouse-operator-install-bundle.yaml)
released version. Running the following command will install ClickHouse Operator
into `kube-system` Namespace.

```bash
kubectl apply -f https://raw.githubusercontent.com/antrea-io/theia/main/build/charts/theia/crds/clickhouse-operator-install-bundle.yaml
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

Run the following command to check if ClickHouse and Grafana are deployed properly:

```bash
kubectl get all -n flow-visibility
```

The expected results will be like:

```bash  
NAME                                  READY   STATUS    RESTARTS      AGE
pod/chi-clickhouse-clickhouse-0-0-0   2/2     Running   1 (19m ago)   19m
pod/grafana-7d8c44f55-lhrcc           1/1     Running   0             19m
pod/zookeeper-0                       1/1     Running   0             19m

NAME                                    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
service/chi-clickhouse-clickhouse-0-0   ClusterIP   None            <none>        8123/TCP,9000/TCP,9009/TCP   19m
service/clickhouse-clickhouse           ClusterIP   10.99.235.79    <none>        8123/TCP,9000/TCP            19m
service/grafana                         NodePort    10.99.215.177   <none>        3000:30914/TCP               19m
service/zookeeper                       ClusterIP   10.106.196.44   <none>        2181/TCP,7000/TCP            19m
service/zookeepers                      ClusterIP   None            <none>        2888/TCP,3888/TCP            19m

NAME                      READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/grafana   1/1     1            1           19m

NAME                                DESIRED   CURRENT   READY   AGE
replicaset.apps/grafana-7d8c44f55   1         1         1       19m

NAME                                             READY   AGE
statefulset.apps/chi-clickhouse-clickhouse-0-0   1/1     19m
statefulset.apps/zookeeper                       1/1     19m

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

From Theia 0.2, Theia supports ClickHouse clustering in lieu of the non-clustered
mode supported in v0.1. To stop the Grafana Flow Collector, please run the
following command first. Refer to [ClickHouse Cluster](#clickhouse-cluster) for more
information.

```bash
kubectl delete clickhouseinstallation.clickhouse.altinity.com clickhouse -n flow-visibility
```

To stop the Grafana Flow Collector, run the following commands if you deploy it
by Helm:

```bash
helm uninstall theia -n flow-visibility
kubectl delete namespace flow-visibility
kubectl delete -f https://raw.githubusercontent.com/antrea-io/theia/main/build/charts/theia/crds/clickhouse-operator-install-bundle.yaml -n kube-system
```

Run the following commands if you deploy it by the generated manifest available
online:

```bash
kubectl delete -f flow-visibility.yml
kubectl delete -f https://raw.githubusercontent.com/antrea-io/theia/main/build/charts/theia/crds/clickhouse-operator-install-bundle.yaml -n kube-system
```

### Configuration

So far, we went through Grafana Flow Collector deployment with the default
configuration. The Grafana Flow Collector also exposes a few configuration
parameters for you to customize the deployment. Read this section to learn more.

#### With Helm

If you install the Grafana Flow Collector using the Helm command, please refer
to Helm chart [README](../build/charts/theia/README.md) or run the following
commands to see configurable options with descriptions and default values.

```bash
helm show values build/charts/theia
```

You can override any of these settings in a YAML formatted file, and then pass
that file during installation or upgrading.

```bash
helm install -f values.yaml theia build/charts/theia -n flow-visibility --create-namespace
helm upgrade -f values.yaml theia build/charts/theia -n flow-visibility
```

Or you can specify each parameter using the `--set key=value[,key=value]`
argument and run the following commands.

```bash
helm install theia build/charts/theia -n flow-visibility --create-namespace \
  --set=clickhouse.storageSize="2Gi"
```

The ClickHouse TCP Service is used by the Flow Aggregator. If you have changed
the TCP port, please update the `clickHouse.databaseURL` in the Flow Aggregator
Helm Chart values.

The ClickHouse credentials are used in the Flow Aggregator. If you have changed
the ClickHouse credentials, please update the `clickHouse.connectionSecret` in the
Flow Aggregator Helm Chart values.

Please refer to the Flow Aggregator Helm Chart
[README](https://github.com/antrea-io/antrea/blob/main/build/charts/flow-aggregator/README.md)
and [Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1)
to learn more about ClickHouse configuration options in Flow Aggregator.

##### ClickHouse Cluster

In v0.1, Theia deploys ClickHouse in a non-cluster mode. Starting with v0.2,
Theia supports ClickHouse clustering in lieu of the non-clustered mode supported
in v0.1. You can upgrade from v0.1 to v0.2 without data loss if
you use PersistentVolume for ClickHouse. To downgrade from v0.2 to v0.1, you
need to uninstall ClickHouse, delete related data if you use PersistenVolume
and redeploy it.

In v0.3, we add a breaking change to ClickHouse data migration. To downgrade
from a version after v0.3 to a version before v0.3 without data lossing, please
first downgrade it to v0.3 and then to the version below v0.3.

A ClickHouse cluster consists of one or more shards. Shards refer to the servers
that contain different parts of the data. You can deploy multiple shards to scale
the cluster horizontally. Each shard consists of one or more replica hosts.
Replicas indicate storing the same data on multiple Pods to improve data
availability and accessibility. You can deploy multiple replicas in a shard to
ensure reliability.

By default, Theia deploys a 1-shard-1-replica ClickHouse cluster and a 1-replica
ZooKeeper. To scale the ClickHouse cluster, please set `clickhouse.cluster.shards`
and `clickhouse.cluster.replicas` per your requirement.

ClickHouse uses [Apache ZooKeeper](https://zookeeper.apache.org/) for storing
replicas meta information. When scaling the ClickHouse cluster, please set
`clickhouse.cluster.installZookeeper.replicas` to at least 3 to ensure fault
tolerance. Each replica is expected to be deployed on a different Node. To
leverage a user-provided ZooKeeper cluster, please refer to the
[ZooKeeper setup instructions for ClickHouse](https://github.com/Altinity/clickhouse-operator/blob/master/docs/zookeeper_setup.md)
and set `clickhouse.cluster.zookeeperHosts` to your ZooKeeper hosts.

When stopping the ClickHouse cluster, please ensure it is stopped before ZooKeeper,
as ClickHouse relies on ZooKeeper to stop correctly. You can run the following
command to stop the ClickHouse cluster.

```bash
kubectl delete clickhouseinstallation.clickhouse.altinity.com clickhouse -n flow-visibility
```

The default affinity allows only one ClickHouse instance per Node. Each replica
is expected to be deployed on a different Node with this affinity. To change the
affinity, please set `clickhouse.cluster.podDistribution` per your requirement.

We recommend using Persistent Volumes if you are going to use ClickHouse cluster
in production. ClickHouse cluster with in memory deployment is mainly for test
purpose, as when a Pod accidentally restarts, the memory will be cleared, which
will lead to a complete data loss. In that case, please refer to the
[ClickHouse document](https://clickhouse.com/docs/en/engines/table-engines/mergetree-family/replication/#recovery-after-complete-data-loss)
to recover the data.

ClickHouse cluster can be deployed with default Local PV or NFS PV by setting
`clickhouse.storage.createPersistentVolume`. To have more flexibility in the
PV creation, you can configure a customized `StorageClass` in
`clickhouse.storage.persistentVolumeClaimSpec`.

#### With Standalone Manifest

If you deploy the Grafana Flow Collector with `flow-visibility.yml`, please
follow the instructions below to customize the configurations.

##### Grafana Configuration

###### Service Customization

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

###### Credentials Configuration

Grafana login credentials are specified in `flow-visibility.yml` as a Secret
named `grafana-secret`.

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

We recommend changing the credentials if you are going to run the Flow Collector
in production.

##### ClickHouse Configuration

###### Credentials Configuration

ClickHouse credentials are also specified in `flow-aggregator.yml` as a Secret
named `clickhouse-secret` as shown below. Please also make the corresponding
changes.

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

If the username `clickhouse_operator` has been changed in `flow-visibility.yml`,
please update the following section accordingly.

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

Please refer to [the section above](#with-helm) on how to make corresponding
changes of the Clickhouse credentials in the Flow Aggregator.

We recommend changing the credentials if you are going to run the Flow Collector
in production.

###### Service Customization

The ClickHouse database is exposed by a ClusterIP Service by default in
`flow-visibility.yml`. The Pod exposes HTTP port at 8123 and TCP port at 9000
by default. The Service type and ports are specified in `flow-visibility.yml`
as `serviceTemplates` of a `ClickHouseInstallation` resource. To use other
Service type and ports, please update the following section.

```yaml
serviceTemplates:
  - name: service-template
    spec:
      ports:
        - name: http
          port: 8123
        - name: tcp
          port: 9000
      type: ClusterIP
```

This Service is used by the Flow Aggregator and Grafana.

- If you have changed the HTTP port, please update the `url` of a ConfigMap named
`grafana-datasource-provider` in `flow-visibility.yml`.

- If you have changed the TCP port, please update the `databaseURL` following
[Flow Aggregator Configuration](#configuration-1), and also update the `jsonData.port`
of the `grafana-datasource-provider` ConfigMap.

```yaml
apiVersion: v1
data:
  datasource_provider.yaml: |-
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
  name: grafana-datasource-provider
  namespace: flow-visibility
```

###### Performance Configuration

The ClickHouse throughput depends on two factors - the storage size of the ClickHouse
and the time interval between the batch commits to the ClickHouse. Larger storage
size and longer commit interval provide higher throughput.

Grafana flow collector supports the ClickHouse in-memory deployment with limited
storage size. This is specified in `flow-visibility.yml` under the `clickhouse`
resource of kind: `ClickHouseInstallation`. The default value of storage size for
the ClickHouse server is 8 GiB. The ClickHouse insertion rate is at around 4,000
records per seconds with this default storage size. Users can expect a linear
growth in the ClickHouse throughput when they enlarge the storage size. For
development or testing environment, you can decrease the storage size to 2 GiB.
To deploy the ClickHouse with a different storage size, please modify the
`sizeLimit` in the following section.

```yaml
- emptyDir:
    medium: Memory
    sizeLimit: 8Gi
  name: clickhouse-storage-volume
```

To deploy ClickHouse with Persistent Volumes and limited storage size, please refer
to [Persistent Volumes](#persistent-volumes).

The time interval between the batch commits to the ClickHouse is specified in the
[Flow Aggregator Configuration](https://github.com/antrea-io/antrea/blob/main/docs/network-flow-visibility.md#configuration-1)
as `commitInterval`. The ClickHouse throughput grows sightly when the commit interval
grows from 1s to 8s. A commit interval larger than 8s provides little improvement on the throughput.

###### Persistent Volumes

By default, ClickHouse is deployed in memory. We support deploying ClickHouse with
Persistent Volumes.

[PersistentVolume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
(PV) is a piece of storage in the K8s cluster, which requires to be manually
provisioned by an administrator or dynamically provisioned using Storage Classes.
A PersistentVolumeClaim (PVC) is a request for storage which consumes PV. As
ClickHouse is deployed as a StatefulSet, the volume can be claimed using
`volumeClaimTemplate`.

Please follow the steps below to deploy the ClickHouse with Persistent Volumes:

1. Provision the PersistentVolume. K8s supports a great number of
[PersistentVolume types](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#types-of-persistent-volumes).
You can provision your own PersistentVolume per your requirements. Here are
two simple examples for your reference.

    Before creating the PV manually, you need to create a `StorageClass` shown
    in the section below.

      ```yaml
      apiVersion: storage.k8s.io/v1
      kind: StorageClass
      metadata:
        name: clickhouse-storage
      provisioner: kubernetes.io/no-provisioner
      volumeBindingMode: WaitForFirstConsumer
      reclaimPolicy: Retain
      allowVolumeExpansion: True
      ```

    After the `StorageClass` is created, you can create a Local PV or a NFS PV
    by following the steps below.

    - Local PV allows you to store the ClickHouse data at a pre-defined path on
    a specific Node. Refer to the section below to create the PV. Please replace
    `LOCAL_PATH` with the path to store the ClickHouse data and label the Node
    used to store the ClickHouse data with `antrea.io/clickhouse-data-node=`.

      ```yaml
      apiVersion: v1
      kind: PersistentVolume
      metadata:
        name: clickhouse-pv
      spec:
        storageClassName: clickhouse-storage
        capacity:
          storage: 8Gi
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        local:
          path: LOCAL_PATH
        nodeAffinity:
          required:
            nodeSelectorTerms:
            - matchExpressions:
              - key: antrea.io/clickhouse-data-node
                operator: Exists
      ```

    - NFS PV allows you to store the ClickHouse data on an existing NFS server.
    Refer to the section below to create the PV. Please replace `NFS_SERVER_ADDRESS`
    with the host name of the NFS server and `NFS_SERVER_PATH` with the exported
    path on the NFS server.

      ```yaml
      apiVersion: v1
      kind: PersistentVolume
      metadata:
        name: clickhouse-pv
      spec:
        storageClassName: clickhouse-storage
        capacity:
          storage: 8Gi
        accessModes:
          - ReadWriteOnce
        volumeMode: Filesystem
        nfs:
          path: NFS_SERVER_PATH
          server: NFS_SERVER_ADDRESS
      ```

    In both examples, you can set `.spec.capacity.storage` in PersistentVolume
    to your storage size. This value is for informative purpose as K8s does not
    enforce the capacity of PVs. If you want to limit the storage usage, you need
    to ask for your storage system to enforce that. For example, you can create
    a Local PV on a partition with the limited size. We recommend using a dedicated
    saving space for the ClickHouse if you are going to run the Flow Collector in
    production.

    As these examples do not use any dynamic provisioner, the reclaim policy
    for the PVs is `Retain` by default. After stopping the Grafana Flow Collector,
    if you no long need the data for future use, you may need to manually clean
    up the data on the local disk or NFS server.

1. Request the PV for ClickHouse. Please add a `volumeClaimTemplate` section
under `.spec.templates` for the resource `ClickHouseInstallation` in
`flow-visibility.yml` as shown in the example below. `storageClassName` should
be set to your own `StorageClass` name, and `.resources.requests.storage`
should be set to your storage size.

    ```yaml
    volumeClaimTemplates:
    - name: clickhouse-storage-template
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 8Gi
        storageClassName: clickhouse-storage
    ```

    Then add this template as `dataVolumeClaimTemplate` to the section below.

    ```yaml
    defaults:
      templates:
        dataVolumeClaimTemplate: clickhouse-storage-template
        podTemplate: pod-template
        serviceTemplate: service-template
    ```

1. Remove the in-memory related deployment options, by removing the appropriate
`volume` and `volumeMount` for the `ClickHouseInstallation` resource in
`flow-visibility.yml`.

    The `volumeMounts` entry to be removed is the following one:

    ```yaml
    - mountPath: /var/lib/clickhouse
      name: clickhouse-storage-volume
    ```

    The `volumes` entry to be removed is the following one:

    ```yaml
    - emptyDir:
        medium: Memory
        sizeLimit: 8Gi
      name: clickhouse-storage-volume
    ```

## Grafana Dashboards

### Home Dashboard

Home dashboard is the first dashboard you will see after login to Grafana. It
provides an overview of the monitored Kubernetes cluster, introduction of
Theia flow visualization, and links to the other pre-built dashboards for
more specific flow visualization.

<img src="https://downloads.antrea.io/static/08242022/flow-visibility-homepage.png" width="900" alt="Grafana Homepage">

### Pre-built Dashboards

The dashboards listed in the `Dashboard Links` panel are pre-built and are
recommended for Theia flow visualization.

- Note that all pre-built dashboards (except for the "Flow Records Dashboard")
filter out Pod traffic for which the source or destination Namespace is one of
`kube-system`, `flow-visibility`, or `flow-aggregator`. The primary motivation
for this is to avoid showing the connections between the Antrea Agents and the
Flow Aggregator, between the Flow Aggregator and ClickHouse, and between
ClickHouse and Grafana. If you choose to install the Grafana Flow Collector
into a different Namespace other than `flow-visibility`, to filter out Pod
traffic in that Namespace, you will need to edit the query to replace
`flow-visibility` by the Namespace you choose.

- Also note that we limit the number of values displayed on panels. For table
panel on the Flow Records Dashboard, the limit is set to 10000. For Sankey
diagrams, Chord diagrams, and time-series line graphs, the limit is 50. For
pie charts, the limit is 25. The motivation is, when the input data is very
large, we want to keep the charts readable and avoid consuming too much time
and resources to render them.

If you want to stop filtering traffic by Namespace, or edit the panel limit,
you will need to edit the ClickHouse SQL query for each panel. Please follow
the [Dashboard Customization](#dashboard-customization) section for more
information. As a special case, to edit the panel limit for pie charts, instead
of editing the query, please follow the
[doc](https://grafana.com/docs/grafana/latest/visualizations/pie-chart-panel/#limit)
to edit `Value options - Limit`.

#### Flow Records Dashboard

Flow Records Dashboard displays the number of flow records being captured in the
selected time range. The detailed metadata of each of the records can be found
in the table below.  

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-flow-records-1.png" width="900" alt="Flow Records Dashboard">

Flow Records Dashboard provides time-range control. The selected time-range will
be applied to all the panels in the dashboard. This feature is also available for
all the other pre-built dashboards.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-flow-records-3.png" width="900" alt="Flow Records Dashboard">

Flow Records Dashboard allows us to add key/value filters that automatically apply
to all the panels in the dashboard. This feature is also available for all the
other pre-built dashboards.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-flow-records-2.png" width="900" alt="Flow Records Dashboard">

Besides the dashboard-wide filter, Flow Records Dashboard also provides column-based
filters that apply to each table column.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-flow-records-4.png" width="900" alt="Flow Records Dashboard">

#### Pod-to-Pod Flows Dashboard

Pod-to-Pod Flows Dashboard shows cumulative bytes and reverse bytes of Pod-to-Pod
traffic in the selected time range, in the form of Sankey diagram. Corresponding
source or destination Pod throughput is visualized using the line graphs. Pie charts
visualize the cumulative traffic grouped by source or destination Pod Namespace.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-pod-1.png" width="900" alt="Pod-to-Pod Flows Dashboard">

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-pod-to-pod-2.png" width="900" alt="Pod-to-Pod Flows Dashboard">

#### Pod-to-External Flows Dashboard

Pod-to-External Flows Dashboard has similar visualization to Pod-to-Pod Flows
Dashboard, visualizing the Pod-to-External flows. The destination of a traffic
flow is represented by the destination IP address.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-pod-to-external-1.png" width="900" alt="Pod-to-External Flows Dashboard">

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-pod-to-external-2.png" width="900" alt="Pod-to-External Flows Dashboard">

#### Pod-to-Service Flows Dashboard

Pod-to-Service Flows Dashboard shares the similar visualizations with Pod-to-Pod/External
Flows Dashboard, visualizing the Pod-to-Service flows. The destination of a traffic
is represented by the destination Service metadata.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-pod-to-service-1.png" width="900" alt="Pod-to-Service Flows Dashboard">

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-pod-to-service-2.png" width="900" alt="Pod-to-Service Flows Dashboard">

#### Node-to-Node Flows Dashboard

Node-to-Node Flows Dashboard visualizes the Node-to-Node traffic, including intra-Node
and inter-Node flows. Cumulative bytes are shown in the Sankey diagrams and pie charts,
and throughput is shown in the line graphs.

<img src="https://downloads.antrea.io/static/02152022/flow-visibility-node-to-node-1.png" width="900" alt="Node-to-Node Flows Dashboard">

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-node-to-node-2.png" width="900" alt="Node-to-Node Flows Dashboard">

#### Network-Policy Flows Dashboard

Network-Policy Flows Dashboard visualizes the traffic with NetworkPolicies enforced.
The Chord diagram visualizes the cumulative bytes of all the traffic in the selected
time range. We use green color to highlight traffic with "Allow" NetworkPolicy
enforced, red color to highlight traffic with "Deny" NetworkPolicy enforced.
Specifically, "Deny" NetworkPolicy refers to NetworkPolicy with
egressNetworkPolicyRuleAction or ingressNetworkPolicyRuleAction set to `Drop`
or `Reject`. Unprotected traffic without NetworkPolicy enforced has the same
color with its source Pod. Every link is in the shape of an arrow, pointing
from source to destination. Line graphs show the evolution of traffic throughput
with NetworkPolicy enforced.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-np-0.png" width="900" alt="Network-Policy Flows Dashboard">

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-np-1.png" width="900" alt="Network-Policy Flows Dashboard">

We can filter on desired fields, e.g. for egress/ingressNetworkPolicyRuleAction
value:

- 0: No action
- 1: Allow
- 2: Drop
- 3: Reject

Here is an example if we filter on traffic with only ingressNetworkPolicyRuleAction
equals to "Allow".

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-np-2.png" width="900" alt="Network-Policy Flows Dashboard">

Mouse over or click on source/destination highlights only the related traffic.
Mouse out or click on the background will bring all the traffic back.

<img src="https://downloads.antrea.io/static/08032022/flow-visibility-np-3.png" width="900" alt="Network-Policy Flows Dashboard">

#### Network Topology Dashboard

Network Topology Dashboard visualizes Pod-to-Pod and Pod-to-Service traffic.
Pods are visually grouped by Node. Each arrow points from the source to the
destination, and is labelled by the cumulative amount of data transmitted in
the selected time range.

<img src="https://downloads.antrea.io/static/03012023/flow-visibility-network-topology-0.png" width="400" alt="Network Topology Dashboard service dependency graph">

### Dashboard Customization

If you would like to make any change to any of the pre-built dashboards, or build
a new dashboard, please follow this [doc](https://grafana.com/docs/grafana/latest/dashboards/)
on how to build a dashboard.

From Theia 0.2, we use PersistentVolume for the Grafana configuration database.
If you create a new dashboard or modify the pre-built dashboards, once you click
on the "Save dashboard" button, the changes will be kept after a Grafana
Deployment restart. Other changes in settings like passwords and preferences will
also be kept. By default, we use HostPath PersistentVolume, which only works when
the Grafana Pod is deployed on the same host. In order to make sure settings are
preserved regardless of where the Grafana Pod is deployed, please choose to use
Local PV, NFS PV or other dynamic provisioning by defining your own StorageClasses.

In Theia 0.1, the changes to dashboards and settings will be lost after restarting
the Grafana Deployment. To restore those changes to dashboards after a restart,
as the first step, you will need to export the dashboard JSON file following the
[doc](https://grafana.com/docs/grafana/latest/dashboards/export-import/),
then there are two ways to import the dashboard depending on your needs:

- In the running Grafana UI, manually import the dashboard JSON files.
- If you want the changed dashboards to be automatically provisioned in Grafana
like our pre-built dashboards, generate a deployment manifest with the changes by
following the steps below:

1. Clone the repository. Exported dashboard JSON files should be placed under
`theia/build/charts/theia/provisioning/dashboards`.

1. If a new dashboard is added, add the file to `grafana.dashboard` in `values.yaml`.
You can also remove a dashboard by deleting the file in this section.

    ```yaml
    dashboards:
      - flow_records_dashboard.json
      - pod_to_pod_dashboard.json
      - pod_to_service_dashboard.json
      - pod_to_external_dashboard.json
      - node_to_node_dashboard.json
      - networkpolicy_dashboard.json
      - network_topology_dashboard.json
      - [new_dashboard_name].json
    ```

1. Deploy the Grafana Flow Collector with Helm by running

    ```bash
    helm install -f <path-to-your-values-file> theia build/charts/theia -n flow-visibility --create-namespace
    ```

    Or generate the new YAML manifest by running:

    ```bash
    ./hack/generate-manifest.sh > build/yamls/flow-visibility.yml
    ```
