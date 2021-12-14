{{- define "gatekeeper.rootCACertificate" -}}
{{ printf "gatekeeper-ca" }}
{{- end -}}

{{- define "gatekeeper.name" -}}
{{ printf "gatekeeper" }}
{{- end -}}

{{- define "gatekeeper.podLabels" -}}
{{- if .Values.podLabels }}
{{- toYaml .Values.podLabels | nindent 8 }}
{{- end }}
{{- end -}}
