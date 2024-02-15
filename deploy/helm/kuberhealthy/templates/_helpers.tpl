{{/* vim: set filetype=mustache: */}}
{{/*
Setup a chart name
*/}}
{{- define "kuberhealthy.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for RBAC APIs.
*/}}
{{- define "rbac.apiVersion" -}}
{{- if semverCompare "^1.8-0" .Capabilities.KubeVersion.GitVersion -}}
"rbac.authorization.k8s.io/v1"
{{- else -}}
"rbac.authorization.k8s.io/v1beta1"
{{- end -}}
{{- end -}}

{{/*
Labels that should be added on each resource
*/}}
{{- define "labels" -}}
{{- if .Values.global.commonLabels }}
{{- toYaml .Values.global.commonLabels }}
{{- end }}
{{- end -}}

