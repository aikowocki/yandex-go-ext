{{/*
Базовое имя приложения (по умолчанию "pyroscope")
Можно переопределить через nameOverride в values.yaml
*/}}
{{- define "pyroscope.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Полное уникальное имя для развёртывания
Комбинирует название релиза Helm с именем приложения
Пример: my-release-pyroscope
*/}}
{{- define "pyroscope.fullname" -}}
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
Информация о чарте для отслеживания версий
Формат: имя-версия (например: pyroscope-0.1.0)
*/}}
{{- define "pyroscope.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.AppVersion | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Все стандартные метки для ресурсов Kubernetes
Используются Service, Prometheus, мониторингом и управлением
Добавляй этот хелпер ко всем ресурсам: labels: {{ include "pyroscope.labels" . | nindent 4 }}
*/}}
{{- define "pyroscope.labels" -}}
helm.sh/chart: {{ include "pyroscope.chart" . }}
{{ include "pyroscope.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Метки для поиска и выбора подов
Service использует эти метки для маршрутизации трафика
*/}}
{{- define "pyroscope.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pyroscope.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
