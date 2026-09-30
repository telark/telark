{{/*
Settings that install cleanly but leave a release unusable or unsafe fail the
render instead, with the value to set.
*/}}
{{- define "validateSecuritySettings" -}}
{{- $auth := .Values.app.auth -}}
{{- if and .Values.services.auth.enabled (not $auth.bootstrap.admin) (eq (toString $auth.passkey.selfRegistration) "false") -}}
{{- fail "app.auth.bootstrap.admin is empty and app.auth.passkey.selfRegistration is \"false\", so nobody could ever log in (auth refuses to start). Set your admin email: --set app.auth.bootstrap.admin=test@example.com" -}}
{{- end -}}
{{- if and (or .Values.ingress.enabled .Values.gateway.enabled) (or (not $auth.passkey.id) (not $auth.passkey.origin)) -}}
{{- fail "The dashboard is exposed (ingress or gateway) but app.auth.passkey.id or app.auth.passkey.origin is empty, so passkey ceremonies would trust client-supplied Host and Origin headers. Pin both: --set app.auth.passkey.id=telark.example.com --set app.auth.passkey.origin=https://telark.example.com" -}}
{{- end -}}
{{- if and .Values.app.kyverno.enabled (ne (toString .Values.app.kyverno.failOpen) (toString .Values.kyverno.features.forceFailurePolicyIgnore.enabled)) -}}
{{- fail (printf "app.kyverno.failOpen=%v but kyverno.features.forceFailurePolicyIgnore.enabled=%v. Helm cannot pass one to the other, so set both to the same value." .Values.app.kyverno.failOpen .Values.kyverno.features.forceFailurePolicyIgnore.enabled) -}}
{{- end -}}
{{- end -}}
