{{- define "clickhouse.monitor.container" }}
{{- $clickhouse := .clickhouse }}
{{- $version := .version }}
- name: clickhouse-monitor
  image: {{ include "clickHouseMonitorImage" . | quote }}
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
      value: "default.flows_local"
    - name: MV_NAMES
      value: "default.flows_pod_view_local default.flows_node_view_local default.flows_policy_view_local"
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
{{- $version := .version }}
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
  env:
    - name: THEIA_VERSION
      value: {{ $version }}
{{- end }}

{{- define "clickhouse.volume" }}
{{- $clickhouse := .clickhouse }}
{{- $enablePV := .enablePV }}
{{- $Files := .Files }}
- name: clickhouse-configmap-volume
  configMap:
    name: clickhouse-mounted-configmap
    items:
      {{- range $path, $_ :=  $Files.Glob  "provisioning/datasources/*.sh" }}
      - key: {{ regexReplaceAll "(.*)/" $path "" }}
        path: {{ regexReplaceAll "(.*)/" $path "" }}
      {{- end }}
      {{- range $path, $_ :=  $Files.Glob  "provisioning/datasources/migrators/upgrade/*" }}
      - key: {{ regexReplaceAll "(.*)/" $path "" }}
        path: migrators/upgrade/{{ regexReplaceAll "(.*)/" $path "" }}
      {{- end }}
      {{- range $path, $_ :=  $Files.Glob  "provisioning/datasources/migrators/downgrade/*" }}
      - key: {{ regexReplaceAll "(.*)/" $path "" }}
        path: migrators/downgrade/{{ regexReplaceAll "(.*)/" $path "" }}
      {{- end }}
{{- if not $enablePV }}
- name: clickhouse-storage-volume
  emptyDir:
    medium: Memory
    sizeLimit: {{ $clickhouse.storage.size }}
{{- end }}
{{- end }}

{{- define "clickHouseMonitorImageTag" -}}
{{- if .clickhouse.monitor.image.tag }}
{{- .clickhouse.monitor.image.tag -}}
{{- else if eq .version "latest" }}
{{- print "latest" -}}
{{- else }}
{{- print "v" .version -}}
{{- end }}
{{- end -}}

{{- define "clickHouseMonitorImage" -}}
{{- print .clickhouse.monitor.image.repository ":" (include "clickHouseMonitorImageTag" .) -}}
{{- end -}}
