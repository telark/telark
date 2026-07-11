
{{- define "serviceDeployment" -}}
{{- $service := .service -}}
{{- $values := .values -}}
{{- $serviceConfig := index $values.services $service -}}
{{- $serviceDefaults := $values.app.serviceDefaults | default dict -}}
{{- $includeResources := dig "includeResources" true $serviceConfig -}}
{{- $includeHealthCheck := dig "includeHealthCheck" true $serviceConfig -}}
{{- $includeSecurity := dig "includeSecurity" true $serviceConfig -}}
{{- $useRedis := dig "useRedis" true $serviceConfig -}}
{{- $useNatsCreds := dig "useNatsCreds" false $serviceConfig -}}
{{- $replicas := default $serviceDefaults.replicas $serviceConfig.replicas | default 1 -}}
{{- $port := default $serviceDefaults.port $serviceConfig.port | default 8080 -}}
{{- $tgps := default $serviceDefaults.terminationGracePeriodSec $serviceConfig.terminationGracePeriodSec | default 30 -}}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/name: {{ $values.app.name }}-{{ $serviceConfig.name }}
    app.kubernetes.io/part-of: {{ $values.app.name }}
  name: {{ $values.app.name }}-{{ $serviceConfig.name }}
  namespace: {{ $values.app.namespace }}
spec:
  replicas: {{ $replicas }}
  selector:
    matchLabels:
      app: {{ $values.app.name }}-{{ $serviceConfig.name }}
      type: {{ $serviceConfig.category }}
  template:
    metadata:
      labels:
        app: {{ $values.app.name }}-{{ $serviceConfig.name }}
        type: {{ $serviceConfig.category }}
    spec:
      terminationGracePeriodSeconds: {{ $tgps }}
      serviceAccountName: {{ $values.app.name }}-{{ $serviceConfig.name }}-sa
      imagePullSecrets:
        - name: {{ $values.app.name }}-reg-cred
{{- if and $includeSecurity $values.app.shared.security }}
      securityContext:
        runAsNonRoot: true
        runAsUser: {{ $values.app.shared.security.runAsUser }}
        runAsGroup: {{ $values.app.shared.security.runAsGroup }}
        fsGroup: {{ $values.app.shared.security.fsGroup }}
{{- end }}
{{- if and $serviceConfig.topologySpread $serviceConfig.topologySpread.enabled }}
      topologySpreadConstraints:
        - maxSkew: {{ $serviceConfig.topologySpread.maxSkew | default 1 }}
          topologyKey: {{ $serviceConfig.topologySpread.topologyKey | default "kubernetes.io/hostname" }}
          whenUnsatisfiable: {{ $serviceConfig.topologySpread.whenUnsatisfiable | default "ScheduleAnyway" }}
          labelSelector:
            matchLabels:
              app: {{ $values.app.name }}-{{ $serviceConfig.name }}
              type: {{ $serviceConfig.category }}
{{- end }}
      containers:
        - name: {{ $values.app.name }}-{{ $serviceConfig.name }}-container
          image: {{ $values.app.image.registry }}/{{ $values.app.image.repository }}:{{ $serviceConfig.imageTagPrefix }}{{ $serviceConfig.version }}
          imagePullPolicy: Always
{{- if and $useRedis $values.app.shared.redis }}
          envFrom:
            - configMapRef:
                name: {{ $values.app.name }}-redis-cm
{{- end }}
          ports:
            - containerPort: {{ $port }}
              name: http
{{- if or $serviceConfig.env (or $serviceConfig.envFromSecret (or $serviceConfig.envFromConfigMap $useNatsCreds)) }}
          env:
{{- range $key, $value := $serviceConfig.env }}
            - name: {{ $key }}
              value: {{ tpl (printf "%v" $value) (dict "Values" $values) | quote }}
{{- end }}
{{- range $key, $secret := $serviceConfig.envFromSecret }}
            - name: {{ $key }}
              valueFrom:
                secretKeyRef:
                  name: {{ $values.app.name }}-{{ $secret.name }}-secret
                  key: {{ $secret.key }}
{{- end }}
{{- if $useNatsCreds }}
{{- range $key, $secret := $values.app.shared.natsEnvFromSecret }}
            - name: {{ $key }}
              valueFrom:
                secretKeyRef:
                  name: {{ $values.app.name }}-{{ $secret.name }}-secret
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
{{- if and $includeSecurity $values.app.shared.security }}
          securityContext:
            allowPrivilegeEscalation: {{ $values.app.shared.security.allowPrivilegeEscalation }}
            runAsNonRoot: true
            runAsUser: {{ $values.app.shared.security.runAsUser }}
            runAsGroup: {{ $values.app.shared.security.runAsGroup }}
            readOnlyRootFilesystem: {{ $values.app.shared.security.readOnlyRootFilesystem }}
            capabilities:
              drop: {{ $values.app.shared.security.dropCapabilities | toJson }}
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
            claimName: {{ tpl ($volume.persistentVolumeClaim.claimName | toString) (dict "Values" $values) }}
          {{- end }}
{{- end }}
{{- end }}
{{- end -}} 
