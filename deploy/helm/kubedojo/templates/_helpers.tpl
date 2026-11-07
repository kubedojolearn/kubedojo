{{/* Release-scoped name prefix, at most 63 characters. */}}
{{- define "kubedojo.fullname" -}}
{{- if contains .Chart.Name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/* Labels on every object. Call with (dict "ctx" $ "component" "api"). */}}
{{- define "kubedojo.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .ctx.Chart.Name .ctx.Chart.Version | replace "+" "_" | trunc 63 }}
{{ include "kubedojo.selectorLabels" . }}
app.kubernetes.io/version: {{ .ctx.Values.image.tag | default .ctx.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .ctx.Release.Service }}
app.kubernetes.io/part-of: kubedojo
{{- end -}}

{{- define "kubedojo.selectorLabels" -}}
app.kubernetes.io/name: {{ .ctx.Chart.Name }}
app.kubernetes.io/instance: {{ .ctx.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{/* Image reference. Call with (dict "ctx" $ "name" "api"). */}}
{{- define "kubedojo.image" -}}
{{- printf "%s/kubedojo-%s:%s" .ctx.Values.image.registry .name (.ctx.Values.image.tag | default .ctx.Chart.AppVersion) -}}
{{- end -}}

{{- define "kubedojo.secret" -}}
{{- required "existingSecret is required: the Secret holding KubeDojo's keys (see README.md)" .Values.existingSecret -}}
{{- end -}}

{{- define "kubedojo.secretEnv" -}}
- name: {{ .key }}
  valueFrom:
    secretKeyRef:
      name: {{ include "kubedojo.secret" .ctx }}
      key: {{ .key }}
      {{- if .optional }}
      optional: true
      {{- end }}
{{- end -}}

{{/* Environment shared by the API, its bootstrap init container and the worker. */}}
{{- define "kubedojo.backendEnv" -}}
- name: APP_ENV
  value: production
- name: LOG_LEVEL
  value: {{ .Values.logLevel | quote }}
{{- if .Values.postgresql.enabled }}
{{ include "kubedojo.secretEnv" (dict "ctx" . "key" "POSTGRES_PASSWORD") }}
- name: DATABASE_URL
  value: "postgres://{{ .Values.postgresql.user }}:$(POSTGRES_PASSWORD)@{{ include "kubedojo.fullname" . }}-postgresql:5432/{{ .Values.postgresql.database }}?sslmode=disable"
{{- else }}
{{ include "kubedojo.secretEnv" (dict "ctx" . "key" "DATABASE_URL") }}
{{- end }}
{{- if .Values.redis.enabled }}
- name: REDIS_URL
  value: "redis://{{ include "kubedojo.fullname" . }}-redis:6379/0"
# In-cluster traffic to the bundled Redis, which only KubeDojo's pods may reach (networkPolicy).
- name: REDIS_ALLOW_PLAINTEXT
  value: "1"
{{- else }}
{{ include "kubedojo.secretEnv" (dict "ctx" . "key" "REDIS_URL") }}
{{- end }}
{{- end -}}

{{- define "kubedojo.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: {{ .uid }}
runAsGroup: {{ .uid }}
seccompProfile:
  type: RuntimeDefault
{{- end -}}

{{- define "kubedojo.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop: [ALL]
{{- end -}}

{{- define "kubedojo.scheduling" -}}
{{- with .Values.nodeSelector }}
nodeSelector:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.tolerations }}
tolerations:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.affinity }}
affinity:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end -}}
