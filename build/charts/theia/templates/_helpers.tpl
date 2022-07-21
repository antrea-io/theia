{{- define "clickhouse.monitor.container" }}
{{- $clickhouse := .clickhouse }}
{{- $version := .version }}
- name: clickhouse-monitor
  image: {{ $clickhouse.monitor.image.repository }}:{{ default $version $clickhouse.monitor.image.tag }}
  imagePullPolicy: {{ $clickhouse.monitor.image.pullPolicy }}
  env:
    - name: CLICKHOUSE_USERNAME
      valueFrom:
        secretKeyRef: 
          name: clickhouse-secret
          key: username
    - name: CLICKHOUSE_PASSWORD
      valueFrom:
        secretKeyRef:
          name: clickhouse-secret
          key: password
    - name: DB_URL
      value: "tcp://localhost:9000"
    - name: TABLE_NAME
      {{- if $clickhouse.cluster.enable }}
      value: "default.flows_local"
      {{- else }}
      value: "default.flows"
      {{- end }}
    - name: MV_NAMES
      {{- if $clickhouse.cluster.enable }}
      value: "default.flows_pod_view_local default.flows_node_view_local default.flows_policy_view_local"
      {{- else }}
      value: "default.flows_pod_view default.flows_node_view default.flows_policy_view"
      {{- end }}
    - name: STORAGE_SIZE
      value: {{ $clickhouse.storage.size | quote }}
    - name: THRESHOLD
      value: {{ $clickhouse.monitor.threshold | quote }}
    - name: DELETE_PERCENTAGE
      value: {{ $clickhouse.monitor.deletePercentage | quote }}
    - name: EXEC_INTERVAL
      value: {{ $clickhouse.monitor.execInterval }}
    - name: SKIP_ROUNDS_NUM
      value: {{ $clickhouse.monitor.skipRoundsNum | quote }}
{{- end }}

{{- define "clickhouse.server.container" }}
{{- $clickhouse := .clickhouse }}
{{- $enablePV := .enablePV }}
- name: clickhouse
  image: {{ $clickhouse.image.repository }}:{{ $clickhouse.image.tag }}
  imagePullPolicy: {{ $clickhouse.image.pullPolicy }}
  volumeMounts:
    - name: clickhouse-configmap-volume
      mountPath: /docker-entrypoint-initdb.d
    {{- if not $enablePV }}
    - name: clickhouse-storage-volume
      mountPath: /var/lib/clickhouse
    {{- end }}
{{- end }}

{{- define "clickhouse.volume" }}
{{- $clickhouse := .clickhouse }}
{{- $enablePV := .enablePV }}
- name: clickhouse-configmap-volume
  configMap:
    name: clickhouse-mounted-configmap
{{- if not $enablePV }}
- name: clickhouse-storage-volume
  emptyDir:
    medium: Memory
    sizeLimit: {{ $clickhouse.storage.size }}
{{- end }}
{{- end }}
