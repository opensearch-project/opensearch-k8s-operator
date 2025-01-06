{{/*
Expand the name of the chart.
*/}}
{{- define "opensearch-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "opensearch-cluster.cluster-name" -}}
{{- default .Values.cluster.name .Release.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "opensearch-cluster.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "opensearch-cluster.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "opensearch-cluster.labels" -}}
helm.sh/chart: {{ include "opensearch-cluster.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.cluster.labels }}
{{ . | toYaml }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "opensearch-cluster.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "opensearch-cluster.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Default pod antiAffinity to nodePool component if no affinity rules defined
This rule helps the nodePool replicas to schedule on different nodes
*/}}
{{- define "nodePools.defaultAffinity" -}}
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            opster.io/opensearch-cluster: {{ $.clusterName }}
        topologyKey: kubernetes.io/hostname
{{- end }}

{{/*
Takes the pod affinity rules from each nodePool and appends the default podAntiAffinity
*/}}
{{- define "nodePools.affinity" -}}
{{- $nodePool := .nodePool -}}
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            opster.io/opensearch-cluster: {{ $.clusterName }}
        topologyKey: kubernetes.io/hostname

    {{- /* checks if preferredDuringSchedulingIgnoredDuringExecution exists under podAntiAffinity and appending */ -}}
    {{- if not (empty $nodePool.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution) }}
    {{ $nodePool.podAntiAffinity.preferredDuringSchedulingIgnoredDuringExecution | toYaml | nindent 4 | trim }}
    {{- end }}

    {{- /* checks if requiredDuringSchedulingIgnoredDuringExecution exists under podAntiAffinity and appending */ -}}
    {{- if $nodePool.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution }}
    requiredDuringSchedulingIgnoredDuringExecution: {{ $nodePool.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution | toYaml | nindent 4 }}
    {{- end }}

  {{- /* checks if podAffinity exists in affinity and appending */ -}}
  {{- if $nodePool.podAffinity }}
  podAffinity: {{ $nodePool.podAffinity | toYaml | nindent 4 }}
  {{- end }}

  {{- /* checks if nodeAffinity exists in affinity and appending */ -}}
  {{- if $nodePool.nodeAffinity }}
  nodeAffinity: {{ $nodePool.nodeAffinity | toYaml | nindent 4 }}
  {{- end }}
{{- end }}
