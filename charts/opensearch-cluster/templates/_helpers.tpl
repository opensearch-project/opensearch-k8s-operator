{{/*
Expand the name of the chart.
*/}}
{{- define "opensearch-cluster.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "opensearch-cluster.cluster-name" -}}
{{- default .Release.Name .Values.cluster.name | trunc 63 | trimSuffix "-" }}
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
HTTPRoute plus optional BackendTLSPolicy for one service. Takes a dict of root, path (values path for errors),
name, route, service, port and tlsHostname (default validation.hostname, a SAN of the service certificate).
*/}}
{{- define "opensearch-cluster.httpRoute" -}}
{{- $route := .route -}}
{{- if not (.root.Capabilities.APIVersions.Has "gateway.networking.k8s.io/v1/HTTPRoute") }}
{{- fail (printf "%s.enabled is true but the cluster does not serve gateway.networking.k8s.io/v1 HTTPRoute; install the Gateway API CRDs (with helm template, pass --api-versions gateway.networking.k8s.io/v1/HTTPRoute)" .path) }}
{{- end }}
{{- if not $route.parentRefs }}
{{- fail (printf "%s.parentRefs must name at least one Gateway when %s.enabled is true" .path .path) }}
{{- end }}
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ .name }}
  namespace: {{ .root.Release.Namespace }}
  labels:
    {{- include "opensearch-cluster.labels" .root | nindent 4 }}
    {{- with $route.labels }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
  {{- with $route.annotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  parentRefs:
    {{- toYaml $route.parentRefs | nindent 4 }}
  {{- with $route.hostnames }}
  hostnames:
    {{- range . }}
    - {{ . | quote }}
    {{- end }}
  {{- end }}
  rules:
    - matches:
        {{- if $route.matches }}
        {{- toYaml $route.matches | nindent 8 }}
        {{- else }}
        - path:
            type: PathPrefix
            value: /
        {{- end }}
      {{- with $route.filters }}
      filters:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with $route.timeouts }}
      timeouts:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      backendRefs:
        - name: {{ .service }}
          port: {{ .port }}
{{- $tls := $route.backendTLSPolicy | default dict }}
{{- if $tls.enabled }}
{{- $apiVersion := $tls.apiVersion | default "gateway.networking.k8s.io/v1" }}
{{- if not (.root.Capabilities.APIVersions.Has (printf "%s/BackendTLSPolicy" $apiVersion)) }}
{{- fail (printf "%s.backendTLSPolicy.enabled is true but the cluster does not serve %s BackendTLSPolicy; install the Gateway API CRDs or set %s.backendTLSPolicy.apiVersion" .path $apiVersion .path) }}
{{- end }}
{{- $validation := $tls.validation | default dict }}
{{- if not (or $validation.caCertificateRefs $validation.wellKnownCACertificates) }}
{{- fail (printf "%s.backendTLSPolicy.validation must set caCertificateRefs or wellKnownCACertificates" .path) }}
{{- end }}
---
apiVersion: {{ $apiVersion }}
kind: BackendTLSPolicy
metadata:
  name: {{ .name }}
  namespace: {{ .root.Release.Namespace }}
  labels:
    {{- include "opensearch-cluster.labels" .root | nindent 4 }}
spec:
  targetRefs:
    - group: ""
      kind: Service
      name: {{ .service }}
  validation:
    {{- merge (dict) $validation (dict "hostname" .tlsHostname) | toYaml | nindent 4 }}
{{- end }}
{{- end }}
