{{/*
Expand the name of the chart.
*/}}
{{- define "jira-sync-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "jira-sync-operator.fullname" -}}
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
{{- define "jira-sync-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "jira-sync-operator.labels" -}}
helm.sh/chart: {{ include "jira-sync-operator.chart" . }}
{{ include "jira-sync-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/component: operator
app.kubernetes.io/part-of: jira-sync
{{- end }}

{{/*
Selector labels
*/}}
{{- define "jira-sync-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "jira-sync-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "jira-sync-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "jira-sync-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the ClusterRole to use
*/}}
{{- define "jira-sync-operator.clusterRoleName" -}}
{{- printf "%s-manager" (include "jira-sync-operator.fullname" .) }}
{{- end }}

{{/*
Create the name of the ClusterRoleBinding to use
*/}}
{{- define "jira-sync-operator.clusterRoleBindingName" -}}
{{- printf "%s-manager" (include "jira-sync-operator.fullname" .) }}
{{- end }}

{{/*
Create the namespace name
*/}}
{{- define "jira-sync-operator.namespace" -}}
{{- if .Values.namespace.create }}
{{- .Values.namespace.name }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
Create security context for operator pod
*/}}
{{- define "jira-sync-operator.securityContext" -}}
{{- toYaml .Values.operator.securityContext }}
{{- end }}

{{/*
Create pod security context for operator
*/}}
{{- define "jira-sync-operator.podSecurityContext" -}}
{{- toYaml .Values.operator.podSecurityContext }}
{{- end }}

{{/*
Create resource requirements for operator
*/}}
{{- define "jira-sync-operator.resources" -}}
{{- toYaml .Values.operator.resources }}
{{- end }}

{{/*
Create image reference for operator
*/}}
{{- define "jira-sync-operator.image" -}}
{{- printf "%s:%s" .Values.operator.image.repository .Values.operator.image.tag }}
{{- end }}