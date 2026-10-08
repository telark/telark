# telark

![Version: 0.1.1](https://img.shields.io/badge/Version-0.1.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.1](https://img.shields.io/badge/AppVersion-0.1.1-informational?style=flat-square)

A protection gate for your Kubernetes applications. Decide what can change an app, and when; see every change that got through and why an app broke.

**Homepage:** <https://telark.io>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| telark | <contact@telark.io> | <https://github.com/telark> |

## Source Code

* <https://github.com/telark/telark>

## Requirements

Kubernetes: `>=1.30.0-0`

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | nats | 9.0.28 |
| https://charts.bitnami.com/bitnami | redis | 23.0.10 |
| https://charts.fairwinds.com/stable | vpa | 5.1.0 |
| https://helm.otwld.com/ | ollama | 1.50.0 |
| https://kubernetes-sigs.github.io/metrics-server/ | metrics-server | 3.12.2 |
| https://kyverno.github.io/kyverno/ | kyverno | 3.9.1 |
| oci://ghcr.io/telark/charts | telark-crds | 0.0.3 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| app.auth.bootstrap.admin | string | `""` |  |
| app.auth.oidc.existingSecret | string | `""` |  |
| app.auth.passkey.id | string | `""` |  |
| app.auth.passkey.name | string | `"Dashboard App"` |  |
| app.auth.passkey.origin | string | `""` |  |
| app.crdGuard.enabled | bool | `true` |  |
| app.crdGuard.enforce | bool | `true` |  |
| app.crdGuard.extraAllowedUsers | list | `[]` |  |
| app.image.pullPolicy | string | `"Always"` |  |
| app.image.pullSecrets | list | `[]` |  |
| app.image.registry | string | `"ghcr.io/telark"` |  |
| app.kubectlImage | string | `"registry.k8s.io/kubectl:v1.37.1@sha256:b7cab618e281b1ee7484e7b706a96e2135fbb6e072c2a573a7dab4e87d7f2385"` |  |
| app.kyverno.enabled | bool | `true` |  |
| app.kyverno.failOpen | bool | `true` |  |
| app.mode | string | `"standard"` |  |
| app.name | string | `"telark"` |  |
| app.namespace | string | `"telark"` |  |
| app.networkPolicy.enabled | bool | `true` |  |
| app.ollama.autoPull | bool | `true` |  |
| app.ollama.enabled | bool | `true` |  |
| app.ollama.runtimeUrl | string | `""` |  |
| app.persistence.enabled | bool | `true` |  |
| app.persistence.reportsSize | string | `"512Mi"` |  |
| app.persistence.snapshotsSize | string | `"512Mi"` |  |
| app.persistence.storageClass | string | `""` |  |
| app.selfMonitoring.enabled | bool | `false` |  |
| app.serviceDefaults.affinity | object | `{}` |  |
| app.serviceDefaults.autoscaling.enabled | bool | `true` |  |
| app.serviceDefaults.autoscaling.maxReplicas | int | `3` |  |
| app.serviceDefaults.autoscaling.minReplicas | int | `1` |  |
| app.serviceDefaults.autoscaling.targetCPUUtilizationPercentage | int | `80` |  |
| app.serviceDefaults.nodeSelector | object | `{}` |  |
| app.serviceDefaults.port | int | `8080` |  |
| app.serviceDefaults.replicas | int | `1` |  |
| app.serviceDefaults.serviceType | string | `"ClusterIP"` |  |
| app.serviceDefaults.terminationGracePeriodSec | int | `60` |  |
| app.serviceDefaults.tolerations | list | `[]` |  |
| app.serviceDefaults.vpa.updateMode | string | `"Auto"` |  |
| app.serviceToken.envVar | string | `"TELARK_SERVICE_TOKEN"` |  |
| app.serviceToken.existingSecret | string | `""` |  |
| app.serviceToken.value | string | `""` |  |
| app.shared.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| app.shared.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| app.shared.containerSecurityContext.enabled | bool | `true` |  |
| app.shared.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| app.shared.containerSecurityContext.runAsGroup | int | `1001` |  |
| app.shared.containerSecurityContext.runAsUser | int | `1001` |  |
| app.shared.containerSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| app.shared.healthCheck.livenessProbe.failureThreshold | int | `3` |  |
| app.shared.healthCheck.livenessProbe.initialDelaySeconds | int | `15` |  |
| app.shared.healthCheck.livenessProbe.path | string | `"/api/v1/status/live"` |  |
| app.shared.healthCheck.livenessProbe.periodSeconds | int | `15` |  |
| app.shared.healthCheck.livenessProbe.timeoutSeconds | int | `15` |  |
| app.shared.healthCheck.port | string | `"http"` |  |
| app.shared.healthCheck.readinessProbe.failureThreshold | int | `3` |  |
| app.shared.healthCheck.readinessProbe.initialDelaySeconds | int | `5` |  |
| app.shared.healthCheck.readinessProbe.path | string | `"/api/v1/status/ready"` |  |
| app.shared.healthCheck.readinessProbe.periodSeconds | int | `5` |  |
| app.shared.healthCheck.readinessProbe.timeoutSeconds | int | `15` |  |
| app.shared.healthCheck.startupProbe.failureThreshold | int | `60` |  |
| app.shared.healthCheck.startupProbe.path | string | `"/api/v1/status/live"` |  |
| app.shared.healthCheck.startupProbe.periodSeconds | int | `5` |  |
| app.shared.healthCheck.startupProbe.timeoutSeconds | int | `5` |  |
| app.shared.nats.NATS_HOST | string | `"{{ .Release.Name }}-nats"` |  |
| app.shared.podSecurityContext.enabled | bool | `true` |  |
| app.shared.podSecurityContext.fsGroup | int | `1001` |  |
| app.shared.podSecurityContext.runAsGroup | int | `1001` |  |
| app.shared.podSecurityContext.runAsUser | int | `1001` |  |
| app.shared.podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| app.shared.redis.REDIS_HOST | string | `"{{ .Release.Name }}-redis-master"` |  |
| app.shared.redis.REDIS_PORT | string | `"6379"` |  |
| app.shared.resources.limits.cpu | string | `"1000m"` |  |
| app.shared.resources.limits.memory | string | `"512Mi"` |  |
| app.shared.resources.requests.cpu | string | `"100m"` |  |
| app.shared.resources.requests.memory | string | `"128Mi"` |  |
| commonAnnotations | object | `{}` |  |
| commonLabels | object | `{}` |  |
| crds.enabled | bool | `true` |  |
| fullnameOverride | string | `""` |  |
| gateway.annotations | object | `{}` |  |
| gateway.enabled | bool | `false` |  |
| gateway.hostnames | list | `[]` |  |
| gateway.parentRefs | list | `[]` |  |
| gateway.service | string | `"ui"` |  |
| global.imagePullSecrets | list | `[]` |  |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `""` |  |
| ingress.enabled | bool | `false` |  |
| ingress.host | string | `""` |  |
| ingress.path | string | `"/"` |  |
| ingress.pathType | string | `"Prefix"` |  |
| ingress.service | string | `"ui"` |  |
| ingress.tls | list | `[]` |  |
| kyverno.admissionController.container.extraArgs.clientRateLimitBurst | int | `100` |  |
| kyverno.admissionController.container.extraArgs.clientRateLimitQPS | int | `50` |  |
| kyverno.admissionController.container.resources.limits.memory | string | `"512Mi"` |  |
| kyverno.admissionController.container.resources.requests.cpu | string | `"100m"` |  |
| kyverno.admissionController.container.resources.requests.memory | string | `"128Mi"` |  |
| kyverno.admissionController.initContainer.resources.limits.memory | string | `"128Mi"` |  |
| kyverno.admissionController.initContainer.resources.requests.cpu | string | `"50m"` |  |
| kyverno.admissionController.initContainer.resources.requests.memory | string | `"64Mi"` |  |
| kyverno.admissionController.podDisruptionBudget.enabled | bool | `true` |  |
| kyverno.admissionController.podDisruptionBudget.minAvailable | int | `1` |  |
| kyverno.admissionController.replicas | int | `2` |  |
| kyverno.backgroundController.extraArgs.clientRateLimitBurst | int | `100` |  |
| kyverno.backgroundController.extraArgs.clientRateLimitQPS | int | `50` |  |
| kyverno.backgroundController.replicas | int | `1` |  |
| kyverno.backgroundController.resources.limits.memory | string | `"256Mi"` |  |
| kyverno.backgroundController.resources.requests.cpu | string | `"100m"` |  |
| kyverno.backgroundController.resources.requests.memory | string | `"128Mi"` |  |
| kyverno.cleanupController.extraArgs.clientRateLimitBurst | int | `100` |  |
| kyverno.cleanupController.extraArgs.clientRateLimitQPS | int | `50` |  |
| kyverno.cleanupController.replicas | int | `1` |  |
| kyverno.cleanupController.resources.limits.memory | string | `"256Mi"` |  |
| kyverno.cleanupController.resources.requests.cpu | string | `"100m"` |  |
| kyverno.cleanupController.resources.requests.memory | string | `"128Mi"` |  |
| kyverno.features.forceFailurePolicyIgnore.enabled | bool | `true` |  |
| kyverno.namespaceOverride | string | `"telark"` |  |
| kyverno.reportsController.extraArgs.clientRateLimitBurst | int | `100` |  |
| kyverno.reportsController.extraArgs.clientRateLimitQPS | int | `50` |  |
| kyverno.reportsController.replicas | int | `1` |  |
| kyverno.reportsController.resources.limits.memory | string | `"256Mi"` |  |
| kyverno.reportsController.resources.requests.cpu | string | `"100m"` |  |
| kyverno.reportsController.resources.requests.memory | string | `"128Mi"` |  |
| kyverno.webhooksCleanup.enabled | bool | `false` |  |
| metrics-server.args | list | `[]` |  |
| metrics-server.enabled | bool | `true` |  |
| metrics-server.resources.limits.cpu | string | `"200m"` |  |
| metrics-server.resources.limits.memory | string | `"400Mi"` |  |
| metrics-server.resources.requests.cpu | string | `"100m"` |  |
| metrics-server.resources.requests.memory | string | `"200Mi"` |  |
| monitoring.serviceMonitor.enabled | bool | `false` |  |
| monitoring.serviceMonitor.interval | string | `"30s"` |  |
| monitoring.serviceMonitor.labels | object | `{}` |  |
| monitoring.serviceMonitor.path | string | `"/metrics"` |  |
| nameOverride | string | `""` |  |
| nats.configuration | string | `"server_name: nats-server\nport: 4222\njetstream {\n  store_dir: \"/data\"\n  max_mem: 1G\n  # Below the 1Gi volume, so JetStream refuses new messages before the disk fills.\n  max_file: 700M\n}\nauthorization {\n  users = [\n    {\n      user: $NATS_PUBLISHER_USER,\n      password: $NATS_PUBLISHER_PASSWORD,\n      permissions: {\n        publish   = [\"telark.applications.*\"]\n        subscribe = [\"_INBOX.>\"]\n      }\n    },\n    {\n      user: $NATS_CONSUMER_USER,\n      password: $NATS_CONSUMER_PASSWORD,\n      permissions: {\n        publish   = [\"$JS.API.>\", \"$JS.ACK.>\"]\n        subscribe = [\"telark.applications.*\", \"_INBOX.>\"]\n      }\n    }\n  ]\n}\n"` |  |
| nats.customLivenessProbe.failureThreshold | int | `6` |  |
| nats.customLivenessProbe.initialDelaySeconds | int | `30` |  |
| nats.customLivenessProbe.periodSeconds | int | `10` |  |
| nats.customLivenessProbe.tcpSocket.port | string | `"client"` |  |
| nats.customLivenessProbe.timeoutSeconds | int | `5` |  |
| nats.customReadinessProbe.failureThreshold | int | `6` |  |
| nats.customReadinessProbe.initialDelaySeconds | int | `5` |  |
| nats.customReadinessProbe.periodSeconds | int | `10` |  |
| nats.customReadinessProbe.tcpSocket.port | string | `"client"` |  |
| nats.customReadinessProbe.timeoutSeconds | int | `5` |  |
| nats.enabled | bool | `true` |  |
| nats.existingSecrets.consumer | string | `""` |  |
| nats.existingSecrets.publisher | string | `""` |  |
| nats.extraEnvVars[0].name | string | `"NATS_PUBLISHER_USER"` |  |
| nats.extraEnvVars[0].valueFrom.secretKeyRef.key | string | `"username"` |  |
| nats.extraEnvVars[0].valueFrom.secretKeyRef.name | string | `"{{ .Values.existingSecrets.publisher | default \"telark-nats-publisher-secret\" }}"` |  |
| nats.extraEnvVars[1].name | string | `"NATS_PUBLISHER_PASSWORD"` |  |
| nats.extraEnvVars[1].valueFrom.secretKeyRef.key | string | `"password"` |  |
| nats.extraEnvVars[1].valueFrom.secretKeyRef.name | string | `"{{ .Values.existingSecrets.publisher | default \"telark-nats-publisher-secret\" }}"` |  |
| nats.extraEnvVars[2].name | string | `"NATS_CONSUMER_USER"` |  |
| nats.extraEnvVars[2].valueFrom.secretKeyRef.key | string | `"username"` |  |
| nats.extraEnvVars[2].valueFrom.secretKeyRef.name | string | `"{{ .Values.existingSecrets.consumer | default \"telark-nats-consumer-secret\" }}"` |  |
| nats.extraEnvVars[3].name | string | `"NATS_CONSUMER_PASSWORD"` |  |
| nats.extraEnvVars[3].valueFrom.secretKeyRef.key | string | `"password"` |  |
| nats.extraEnvVars[3].valueFrom.secretKeyRef.name | string | `"{{ .Values.existingSecrets.consumer | default \"telark-nats-consumer-secret\" }}"` |  |
| nats.image.pullPolicy | string | `"IfNotPresent"` |  |
| nats.image.registry | string | `"docker.io"` |  |
| nats.image.repository | string | `"nats"` |  |
| nats.image.tag | string | `"2.12.15-scratch"` |  |
| nats.networkPolicy.enabled | bool | `false` |  |
| nats.persistence.enabled | bool | `true` |  |
| nats.persistence.path | string | `"/data"` |  |
| nats.persistence.size | string | `"1Gi"` |  |
| nats.service.type | string | `"ClusterIP"` |  |
| ollama.extraEnv[0].name | string | `"OLLAMA_NO_CLOUD"` |  |
| ollama.extraEnv[0].value | string | `"true"` |  |
| ollama.extraEnv[1].name | string | `"OLLAMA_CONTEXT_LENGTH"` |  |
| ollama.extraEnv[1].value | string | `"4096"` |  |
| ollama.extraEnv[2].name | string | `"OLLAMA_KEEP_ALIVE"` |  |
| ollama.extraEnv[2].value | string | `"-1"` |  |
| ollama.extraEnv[3].name | string | `"OLLAMA_NUM_PARALLEL"` |  |
| ollama.extraEnv[3].value | string | `"1"` |  |
| ollama.extraEnv[4].name | string | `"OLLAMA_MAX_LOADED_MODELS"` |  |
| ollama.extraEnv[4].value | string | `"1"` |  |
| ollama.extraEnv[5].name | string | `"OLLAMA_MAX_QUEUE"` |  |
| ollama.extraEnv[5].value | string | `"8"` |  |
| ollama.extraEnv[6].name | string | `"OLLAMA_FLASH_ATTENTION"` |  |
| ollama.extraEnv[6].value | string | `"1"` |  |
| ollama.extraEnv[7].name | string | `"OLLAMA_KV_CACHE_TYPE"` |  |
| ollama.extraEnv[7].value | string | `"q8_0"` |  |
| ollama.ollama.models.pull | list | `[]` |  |
| ollama.persistentVolume.annotations."helm.sh/resource-policy" | string | `"keep"` |  |
| ollama.persistentVolume.enabled | bool | `true` |  |
| ollama.persistentVolume.size | string | `"6Gi"` |  |
| ollama.persistentVolume.storageClass | string | `""` |  |
| ollama.resources.limits.cpu | string | `"2"` |  |
| ollama.resources.requests.cpu | string | `"250m"` |  |
| ollama.resources.requests.memory | string | `"1536Mi"` |  |
| ollama.securityContext.allowPrivilegeEscalation | bool | `false` |  |
| ollama.securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| ollama.securityContext.readOnlyRootFilesystem | bool | `true` |  |
| ollama.securityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| redis.architecture | string | `"standalone"` |  |
| redis.auth.enabled | bool | `true` |  |
| redis.auth.existingSecret | string | `"{{ .Release.Name }}-redis-secret"` |  |
| redis.image.digest | string | `"sha256:33a5a129cadcc5dfa294e5a1fe622efcbe3774a8b85c731ac14d0532877b7d16"` |  |
| redis.master.disableCommands[0] | string | `"FLUSHDB"` |  |
| redis.master.disableCommands[1] | string | `"FLUSHALL"` |  |
| redis.master.disableCommands[2] | string | `"CONFIG"` |  |
| redis.master.disableCommands[3] | string | `"ACL"` |  |
| redis.master.persistence.enabled | bool | `true` |  |
| redis.master.persistence.size | string | `"2Gi"` |  |
| redis.master.resources.limits.cpu | string | `"500m"` |  |
| redis.master.resources.limits.ephemeral-storage | string | `"2Gi"` |  |
| redis.master.resources.limits.memory | string | `"512Mi"` |  |
| redis.master.resources.requests.cpu | string | `"100m"` |  |
| redis.master.resources.requests.ephemeral-storage | string | `"50Mi"` |  |
| redis.master.resources.requests.memory | string | `"128Mi"` |  |
| redis.networkPolicy.allowExternal | bool | `false` |  |
| redis.networkPolicy.enabled | bool | `true` |  |
| services.analyzer.autoscaling.enabled | bool | `false` |  |
| services.analyzer.category | string | `"ai-insights"` |  |
| services.analyzer.enabled | bool | `true` |  |
| services.analyzer.env.ANALYZER_AUTO_COOLDOWN_SEC | string | `"600"` |  |
| services.analyzer.env.ANALYZER_CHANGE_RISK_MIN_SPAN_SEC | string | `"259200"` |  |
| services.analyzer.env.ANALYZER_CHANGE_VELOCITY_PER_DAY | string | `"20"` |  |
| services.analyzer.env.ANALYZER_CHARS_PER_TOKEN | string | `"3.5"` |  |
| services.analyzer.env.ANALYZER_CONFIG_POLL_SEC | string | `"30"` |  |
| services.analyzer.env.ANALYZER_CONTEXT_TOKENS | string | `"4096"` |  |
| services.analyzer.env.ANALYZER_EMIT_TIMEOUT_SEC | string | `"180"` |  |
| services.analyzer.env.ANALYZER_LOOP_TIMEOUT_SEC | string | `"120"` |  |
| services.analyzer.env.ANALYZER_MANUAL_COOLDOWN_SEC | string | `"60"` |  |
| services.analyzer.env.ANALYZER_MAX_STEPS | string | `"8"` |  |
| services.analyzer.env.ANALYZER_MAX_TOOL_CALLS | string | `"8"` |  |
| services.analyzer.env.ANALYZER_MODE | string | `"fast"` |  |
| services.analyzer.env.ANALYZER_NARRATE_TIMEOUT_SEC | string | `"45"` |  |
| services.analyzer.env.ANALYZER_NUM_THREAD | string | `"2"` |  |
| services.analyzer.env.ANALYZER_PRODUCTION_PATTERN | string | `(^\|[-_.])(prod\|production\|prd)($\|[-_.])` | Namespaces or plan environments matching it count as production. |
| services.analyzer.env.ANALYZER_QUEUE_MAX | string | `"100"` |  |
| services.analyzer.env.ANALYZER_REVIEW_APPS_PER_MIN | string | `"20"` |  |
| services.analyzer.env.ANALYZER_REVIEW_INTERVAL_SEC | string | `"7200"` |  |
| services.analyzer.env.ANALYZER_REVIEW_TICK_SEC | string | `"120"` |  |
| services.analyzer.env.ANALYZER_REVIEW_WORKLOADS_MAX | string | `"10"` |  |
| services.analyzer.env.ANALYZER_TOOL_RESULT_MAX_BYTES | string | `"2048"` |  |
| services.analyzer.env.ANALYZER_USAGE_MIN_SAMPLES | string | `"12"` |  |
| services.analyzer.env.ANALYZER_USAGE_MIN_SPAN_SEC | string | `"43200"` |  |
| services.analyzer.env.ANALYZER_WALL_SEC | string | `"480"` |  |
| services.analyzer.env.CORS_ALLOWED_ORIGINS | string | `""` |  |
| services.analyzer.env.OLLAMA_AUTO_PULL | string | `"{{ .Values.app.ollama.autoPull }}"` |  |
| services.analyzer.env.OLLAMA_HOST | string | `"{{ default (printf \"http://%s-ollama:11434\" .Release.Name) .Values.app.ollama.runtimeUrl }}"` |  |
| services.analyzer.env.OLLAMA_PRUNE_MODELS | string | `"{{ and .Values.app.ollama.enabled (empty .Values.app.ollama.runtimeUrl) }}"` |  |
| services.analyzer.env.REDIS_POOL_SIZE | string | `"10"` |  |
| services.analyzer.name | string | `"analyzer-service"` |  |
| services.analyzer.pdb.enabled | bool | `false` |  |
| services.analyzer.repository | string | `"analyzer"` |  |
| services.auth.automountServiceAccountToken | bool | `false` |  |
| services.auth.category | string | `"auth"` |  |
| services.auth.enabled | bool | `true` |  |
| services.auth.env.BOOTSTRAP_ADMIN | string | `"{{ .Values.app.auth.bootstrap.admin }}"` |  |
| services.auth.env.CHALLENGE_TIMEOUT | string | `"60"` |  |
| services.auth.env.CLEANUP_DEDUP_TTL_SECONDS | string | `"600"` |  |
| services.auth.env.CLEANUP_JOB_MAX_ATTEMPTS | string | `"5"` |  |
| services.auth.env.CLEANUP_LAG_ALERT_THRESHOLD | string | `"500"` |  |
| services.auth.env.CLEANUP_LIST_TIMEOUT_SECONDS | string | `"10"` |  |
| services.auth.env.CLEANUP_MAX_CONCURRENT_PATCHES | string | `"4"` |  |
| services.auth.env.CLEANUP_PATCH_TIMEOUT_SECONDS | string | `"5"` |  |
| services.auth.env.CLEANUP_STREAM_MAXLEN | string | `"10000"` |  |
| services.auth.env.CLEANUP_SWEEPER_INTERVAL_SECONDS | string | `"60"` |  |
| services.auth.env.CLEANUP_WORKERS_PER_TYPE | string | `"2"` |  |
| services.auth.env.CLEANUP_XCLAIM_MIN_IDLE_SECONDS | string | `"60"` |  |
| services.auth.env.CORS_ALLOWED_ORIGINS | string | `""` |  |
| services.auth.env.ENROLL_INVITE_TTL_SEC | string | `"3600"` |  |
| services.auth.env.OIDC_TRUST_FILE | string | `"/etc/telark/oidc/googleJwkJson"` |  |
| services.auth.env.RECONCILE_BACKOFF_INITIAL_SECONDS | string | `"5"` |  |
| services.auth.env.RECONCILE_BACKOFF_MAX_SECONDS | string | `"300"` |  |
| services.auth.env.RECONCILE_PASS_DEADLINE_SECONDS | string | `"30"` |  |
| services.auth.env.RECONCILE_TICK_SECONDS | string | `"5"` |  |
| services.auth.env.REDIS_DB | string | `"1"` |  |
| services.auth.env.REDIS_MAX_WAIT_SEC | string | `"30"` |  |
| services.auth.env.REDIS_PING_TIMEOUT_SEC | string | `"3"` |  |
| services.auth.env.REDIS_POOL_SIZE | string | `"16"` |  |
| services.auth.env.REDIS_RETRY_INTERVAL_SEC | string | `"2"` |  |
| services.auth.env.RP_ID | string | `"{{ .Values.app.auth.passkey.id }}"` |  |
| services.auth.env.RP_NAME | string | `"{{ .Values.app.auth.passkey.name }}"` |  |
| services.auth.env.RP_ORIGIN | string | `"{{ .Values.app.auth.passkey.origin }}"` |  |
| services.auth.env.SESSION_EXPIRY | string | `"24"` |  |
| services.auth.name | string | `"auth-service"` |  |
| services.auth.pdb.enabled | bool | `false` |  |
| services.auth.repository | string | `"auth"` |  |
| services.auth.terminationGracePeriodSec | int | `30` |  |
| services.auth.volumeMounts[0].name | string | `"oidc-trust"` |  |
| services.auth.volumeMounts[0].path | string | `"/etc/telark/oidc"` |  |
| services.auth.volumeMounts[0].readOnly | bool | `true` |  |
| services.auth.volumes[0].name | string | `"oidc-trust"` |  |
| services.auth.volumes[0].secret.name | string | `"{{ include \"telark.oidcTrustSecretName\" . }}"` |  |
| services.discovery.category | string | `"discovery"` |  |
| services.discovery.enabled | bool | `true` |  |
| services.discovery.env.COORDINATION_BATCH_BLOCK_SEC | string | `"2"` |  |
| services.discovery.env.COORDINATION_BATCH_SIZE | string | `"10"` |  |
| services.discovery.env.COORDINATION_DEDUP_TTL_SEC | string | `"60"` |  |
| services.discovery.env.COORDINATION_ELECTION_RENEW_SEC | string | `"5"` |  |
| services.discovery.env.COORDINATION_ELECTION_TTL_SEC | string | `"15"` |  |
| services.discovery.env.COORDINATION_LOCK_HEARTBEAT_SEC | string | `"30"` |  |
| services.discovery.env.COORDINATION_LOCK_TTL_SEC | string | `"120"` |  |
| services.discovery.env.COORDINATION_MAX_RETRY_ATTEMPTS | string | `"5"` |  |
| services.discovery.env.COORDINATION_SHUTDOWN_CLEANUP_TIMEOUT_SEC | string | `"45"` |  |
| services.discovery.env.COORDINATION_STALE_CLAIM_INTERVAL_SEC | string | `"60"` |  |
| services.discovery.env.COORDINATION_STALE_CLAIM_MIN_IDLE_SEC | string | `"300"` |  |
| services.discovery.env.CORS_ALLOWED_ORIGINS | string | `""` |  |
| services.discovery.env.DISCOVERY_AUTO_CLEANUP_CYCLE_INTERVAL_SEC | string | `"60"` |  |
| services.discovery.env.DISCOVERY_AUTO_CLEANUP_DELETE_ENABLED | string | `"true"` |  |
| services.discovery.env.DISCOVERY_AUTO_CLEANUP_EMPTY_CYCLES_REQUIRED | string | `"2"` |  |
| services.discovery.env.DISCOVERY_AUTO_CLEANUP_ENABLED | string | `"true"` |  |
| services.discovery.env.DISCOVERY_AUTO_CLEANUP_GRACE_PERIOD_SEC | string | `"0"` |  |
| services.discovery.env.DISCOVERY_COALESCE_BUFFER_MAX_ENTRIES | string | `"500"` |  |
| services.discovery.env.DISCOVERY_INFORMER_COALESCING_MAX_WAIT_SEC | string | `"10"` |  |
| services.discovery.env.DISCOVERY_INFORMER_COALESCING_WINDOW_SEC | string | `"5"` |  |
| services.discovery.env.DISCOVERY_INFORMER_RESYNC_JITTER_FRACTION | string | `"0.2"` |  |
| services.discovery.env.DISCOVERY_INFORMER_RESYNC_SEC | string | `"600"` |  |
| services.discovery.env.DISCOVERY_K8S_CLIENT_BURST | string | `"200"` |  |
| services.discovery.env.DISCOVERY_K8S_CLIENT_QPS | string | `"100"` |  |
| services.discovery.env.DISCOVERY_ROLLBACK_INFORMER_RESYNC_SEC | string | `"600"` |  |
| services.discovery.env.DISCOVERY_ROLLBACK_K8S_CLIENT_BURST | string | `"200"` |  |
| services.discovery.env.DISCOVERY_ROLLBACK_K8S_CLIENT_QPS | string | `"100"` |  |
| services.discovery.env.DISCOVERY_SNAPSHOT_FETCH_TIMEOUT_MS | string | `"5000"` |  |
| services.discovery.env.FORCE_SYNC_ACK_RETENTION_SEC | string | `"3600"` |  |
| services.discovery.env.FORCE_SYNC_DEDUP_TTL_SEC | string | `"600"` |  |
| services.discovery.env.FORCE_SYNC_JOB_TIMEOUT_SEC | string | `"300"` |  |
| services.discovery.env.FORCE_SYNC_MAINTENANCE_INTERVAL_SEC | string | `"60"` |  |
| services.discovery.env.FORCE_SYNC_PEL_IDLE_RECLAIM_SEC | string | `"60"` |  |
| services.discovery.env.FORCE_SYNC_STREAM_MAX_LEN | string | `"5000"` |  |
| services.discovery.env.FORCE_SYNC_WORKERS | string | `"6"` |  |
| services.discovery.env.INSIGHTS_INDEX_REFRESH_SEC | string | `"15"` |  |
| services.discovery.env.INSIGHTS_INDEX_RESYNC_SEC | string | `"300"` |  |
| services.discovery.env.INSIGHTS_STALE_AFTER_SEC | string | `"86400"` |  |
| services.discovery.env.PROTECTION_PLAN_REPORT_CHECKPOINT_SEC | string | `"900"` |  |
| services.discovery.env.PROTECTION_PLAN_REPORT_MAX_VIOLATIONS | string | `"5000"` |  |
| services.discovery.env.PROTECTION_PLAN_TICK_INTERVAL_SEC | string | `"31"` |  |
| services.discovery.env.REDIS_MAX_WAIT_SEC | string | `"180"` |  |
| services.discovery.env.REDIS_PING_TIMEOUT_SEC | string | `"2"` |  |
| services.discovery.env.REDIS_POOL_SIZE | string | `"20"` |  |
| services.discovery.env.REDIS_RETRY_INTERVAL_SEC | string | `"5"` |  |
| services.discovery.env.REST_EXPORTER_DURATION_LOG_DEDUP_SEC | string | `"10"` |  |
| services.discovery.env.REST_EXPORTER_DURATION_LOG_ENABLED | string | `"true"` |  |
| services.discovery.env.SELF_MONITORING_ENABLED | string | `"{{ .Values.app.selfMonitoring.enabled }}"` |  |
| services.discovery.env.SNAPSHOT_WRITE_MAX_ATTEMPTS | string | `"5"` |  |
| services.discovery.env.SNAPSHOT_WRITE_RETRY_INTERVAL_SEC | string | `"2"` |  |
| services.discovery.name | string | `"discovery-service"` |  |
| services.discovery.natsUser | string | `"publisher"` |  |
| services.discovery.pdb.enabled | bool | `false` |  |
| services.discovery.repository | string | `"discovery"` |  |
| services.discovery.topologySpread.enabled | bool | `true` |  |
| services.discovery.topologySpread.maxSkew | int | `1` |  |
| services.discovery.topologySpread.topologyKey | string | `"kubernetes.io/hostname"` |  |
| services.discovery.topologySpread.whenUnsatisfiable | string | `"ScheduleAnyway"` |  |
| services.discovery.useNatsCreds | bool | `true` |  |
| services.exporter.autoscaling.enabled | bool | `false` |  |
| services.exporter.category | string | `"persistence-manager"` |  |
| services.exporter.enabled | bool | `true` |  |
| services.exporter.env.BOOTSTRAP_ADMIN | string | `"{{ .Values.app.auth.bootstrap.admin }}"` |  |
| services.exporter.env.CORS_ALLOWED_ORIGINS | string | `""` |  |
| services.exporter.env.EXPORTER_K8S_CLIENT_BURST | string | `"100"` |  |
| services.exporter.env.EXPORTER_K8S_CLIENT_QPS | string | `"50"` |  |
| services.exporter.env.EXPORTER_LIST_RENDER_CONCURRENCY | string | `"2"` |  |
| services.exporter.env.OIDC_TRUST_SECRET_NAME | string | `"{{ include \"telark.oidcTrustSecretName\" . }}"` |  |
| services.exporter.env.REPORTS_PATH | string | `"/reports"` |  |
| services.exporter.env.SNAPSHOTS_PATH | string | `"/snapshots"` |  |
| services.exporter.env.SNAPSHOTS_PVC_NAME | string | `"{{ include \"telark.exporterSnapshotsPvcName\" . }}"` |  |
| services.exporter.env.SNAPSHOTS_PVC_NAMESPACE | string | `"{{ .Values.app.namespace }}"` |  |
| services.exporter.env.SNAPSHOT_GC_INTERVAL_SEC | string | `"3600"` |  |
| services.exporter.name | string | `"exporter-service"` |  |
| services.exporter.pdb.enabled | bool | `false` |  |
| services.exporter.replicas | int | `1` |  |
| services.exporter.repository | string | `"exporter"` |  |
| services.exporter.volumeMounts[0].name | string | `"snapshots-storage"` |  |
| services.exporter.volumeMounts[0].path | string | `"/snapshots"` |  |
| services.exporter.volumeMounts[1].name | string | `"reports-storage"` |  |
| services.exporter.volumeMounts[1].path | string | `"/reports"` |  |
| services.exporter.volumes[0].name | string | `"snapshots-storage"` |  |
| services.exporter.volumes[0].persistentVolumeClaim.claimName | string | `"{{ include \"telark.exporterSnapshotsPvcName\" . }}"` |  |
| services.exporter.volumes[1].name | string | `"reports-storage"` |  |
| services.exporter.volumes[1].persistentVolumeClaim.claimName | string | `"{{ include \"telark.exporterReportsPvcName\" . }}"` |  |
| services.notifier.automountServiceAccountToken | bool | `false` |  |
| services.notifier.category | string | `"event-streaming"` |  |
| services.notifier.enabled | bool | `true` |  |
| services.notifier.env.NOTIFIER_APPLY_WORKERS | string | `"8"` |  |
| services.notifier.name | string | `"notifier-service"` |  |
| services.notifier.natsUser | string | `"consumer"` |  |
| services.notifier.pdb.enabled | bool | `false` |  |
| services.notifier.repository | string | `"notifier"` |  |
| services.notifier.terminationGracePeriodSec | int | `30` |  |
| services.notifier.useNatsCreds | bool | `true` |  |
| services.ui.automountServiceAccountToken | bool | `false` |  |
| services.ui.category | string | `"ui"` |  |
| services.ui.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| services.ui.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| services.ui.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| services.ui.enabled | bool | `true` |  |
| services.ui.healthCheck.livenessProbe.path | string | `"/healthz"` |  |
| services.ui.healthCheck.readinessProbe.path | string | `"/healthz"` |  |
| services.ui.healthCheck.startupProbe.path | string | `"/healthz"` |  |
| services.ui.includeSecurity | bool | `false` |  |
| services.ui.name | string | `"ui-service"` |  |
| services.ui.podSecurityContext.runAsGroup | int | `101` |  |
| services.ui.podSecurityContext.runAsNonRoot | bool | `true` |  |
| services.ui.podSecurityContext.runAsUser | int | `101` |  |
| services.ui.podSecurityContext.seccompProfile.type | string | `"RuntimeDefault"` |  |
| services.ui.repository | string | `"ui"` |  |
| services.ui.serviceToken | bool | `false` |  |
| services.ui.terminationGracePeriodSec | int | `30` |  |
| services.ui.useRedis | bool | `false` |  |
| services.ui.volumeMounts[0].name | string | `"nginx-cache"` |  |
| services.ui.volumeMounts[0].path | string | `"/var/cache/nginx"` |  |
| services.ui.volumeMounts[1].name | string | `"tmp"` |  |
| services.ui.volumeMounts[1].path | string | `"/tmp"` |  |
| services.ui.volumes[0].emptyDir | object | `{}` |  |
| services.ui.volumes[0].name | string | `"nginx-cache"` |  |
| services.ui.volumes[1].emptyDir | object | `{}` |  |
| services.ui.volumes[1].name | string | `"tmp"` |  |
| vpa.enabled | bool | `false` |  |

