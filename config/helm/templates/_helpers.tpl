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
{{- eq (.Values.webhook.certManager.enabled | default false) true -}}
{{- end -}}

{{/*
Checks if cert-manager is disabled.
Returns true if webhook.certManager.enabled is false.
*/}}
{{- define "cert_manager_disabled" -}}
{{- eq (.Values.webhook.certManager.enabled | default false) false -}}
{{- end -}}

{{/*
Checks if the CA bundle is specified.
Returns true if webhook.ca.caBundle is empty.
*/}}
{{- define "ca_bundle_unspecified" -}}
{{- eq .Values.webhook.ca.caBundle "" -}}
{{- end -}}

{{/*
Checks if the CA bundle is unspecified.
Returns true if webhook.ca.caBundle isn't empty.
*/}}
{{- define "ca_bundle_specified" -}}
{{- ne .Values.webhook.ca.caBundle "" -}}
{{- end -}}

{{/*
Checks if cert-manager is disabled and CA bundle is unspecified.
Returns true if both conditions are met.
*/}}
{{- define "cert_manager_disabled_and_ca_bundle_unspecified" -}}
{{- $certManagerDisabled := include "cert_manager_disabled" . }}
{{- $caBundleUnspecified := include "ca_bundle_unspecified" . }}
{{- if and (eq $certManagerDisabled "true") (eq $caBundleUnspecified "true") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}