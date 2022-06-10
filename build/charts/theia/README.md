# theia

![Version: 0.1.0-dev](https://img.shields.io/badge/Version-0.1.0--dev-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0-dev](https://img.shields.io/badge/AppVersion-0.1.0--dev-informational?style=flat-square)

Antrea Network Flow Visibility

**Homepage:** <https://antrea.io/>

## Source Code

* <https://github.com/antrea-io/theia>

## Requirements

Kubernetes: `>= 1.16.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| clickhouse.connectionSecret | object | `{"password":"clickhouse_operator_password","username":"clickhouse_operator"}` | Credentials to connect to ClickHouse. They will be stored in a secret. |
| clickhouse.image | object | `{"pullPolicy":"IfNotPresent","repository":"projects.registry.vmware.com/antrea/theia-clickhouse-server","tag":"21.11"}` | Container image used by ClickHouse. |
| clickhouse.monitor.deletePercentage | float | `0.5` | The percentage of records in ClickHouse that will be deleted when the storage grows above threshold. Vary from 0 to 1. |
| clickhouse.monitor.enable | bool | `true` | Determine whether to run a monitor to periodically check the ClickHouse memory usage and clean data. |
| clickhouse.monitor.execInterval | string | `"1m"` | The time interval between two round of monitoring. Can be a plain integer using one of these unit suffixes ns, us (or µs), ms, s, m, h. |
| clickhouse.monitor.image | object | `{"pullPolicy":"IfNotPresent","repository":"projects.registry.vmware.com/antrea/theia-clickhouse-monitor","tag":"latest"}` | Container image used by the ClickHouse Monitor. |
| clickhouse.monitor.skipRoundsNum | int | `3` | The number of rounds for the monitor to stop after a deletion to wait for the ClickHouse MergeTree Engine to release memory. |
| clickhouse.monitor.threshold | float | `0.5` | The storage percentage at which the monitor starts to delete old records. Vary from 0 to 1. |
| clickhouse.service.httpPort | int | `8123` | TCP port number for the ClickHouse service. |
| clickhouse.service.tcpPort | int | `9000` | TCP port number for the ClickHouse service. |
| clickhouse.service.type | string | `"ClusterIP"` | The type of Service exposing ClickHouse. It can be one of ClusterIP, NodePort or LoadBalancer. |
| clickhouse.storage.createPersistentVolume.local.affinity | object | `{}` | Affinity for the Local Persistent Volume. By default it requires to label the Node used to store the ClickHouse data with "antrea.io/clickhouse-data-node=". |
| clickhouse.storage.createPersistentVolume.local.path | string | `"/data"` | The local path. Required when type is "Local". |
| clickhouse.storage.createPersistentVolume.nfs.host | string | `""` | The NFS server hostname or IP address. Required when type is "NFS". |
| clickhouse.storage.createPersistentVolume.nfs.path | string | `""` | The path exported on the NFS server. Required when type is "NFS". |
| clickhouse.storage.createPersistentVolume.type | string | `""` | Type of PersistentVolume. Can be set to "Local" or "NFS". Please set this value to use a PersistentVolume created by Theia. |
| clickhouse.storage.persistentVolumeClaimSpec | object | `{}` | Specification for PersistentVolumeClaim. This is ignored if createPersistentVolume.type is non-empty.  To use a custom PersistentVolume, please set  storageClassName: "" volumeName: "<my-pv>". To dynamically provision a PersistentVolume, please set storageClassName: "<my-storage-class>". Memory storage is used if both createPersistentVolume.type and persistentVolumeClaimSpec are empty. |
| clickhouse.storage.size | string | `"8Gi"` | ClickHouse storage size. Can be a plain integer or as a fixed-point number using one of these quantity suffixes: E, P, T, G, M, K. Or the power-of-two equivalents: Ei, Pi, Ti, Gi, Mi, Ki. |
| clickhouse.ttl | string | `"1 HOUR"` | Time to live for data in the ClickHouse. Can be a plain integer using one of these unit suffixes SECOND, MINUTE, HOUR, DAY, WEEK, MONTH, QUARTER, YEAR. |
| grafana.dashboards | list | `["flow_records_dashboard.json","pod_to_pod_dashboard.json","pod_to_service_dashboard.json","pod_to_external_dashboard.json","node_to_node_dashboard.json","networkpolicy_dashboard.json"]` | The dashboards to be displayed in Grafana UI. The files must be put under  provisioning/dashboards. |
| grafana.enable | bool | `true` | Determine whether to install Grafana. It is used as a data visualization and monitoring tool.   |
| grafana.image | object | `{"pullPolicy":"IfNotPresent","repository":"projects.registry.vmware.com/antrea/theia-grafana","tag":"8.3.3"}` | Container image used by Grafana. |
| grafana.installPlugins | list | `["https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-sankey-plugin-1.0.1.zip;theia-grafana-sankey-plugin","https://downloads.antrea.io/artifacts/grafana-custom-plugins/theia-grafana-chord-plugin-1.0.0.zip;theia-grafana-chord-plugin","grafana-clickhouse-datasource 1.0.1"]` | Grafana plugins to install. |
| grafana.loginSecret | object | `{"password":"admin","username":"admin"}` | Credentials to login to Grafana. They will be stored in a secret. |
| grafana.service.tcpPort | int | `3000` | TCP port number for the Grafana service. |
| grafana.service.type | string | `"NodePort"` | The type of Service exposing Grafana. It must be one of NodePort or LoadBalancer. |
| sparkOperator.enable | bool | `false` | Determine whether to install Spark Operator. It is required to run Network Policy Recommendation jobs. |
| sparkOperator.image | object | `{"pullPolicy":"IfNotPresent","repository":"projects.registry.vmware.com/antrea/theia-spark-operator","tag":"v1beta2-1.3.3-3.1.1"}` | Container image used by Spark Operator. |
| sparkOperator.name | string | `"policy-recommendation"` | Name of Spark Operator. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.7.0](https://github.com/norwoodj/helm-docs/releases/v1.7.0)
