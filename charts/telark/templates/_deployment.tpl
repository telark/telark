
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
{{- $serviceToken := dig "serviceToken" true $serviceConfig -}}
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
{{- $healthCheck := mergeOverwrite (deepCopy ($values.app.shared.healthCheck | default dict)) ($serviceConfig.healthCheck | default dict) -}}
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
  # Derived from the exporter replica count: RollingUpdate on a shared
  # ReadWriteMany volume, Recreate on ReadWriteOnce (a rolling update would
  # start the new pod before the old one released the volume).
  strategy:
    type: {{ $strategy }}
{{- if eq $strategy "RollingUpdate" }}
    # Rendered so a later switch to Recreate removes it: the API refuses both together.
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
{{- end }}
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
{{- if hasKey $serviceConfig "automountServiceAccountToken" }}
      automountServiceAccountToken: {{ $serviceConfig.automountServiceAccountToken }}
{{- end }}
      {{- include "telark.imagePullSecrets" $root | nindent 6 }}
{{- if $serviceConfig.podSecurityContext }}
      securityContext:
        {{- toYaml $serviceConfig.podSecurityContext | nindent 8 }}
{{- else if and $includeSecurity $podSecurity.enabled }}
      securityContext:
        runAsNonRoot: true
        runAsUser: {{ $podSecurity.runAsUser }}
        runAsGroup: {{ $podSecurity.runAsGroup }}
        fsGroup: {{ $podSecurity.fsGroup }}
        {{- with $podSecurity.seccompProfile }}
        seccompProfile:
          {{- toYaml . | nindent 10 }}
        {{- end }}
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
          # Counts only this ReplicaSet's pods, so a rollout spreads the new pods too.
          matchLabelKeys:
            - pod-template-hash
{{- end }}
{{- with $serviceConfig.initContainers }}
      initContainers:
        {{- toYaml . | nindent 8 }}
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
{{- if $serviceToken }}
            - name: {{ $values.app.serviceToken.envVar }}
              valueFrom:
                secretKeyRef:
                  name: {{ include "telark.serviceTokenSecretName" $root }}
                  key: token
{{- end }}
{{- if $useRedis }}
            - name: REDIS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ include "telark.redisSecretName" $root }}
                  key: {{ $root.Values.redis.auth.existingSecretPasswordKey | default "redis-password" }}
{{- end }}
            - name: POD_IP
              valueFrom:
                fieldRef:
                  fieldPath: status.podIP
            - name: POD_NAMESPACE
              valueFrom:
                fieldRef:
                  fieldPath: metadata.namespace
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
{{- $natsSecret := include "telark.natsSecretName" (dict "root" $root "user" $serviceConfig.natsUser) }}
            - name: NATS_USER
              valueFrom:
                secretKeyRef:
                  name: {{ $natsSecret }}
                  key: username
            - name: NATS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ $natsSecret }}
                  key: password
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
{{- if and $includeHealthCheck $healthCheck }}
{{- range $probe := list "startupProbe" "livenessProbe" "readinessProbe" }}
{{- with get $healthCheck $probe }}
          {{ $probe }}:
            httpGet:
              path: {{ .path }}
              port: {{ $healthCheck.port }}
            {{- with .initialDelaySeconds }}
            initialDelaySeconds: {{ . }}
            {{- end }}
            periodSeconds: {{ .periodSeconds }}
            timeoutSeconds: {{ .timeoutSeconds }}
            failureThreshold: {{ .failureThreshold }}
{{- end }}
{{- end }}
{{- end }}
{{- if $serviceConfig.containerSecurityContext }}
          securityContext:
            {{- toYaml $serviceConfig.containerSecurityContext | nindent 12 }}
{{- else if and $includeSecurity $containerSecurity.enabled }}
          securityContext:
            allowPrivilegeEscalation: {{ $containerSecurity.allowPrivilegeEscalation }}
            runAsNonRoot: true
            runAsUser: {{ $containerSecurity.runAsUser }}
            runAsGroup: {{ $containerSecurity.runAsGroup }}
            readOnlyRootFilesystem: {{ $containerSecurity.readOnlyRootFilesystem }}
            capabilities:
              drop: {{ $containerSecurity.capabilities.drop | toJson }}
            {{- with $containerSecurity.seccompProfile }}
            seccompProfile:
              {{- toYaml . | nindent 14 }}
            {{- end }}
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
            secretName: {{ tpl ($volume.secret.name | toString) $root }}
          {{- end }}
          {{- if hasKey $volume "emptyDir" }}
          emptyDir: {{ $volume.emptyDir | default dict | toJson }}
          {{- end }}
          {{- if $volume.persistentVolumeClaim }}
          persistentVolumeClaim:
            claimName: {{ tpl ($volume.persistentVolumeClaim.claimName | toString) $root }}
          {{- end }}
{{- end }}
{{- end }}
{{- end -}}
