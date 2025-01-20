{{/*
Checks if the self-signed issuer should be used.
Returns true if webhook.certManager.issuer.name is empty.
*/}}
{{- define "use_selfsigned_issuer" -}}
{{- eq .Values.webhook.certManager.issuer.name "" -}}
{{- end -}}

{{/*
Checks if an existing issuer should be used.
Returns true if webhook.certManager.issuer.name is not empty.
*/}}
{{- define "use_existing_issuer" -}}
{{- not (eq .Values.webhook.certManager.issuer.name "") -}}
{{- end -}}

{{/*
Checks if cert-manager is enabled.
Returns true if webhook.certManager.enabled is true.
*/}}
{{- define "cert_manager_enabled" -}}
{{- .Values.webhook.certManager.enabled | default false -}}
{{- end -}}

{{/*
Checks if cert-manager is disabled.
Returns true if webhook.certManager.enabled is false.
*/}}
{{- define "cert_manager_disabled" -}}
{{- not (.Values.webhook.certManager.enabled | default false) -}}
{{- end -}}
