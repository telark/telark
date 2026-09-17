
{{- define "serviceDeployment" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $root := .root -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $includeResources := dig "includeResources" true $serviceConfig -}}
{{- $includeHealthCheck := dig "includeHealthCheck" true $serviceConfig -}}
{{- $includeSecurity := dig "includeSecurity" true $serviceConfig -}}
{{- $useRedis := dig "useRedis" true $serviceConfig -}}
{{- $useNatsCreds := dig "useNatsCreds" false $serviceConfig -}}
{{- $replicas := default $serviceDefaults.replicas $serviceConfig.replicas | default 1 -}}
{{- $autoscaling := mergeOverwrite (deepCopy ($serviceDefaults.autoscaling | default dict)) ($serviceConfig.autoscaling | default dict) -}}
{{- $port := default $serviceDefaults.port $serviceConfig.port | default 8080 -}}
{{- $tgps := default $serviceDefaults.terminationGracePeriodSec $serviceConfig.terminationGracePeriodSec | default 30 -}}
{{- $strategy := default $serviceDefaults.strategy $serviceConfig.strategy -}}
{{- $priorityClass := default $serviceDefaults.priorityClassName $serviceConfig.priorityClassName -}}
{{- $nodeSelector := $serviceConfig.nodeSelector | default $serviceDefaults.nodeSelector -}}
{{- $tolerations := $serviceConfig.tolerations | default $serviceDefaults.tolerations -}}
{{- $affinity := $serviceConfig.affinity | default $serviceDefaults.affinity -}}
{{- $podSecurity := $values.app.shared.podSecurityContext | default dict -}}
{{- $containerSecurity := $values.app.shared.containerSecurityContext | default dict -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    {{- include "telark.labels" $root | nindent 4 }}
    app.kubernetes.io/component: {{ $serviceConfig.name }}
    {{ printf "%s.io/category" (include "telark.name" $root) }}: {{ $serviceConfig.category }}
  {{- with (include "telark.annotations" $root) }}
  annotations:
    {{- . | nindent 4 }}
  {{- end }}
  name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}
  namespace: {{ $values.app.namespace }}
spec:
{{- if not $autoscaling.enabled }}
  replicas: {{ $replicas }}
{{- end }}
{{- if $strategy }}
  # Recreate for anything holding a ReadWriteOnce volume: the default rolling
  # update starts the new pod first, and it cannot attach a volume the old pod
  # still holds on another node, so the rollout stalls.
  strategy:
    type: {{ $strategy }}
{{- end }}
  selector:
    matchLabels:
      {{- include "telark.selectorLabels" (dict "root" $root "component" $serviceConfig.name) | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "telark.labels" $root | nindent 8 }}
        app.kubernetes.io/component: {{ $serviceConfig.name }}
        {{ printf "%s.io/category" (include "telark.name" $root) }}: {{ $serviceConfig.category }}
{{- if $useRedis }}
        {{ printf "%s-redis-client" $root.Release.Name }}: "true"
{{- end }}
    spec:
      terminationGracePeriodSeconds: {{ $tgps }}
{{- if $priorityClass }}
      priorityClassName: {{ $priorityClass }}
{{- end }}
      serviceAccountName: {{ include "telark.serviceAccountName" (dict "root" $root "serviceConfig" $serviceConfig) }}
      {{- include "telark.imagePullSecrets" $root | nindent 6 }}
{{- if and $includeSecurity $podSecurity.enabled }}
      securityContext:
        runAsNonRoot: true
        runAsUser: {{ $podSecurity.runAsUser }}
        runAsGroup: {{ $podSecurity.runAsGroup }}
        fsGroup: {{ $podSecurity.fsGroup }}
{{- end }}
{{- if $nodeSelector }}
      nodeSelector:
        {{- toYaml $nodeSelector | nindent 8 }}
{{- end }}
{{- if $tolerations }}
      tolerations:
        {{- toYaml $tolerations | nindent 8 }}
{{- end }}
{{- if $affinity }}
      affinity:
        {{- toYaml $affinity | nindent 8 }}
{{- end }}
{{- if and $serviceConfig.topologySpread $serviceConfig.topologySpread.enabled }}
      topologySpreadConstraints:
        - maxSkew: {{ $serviceConfig.topologySpread.maxSkew | default 1 }}
          topologyKey: {{ $serviceConfig.topologySpread.topologyKey | default "kubernetes.io/hostname" }}
          whenUnsatisfiable: {{ $serviceConfig.topologySpread.whenUnsatisfiable | default "ScheduleAnyway" }}
          labelSelector:
            matchLabels:
              {{- include "telark.selectorLabels" (dict "root" $root "component" $serviceConfig.name) | nindent 14 }}
{{- end }}
      containers:
        - name: {{ include "telark.fullname" $root }}-{{ $serviceConfig.name }}-container
          image: {{ $values.app.image.registry }}/{{ $serviceConfig.repository }}:{{ $serviceConfig.version }}
          imagePullPolicy: {{ $values.app.image.pullPolicy | default "Always" }}
{{- if and $useRedis $values.app.shared.redis }}
          envFrom:
            - configMapRef:
                name: {{ include "telark.fullname" $root }}-redis-cm
{{- end }}
          ports:
            - containerPort: {{ $port }}
              name: http
          env:
            {{/* Every service authenticates its peers, so this is never optional. */}}
            - name: {{ $values.app.serviceToken.envVar }}
              valueFrom:
                secretKeyRef:
                  name: {{ include "telark.fullname" $root }}-service-token-secret
                  key: token
{{- range $key, $value := $serviceConfig.env }}
            - name: {{ $key }}
              value: {{ tpl (printf "%v" $value) $root | quote }}
{{- end }}
{{- range $key, $secret := $serviceConfig.envFromSecret }}
            - name: {{ $key }}
              valueFrom:
                secretKeyRef:
                  name: {{ include "telark.fullname" $root }}-{{ $secret.name }}-secret
                  key: {{ $secret.key }}
{{- end }}
{{- if $useNatsCreds }}
{{- range $key, $value := $values.app.shared.nats }}
            - name: {{ $key }}
              value: {{ tpl ($value | toString) $root | quote }}
{{- end }}
{{- range $key, $secret := $values.app.shared.natsEnvFromSecret }}
            - name: {{ $key }}
              valueFrom:
                secretKeyRef:
                  name: {{ include "telark.fullname" $root }}-{{ $secret.name }}-secret
                  key: {{ $secret.key }}
{{- end }}
{{- end }}
{{- range $key, $cm := $serviceConfig.envFromConfigMap }}
            - name: {{ $key }}
              valueFrom:
                configMapKeyRef:
                  name: {{ $cm.name }}
                  key: {{ $cm.key }}
{{- end }}
{{- if and $includeResources $values.app.shared.resources }}
          resources:
            requests:
              memory: {{ $values.app.shared.resources.requests.memory }}
              cpu: {{ $values.app.shared.resources.requests.cpu }}
            limits:
              memory: {{ $values.app.shared.resources.limits.memory }}
              cpu: {{ $values.app.shared.resources.limits.cpu }}
{{- end }}
{{- if and $includeHealthCheck $values.app.shared.healthCheck }}
          livenessProbe:
            httpGet:
              path: {{  $values.app.shared.healthCheck.livenessProbe.path }}
              port: {{ $values.app.shared.healthCheck.port }}
            initialDelaySeconds: {{ $values.app.shared.healthCheck.livenessProbe.initialDelaySeconds }}
            periodSeconds: {{ $values.app.shared.healthCheck.livenessProbe.periodSeconds }}
            timeoutSeconds: {{ $values.app.shared.healthCheck.livenessProbe.timeoutSeconds }}
            failureThreshold: {{ $values.app.shared.healthCheck.livenessProbe.failureThreshold }}
          readinessProbe:
            httpGet:
              path: {{ $values.app.shared.healthCheck.readinessProbe.path }}
              port: {{ $values.app.shared.healthCheck.port }}
            initialDelaySeconds: {{ $values.app.shared.healthCheck.readinessProbe.initialDelaySeconds }}
            periodSeconds: {{ $values.app.shared.healthCheck.readinessProbe.periodSeconds }}
            timeoutSeconds: {{ $values.app.shared.healthCheck.readinessProbe.timeoutSeconds }}
            failureThreshold: {{ $values.app.shared.healthCheck.readinessProbe.failureThreshold }}
{{- end }}
{{- if and $includeSecurity $containerSecurity.enabled }}
          securityContext:
            allowPrivilegeEscalation: {{ $containerSecurity.allowPrivilegeEscalation }}
            runAsNonRoot: true
            runAsUser: {{ $containerSecurity.runAsUser }}
            runAsGroup: {{ $containerSecurity.runAsGroup }}
            readOnlyRootFilesystem: {{ $containerSecurity.readOnlyRootFilesystem }}
            capabilities:
              drop: {{ $containerSecurity.capabilities.drop | toJson }}
{{- end }}
{{- if $serviceConfig.volumeMounts }}
          volumeMounts:
{{- range $mount := $serviceConfig.volumeMounts }}
            - name: {{ $mount.name }}
              mountPath: {{ $mount.path }}
              {{- if $mount.readOnly }}
              readOnly: {{ $mount.readOnly }}
              {{- end }}
{{- end }}
{{- end }}
{{- if $serviceConfig.volumes }}
      volumes:
{{- range $volume := $serviceConfig.volumes }}
        - name: {{ $volume.name }}
          {{- if $volume.configMap }}
          configMap:
            name: {{ $volume.configMap.name }}
          {{- end }}
          {{- if $volume.secret }}
          secret:
            secretName: {{ $volume.secret.name }}
          {{- end }}
          {{- if $volume.persistentVolumeClaim }}
          persistentVolumeClaim:
            claimName: {{ tpl ($volume.persistentVolumeClaim.claimName | toString) $root }}
          {{- end }}
{{- end }}
{{- end }}
{{- end -}}
