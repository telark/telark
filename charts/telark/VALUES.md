# telark

![Version: 0.3.0](https://img.shields.io/badge/Version-0.3.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 2.0](https://img.shields.io/badge/AppVersion-2.0-informational?style=flat-square)

A protection gate for your Kubernetes workloads — discover your applications, then decide what can change them, and when

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
| file://../telark-crds | telark-crds | >= 0.0.0 |
| https://charts.bitnami.com/bitnami | nats | 9.0.28 |
| https://charts.bitnami.com/bitnami | redis | 23.0.10 |
| https://charts.fairwinds.com/stable | vpa | 5.1.0 |
| https://helm.otwld.com/ | ollama | 1.50.0 |
| https://kubernetes-sigs.github.io/metrics-server/ | metrics-server | 3.12.2 |
| https://kyverno.github.io/kyverno/ | kyverno | 3.7.1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| app.auth.bootstrap.admins[0] | string | `"contact@telark.io"` |  |
| app.auth.passkey.id | string | `""` |  |
| app.auth.passkey.name | string | `"Dashboard App"` |  |
| app.auth.passkey.origin | string | `""` |  |
| app.auth.passkey.selfRegistration | string | `"true"` |  |
| app.crdGuard.enabled | bool | `false` |  |
| app.crdGuard.enforce | bool | `false` |  |
| app.crdGuard.extraAllowedUsers | list | `[]` |  |
| app.image.pullPolicy | string | `"Always"` |  |
| app.image.pullSecrets | list | `[]` |  |
| app.image.registry | string | `"telark"` |  |
| app.kyverno.enabled | bool | `true` |  |
| app.mode | string | `"standard"` |  |
| app.name | string | `"telark"` |  |
| app.namespace | string | `"telark"` |  |
| app.ollama.enabled | bool | `false` |  |
| app.persistence.enabled | bool | `true` |  |
| app.persistence.size | string | `"10Gi"` |  |
| app.persistence.storageClass | string | `""` |  |
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
| app.serviceToken.value | string | `""` |  |
| app.shared.containerSecurityContext.allowPrivilegeEscalation | bool | `false` |  |
| app.shared.containerSecurityContext.capabilities.drop[0] | string | `"ALL"` |  |
| app.shared.containerSecurityContext.enabled | bool | `true` |  |
| app.shared.containerSecurityContext.readOnlyRootFilesystem | bool | `true` |  |
| app.shared.containerSecurityContext.runAsGroup | int | `1001` |  |
| app.shared.containerSecurityContext.runAsUser | int | `1001` |  |
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
| app.shared.nats.NATS_HOST | string | `"{{ .Release.Name }}-nats"` |  |
| app.shared.natsEnvFromSecret.NATS_PASSWORD.key | string | `"password"` |  |
| app.shared.natsEnvFromSecret.NATS_PASSWORD.name | string | `"nats"` |  |
| app.shared.natsEnvFromSecret.NATS_USER.key | string | `"username"` |  |
| app.shared.natsEnvFromSecret.NATS_USER.name | string | `"nats"` |  |
| app.shared.podSecurityContext.enabled | bool | `true` |  |
| app.shared.podSecurityContext.fsGroup | int | `1001` |  |
| app.shared.podSecurityContext.runAsGroup | int | `1001` |  |
| app.shared.podSecurityContext.runAsUser | int | `1001` |  |
| app.shared.redis.REDIS_HOST | string | `"{{ .Release.Name }}-redis-master"` |  |
| app.shared.redis.REDIS_PORT | string | `"6379"` |  |
| app.shared.resources.limits.cpu | string | `"500m"` |  |
| app.shared.resources.limits.memory | string | `"512Mi"` |  |
| app.shared.resources.requests.cpu | string | `"100m"` |  |
| app.shared.resources.requests.memory | string | `"128Mi"` |  |
| app.singleNode | bool | `false` |  |
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
| kyverno.admissionController.container.resources.requests.cpu | string | `"200m"` |  |
| kyverno.admissionController.container.resources.requests.memory | string | `"256Mi"` |  |
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
| metrics-server.args[0] | string | `"--kubelet-insecure-tls"` |  |
| metrics-server.args[1] | string | `"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"` |  |
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
| nats.configuration | string | `"server_name: nats-server\nport: 4222\njetstream {\n  store_dir: \"/data\"\n  max_mem: 1G\n  max_file: 5G\n}\nauthorization {\n  users = [\n    {\n      user: \"nats\",\n      password: $NATS_PASSWORD,\n      permissions: {\n        publish   = [\"telark.applications.*\", \"$JS.ACK.>\", \"$JS.API.>\", \"_INBOX.>\"]\n        subscribe = [\"telark.applications.*\", \"$JS.ACK.>\", \"$JS.API.>\", \"_INBOX.>\"]\n      }\n    }\n  ]\n}\nhttp_port: 8222\n"` |  |
| nats.enabled | bool | `true` |  |
| nats.extraEnvVars[0].name | string | `"NATS_PASSWORD"` |  |
| nats.extraEnvVars[0].valueFrom.secretKeyRef.key | string | `"password"` |  |
| nats.extraEnvVars[0].valueFrom.secretKeyRef.name | string | `"telark-nats-secret"` |  |
| nats.image.pullPolicy | string | `"IfNotPresent"` |  |
| nats.image.registry | string | `"docker.io"` |  |
| nats.image.repository | string | `"nats"` |  |
| nats.image.tag | string | `"2.12.1-scratch"` |  |
| nats.persistence.enabled | bool | `true` |  |
| nats.persistence.path | string | `"/data"` |  |
| nats.persistence.size | string | `"4Gi"` |  |
| nats.securityContext.fsGroup | int | `1000` |  |
| nats.securityContext.runAsGroup | int | `1000` |  |
| nats.securityContext.runAsUser | int | `1000` |  |
| nats.service.type | string | `"ClusterIP"` |  |
| ollama.extraEnv[0].name | string | `"OLLAMA_NO_CLOUD"` |  |
| ollama.extraEnv[0].value | string | `"true"` |  |
| ollama.extraEnv[1].name | string | `"OLLAMA_CONTEXT_LENGTH"` |  |
| ollama.extraEnv[1].value | string | `"2048"` |  |
| ollama.extraEnv[2].name | string | `"OLLAMA_KEEP_ALIVE"` |  |
| ollama.extraEnv[2].value | string | `"-1"` |  |
| ollama.extraEnv[3].name | string | `"OLLAMA_FLASH_ATTENTION"` |  |
| ollama.extraEnv[3].value | string | `"true"` |  |
| ollama.extraEnv[4].name | string | `"OLLAMA_NUM_PARALLEL"` |  |
| ollama.extraEnv[4].value | string | `"1"` |  |
| ollama.ollama.models.pull[0] | string | `"qwen2.5:3b"` |  |
| ollama.persistentVolume.enabled | bool | `true` |  |
| ollama.persistentVolume.size | string | `"10Gi"` |  |
| ollama.persistentVolume.storageClass | string | `""` |  |
| redis.architecture | string | `"standalone"` |  |
| redis.auth.enabled | bool | `false` |  |
| redis.master.persistence.enabled | bool | `true` |  |
| redis.master.persistence.size | string | `"4Gi"` |  |
| redis.networkPolicy.allowExternal | bool | `false` |  |
| redis.networkPolicy.enabled | bool | `true` |  |
| services.auth.category | string | `"auth"` |  |
| services.auth.enabled | bool | `true` |  |
| services.auth.env.BOOTSTRAP_ADMINS | string | `"{{ join \",\" .Values.app.auth.bootstrap.admins }}"` |  |
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
| services.auth.env.RECONCILE_BACKOFF_INITIAL_SECONDS | string | `"5"` |  |
| services.auth.env.RECONCILE_BACKOFF_MAX_SECONDS | string | `"300"` |  |
| services.auth.env.RECONCILE_PASS_DEADLINE_SECONDS | string | `"30"` |  |
| services.auth.env.RECONCILE_TICK_SECONDS | string | `"5"` |  |
| services.auth.env.REDIS_DB | string | `"1"` |  |
| services.auth.env.REDIS_MAX_WAIT_SEC | string | `"30"` |  |
| services.auth.env.REDIS_PING_TIMEOUT_SEC | string | `"3"` |  |
| services.auth.env.REDIS_RETRY_INTERVAL_SEC | string | `"2"` |  |
| services.auth.env.RP_ID | string | `"{{ .Values.app.auth.passkey.id }}"` |  |
| services.auth.env.RP_NAME | string | `"{{ .Values.app.auth.passkey.name }}"` |  |
| services.auth.env.RP_ORIGIN | string | `"{{ .Values.app.auth.passkey.origin }}"` |  |
| services.auth.env.SELF_REGISTRATION_ENABLED | string | `"{{ .Values.app.auth.passkey.selfRegistration }}"` |  |
| services.auth.env.SESSION_EXPIRY | string | `"24"` |  |
| services.auth.name | string | `"auth-service"` |  |
| services.auth.pdb.enabled | bool | `false` |  |
| services.auth.repository | string | `"auth"` |  |
| services.auth.terminationGracePeriodSec | int | `30` |  |
| services.discovery.category | string | `"sync"` |  |
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
| services.discovery.env.INSIGHTS_TICK_INTERVAL_SEC | string | `"300"` |  |
| services.discovery.env.PROTECTION_PLAN_TICK_INTERVAL_SEC | string | `"31"` |  |
| services.discovery.env.REDIS_MAX_WAIT_SEC | string | `"180"` |  |
| services.discovery.env.REDIS_PING_TIMEOUT_SEC | string | `"2"` |  |
| services.discovery.env.REDIS_RETRY_INTERVAL_SEC | string | `"5"` |  |
| services.discovery.env.REST_EXPORTER_DURATION_LOG_DEDUP_SEC | string | `"10"` |  |
| services.discovery.env.REST_EXPORTER_DURATION_LOG_ENABLED | string | `"true"` |  |
| services.discovery.env.SNAPSHOT_WRITE_MAX_ATTEMPTS | string | `"5"` |  |
| services.discovery.env.SNAPSHOT_WRITE_RETRY_INTERVAL_SEC | string | `"2"` |  |
| services.discovery.includeSecurity | bool | `false` |  |
| services.discovery.name | string | `"discovery-service"` |  |
| services.discovery.pdb.enabled | bool | `false` |  |
| services.discovery.repository | string | `"discovery"` |  |
| services.discovery.topologySpread.enabled | bool | `true` |  |
| services.discovery.topologySpread.maxSkew | int | `1` |  |
| services.discovery.topologySpread.topologyKey | string | `"kubernetes.io/hostname"` |  |
| services.discovery.topologySpread.whenUnsatisfiable | string | `"ScheduleAnyway"` |  |
| services.discovery.useNatsCreds | bool | `true` |  |
| services.enrichment.category | string | `"ai-inisghts"` |  |
| services.enrichment.enabled | bool | `true` |  |
| services.enrichment.env.ANTHROPIC_MODEL | string | `"claude-haiku-4-5-20251001"` |  |
| services.enrichment.env.GEMINI_MODEL | string | `"gemini-2.5-flash"` |  |
| services.enrichment.env.GROQ_MODEL | string | `"llama-3.3-70b-versatile"` |  |
| services.enrichment.env.METRICS_INTERVAL_S | string | `"60"` |  |
| services.enrichment.env.NUM_WORKERS | string | `"3"` |  |
| services.enrichment.env.OLLAMA_HOST | string | `"http://{{ .Release.Name }}-ollama:11434"` |  |
| services.enrichment.env.OLLAMA_MODEL | string | `"qwen2.5:3b"` |  |
| services.enrichment.env.REDIS_POOL_SIZE | string | `"10"` |  |
| services.enrichment.env.WORKER_SHUTDOWN_TIMEOUT_S | string | `"30"` |  |
| services.enrichment.name | string | `"enrichment-service"` |  |
| services.enrichment.pdb.enabled | bool | `false` |  |
| services.enrichment.repository | string | `"enrichment"` |  |
| services.exporter.autoscaling.enabled | bool | `false` |  |
| services.exporter.category | string | `"export"` |  |
| services.exporter.enabled | bool | `true` |  |
| services.exporter.env.AI_KEY_SECRET_NAME | string | `"{{ printf \"%s-ai-provider-key\" .Values.app.name }}"` |  |
| services.exporter.env.AI_KEY_SECRET_NAMESPACE | string | `"{{ .Values.app.namespace }}"` |  |
| services.exporter.env.EXPORTER_K8S_CLIENT_BURST | string | `"100"` |  |
| services.exporter.env.EXPORTER_K8S_CLIENT_QPS | string | `"50"` |  |
| services.exporter.env.SNAPSHOTS_PATH | string | `"/snapshots"` |  |
| services.exporter.env.SNAPSHOTS_PVC_NAME | string | `"{{ include \"telark.exporterSnapshotsPvcName\" . }}"` |  |
| services.exporter.env.SNAPSHOTS_PVC_NAMESPACE | string | `"{{ .Values.app.namespace }}"` |  |
| services.exporter.env.SNAPSHOT_GC_INTERVAL_SEC | string | `"3600"` |  |
| services.exporter.name | string | `"exporter-service"` |  |
| services.exporter.pdb.enabled | bool | `false` |  |
| services.exporter.replicas | int | `2` |  |
| services.exporter.repository | string | `"exporter"` |  |
| services.exporter.volumeMounts[0].name | string | `"snapshots-storage"` |  |
| services.exporter.volumeMounts[0].path | string | `"/snapshots"` |  |
| services.exporter.volumes[0].name | string | `"snapshots-storage"` |  |
| services.exporter.volumes[0].persistentVolumeClaim.claimName | string | `"{{ include \"telark.exporterSnapshotsPvcName\" . }}"` |  |
| services.notifier.category | string | `"notification"` |  |
| services.notifier.enabled | bool | `true` |  |
| services.notifier.env.NOTIFIER_APPLY_WORKERS | string | `"8"` |  |
| services.notifier.name | string | `"notifier-service"` |  |
| services.notifier.pdb.enabled | bool | `false` |  |
| services.notifier.repository | string | `"notifier"` |  |
| services.notifier.terminationGracePeriodSec | int | `30` |  |
| services.notifier.useNatsCreds | bool | `true` |  |
| services.ui.category | string | `"ui"` |  |
| services.ui.enabled | bool | `true` |  |
| services.ui.includeHealthCheck | bool | `false` |  |
| services.ui.includeSecurity | bool | `false` |  |
| services.ui.name | string | `"ui-service"` |  |
| services.ui.repository | string | `"ui"` |  |
| services.ui.terminationGracePeriodSec | int | `30` |  |
| services.ui.useRedis | bool | `false` |  |
| vpa.enabled | bool | `false` |  |

