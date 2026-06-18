{{- define "w7panel-cloudconfig.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "w7panel-cloudconfig.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "w7panel-cloudconfig.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{- define "w7panel-cloudconfig.labels" -}}
app.kubernetes.io/name: {{ include "w7panel-cloudconfig.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "w7panel-cloudconfig.selectorLabels" -}}
app.kubernetes.io/name: {{ include "w7panel-cloudconfig.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "w7panel-cloudconfig.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "w7panel-cloudconfig.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}
