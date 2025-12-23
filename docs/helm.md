# argocd-monorepo-controller

![Version: v0.0.4](https://img.shields.io/badge/Version-v0.0.4-informational?style=flat-square) ![AppVersion: v0.0.4](https://img.shields.io/badge/AppVersion-v0.0.4-informational?style=flat-square)

A Helm chart for Argocd Monorepo Controller, an ArgoCD addon that accurately tracks last commits that actually changed the application

**Homepage:** <https://github.com/argoproj-labs/argocd-monorepo-controller>

## Source Code

* <https://github.com/argoproj-labs/argocd-monorepo-controller/tree/main/helm>

## Requirements

Kubernetes: `>=1.25.0-0`

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| configs.params.annotations | object | `{}` | Annotations to be added to the argocd-cmd-params-cm ConfigMap |
| configs.params.create | bool | `true` | Create the argocd-cmd-params-cm configmap If false, it is expected the configmap will be created by something else. |
| controller.affinity | object | `{}` (defaults to global.affinity preset) | Assign custom [affinity] rules to the deployment |
| controller.automountServiceAccountToken | bool | `true` | Automount API credentials for the Service Account into the pod. |
| controller.clusterRoleRules.enabled | bool | `false` | Enable custom rules for the argocd monorepo controller's ClusterRole resource |
| controller.clusterRoleRules.rules | list | `[]` | List of custom rules for the argocd monorepo controller's ClusterRole resource |
| controller.containerPorts.metrics | int | `8090` | Metrics container port |
| controller.containerSecurityContext | object | See [values.yaml] | monorepo controller container-level security context |
| controller.deploymentAnnotations | object | `{}` | Annotations for the argocd monorepo controller Deployment |
| controller.deploymentLabels | object | `{}` | Labels for the argocd monorepo controller Deployment |
| controller.dnsConfig | object | `{}` | [DNS configuration] |
| controller.dnsPolicy | string | `"ClusterFirst"` | Alternative DNS policy for argocd monorepo controller pods |
| controller.emptyDir.sizeLimit | string | `""` (defaults not set if not specified i.e. no size limit) | EmptyDir size limit for argocd monorepo controller |
| controller.env | list | `[]` | Environment variables to pass to argocd monorepo controller |
| controller.envFrom | list | `[]` (See [values.yaml]) | envFrom to pass to argocd monorepo controller |
| controller.extraArgs | list | `[]` | Additional command line arguments to pass to argocd monorepo controller |
| controller.extraContainers | list | `[]` | Additional containers to be added to the argocd monorepo controller pod |
| controller.hostNetwork | bool | `false` | Host Network for argocd monorepo controller pods |
| controller.image.imagePullPolicy | string | `""` (defaults to global.image.imagePullPolicy) | Image pull policy for the argocd monorepo controller |
| controller.image.repository | string | `""` (defaults to global.image.repository) | Repository to use for the argocd monorepo controller |
| controller.image.tag | string | `""` (defaults to global.image.tag) | Tag to use for the argocd monorepo controller |
| controller.imagePullSecrets | list | `[]` (defaults to global.imagePullSecrets) | Secrets with credentials to pull images from a private registry |
| controller.initContainers | list | `[]` | Init containers to add to the argocd monorepo controller pod |
| controller.metrics.enabled | bool | `false` | Deploy metrics service |
| controller.metrics.rules.additionalLabels | object | `{}` | PrometheusRule labels |
| controller.metrics.rules.annotations | object | `{}` | PrometheusRule annotations |
| controller.metrics.rules.enabled | bool | `false` | Deploy a PrometheusRule for the argocd monorepo controller |
| controller.metrics.rules.namespace | string | `""` | PrometheusRule namespace |
| controller.metrics.rules.selector | object | `{}` | PrometheusRule selector |
| controller.metrics.rules.spec | list | `[]` | PrometheusRule.Spec for the argocd monorepo controller |
| controller.metrics.scrapeTimeout | string | `""` | Prometheus ServiceMonitor scrapeTimeout. If empty, Prometheus uses the global scrape timeout unless it is less than the target's scrape interval value in which the latter is used. |
| controller.metrics.service.annotations | object | `{}` | Metrics service annotations |
| controller.metrics.service.clusterIP | string | `""` | Metrics service clusterIP. `None` makes a "headless service" (no virtual IP) |
| controller.metrics.service.labels | object | `{}` | Metrics service labels |
| controller.metrics.service.portName | string | `"http-metrics"` | Metrics service port name |
| controller.metrics.service.servicePort | int | `8090` | Metrics service port |
| controller.metrics.service.type | string | `"ClusterIP"` | Metrics service type |
| controller.metrics.serviceMonitor.additionalLabels | object | `{}` | Prometheus ServiceMonitor labels |
| controller.metrics.serviceMonitor.annotations | object | `{}` | Prometheus ServiceMonitor annotations |
| controller.metrics.serviceMonitor.enabled | bool | `true` | Enable a prometheus ServiceMonitor |
| controller.metrics.serviceMonitor.honorLabels | bool | `false` | When true, honorLabels preserves the metric’s labels when they collide with the target’s labels. |
| controller.metrics.serviceMonitor.interval | string | `"30s"` | Prometheus ServiceMonitor interval |
| controller.metrics.serviceMonitor.metricRelabelings | list | `[]` | Prometheus [MetricRelabelConfigs] to apply to samples before ingestion |
| controller.metrics.serviceMonitor.namespace | string | `""` | Prometheus ServiceMonitor namespace |
| controller.metrics.serviceMonitor.relabelings | list | `[]` | Prometheus [RelabelConfigs] to apply to samples before scraping |
| controller.metrics.serviceMonitor.scheme | string | `""` | Prometheus ServiceMonitor scheme |
| controller.metrics.serviceMonitor.selector | object | `{}` | Prometheus ServiceMonitor selector |
| controller.metrics.serviceMonitor.tlsConfig | object | `{}` | Prometheus ServiceMonitor tlsConfig |
| controller.name | string | `"monorepo-controller"` | Controller name string |
| controller.networkPolicy.create | bool | `false` (defaults to global.networkPolicy.create) | Default network policy rules used by argocd monorepo controller |
| controller.nodeSelector | object | `{}` (defaults to global.nodeSelector) | [Node selector] |
| controller.pdb.annotations | object | `{}` | Annotations to be added to argocd monorepo controller pdb |
| controller.pdb.enabled | bool | `false` | Deploy a [PodDisruptionBudget] for the argocd monorepo controller |
| controller.pdb.labels | object | `{}` | Labels to be added to argocd monorepo controller pdb |
| controller.pdb.maxUnavailable | string | `""` | Number of pods that are unavailable after eviction as number or percentage (eg.: 50%). |
| controller.pdb.minAvailable | string | `""` (defaults to 0 if not specified) | Number of pods that are available after eviction as number or percentage (eg.: 50%) |
| controller.podAnnotations | object | `{}` | Annotations to be added to argocd monorepo controller pods |
| controller.podLabels | object | `{}` | Labels to be added to argocd monorepo controller pods |
| controller.priorityClassName | string | `""` (defaults to global.priorityClassName) | Priority class for the argocd monorepo controller pods |
| controller.readinessProbe.failureThreshold | int | `3` | Minimum consecutive failures for the [probe] to be considered failed after having succeeded |
| controller.readinessProbe.initialDelaySeconds | int | `10` | Number of seconds after the container has started before [probe] is initiated |
| controller.readinessProbe.periodSeconds | int | `10` | How often (in seconds) to perform the [probe] |
| controller.readinessProbe.successThreshold | int | `1` | Minimum consecutive successes for the [probe] to be considered successful after having failed |
| controller.readinessProbe.timeoutSeconds | int | `1` | Number of seconds after which the [probe] times out |
| controller.replicas | int | `1` | The number of argocd monorepo controller pods to run. |
| controller.resources | object | `{}` | Resource limits and requests for the argocd monorepo controller pods |
| controller.revisionHistoryLimit | int | `5` | Maximum number of controller revisions that will be maintained |
| controller.roleRules | list | `[]` | List of custom rules for the argocd monorepo controller's Role resource |
| controller.runtimeClassName | string | `""` (defaults to global.runtimeClassName) | Runtime class name for the argocd monorepo controller |
| controller.serviceAccount.annotations | object | `{}` | Annotations applied to created service account |
| controller.serviceAccount.automountServiceAccountToken | bool | `true` | Automount API credentials for the Service Account |
| controller.serviceAccount.create | bool | `true` | Create a service account for the argocd monorepo controller |
| controller.serviceAccount.labels | object | `{}` | Labels applied to created service account |
| controller.serviceAccount.name | string | `"argocd-monorepo-controller"` | Service account name |
| controller.terminationGracePeriodSeconds | int | `30` | terminationGracePeriodSeconds for container lifecycle hook |
| controller.tolerations | list | `[]` (defaults to global.tolerations) | [Tolerations] for use with node taints |
| controller.topologySpreadConstraints | list | `[]` (defaults to global.topologySpreadConstraints) | Assign custom [TopologySpreadConstraints] rules to the argocd monorepo controller |
| controller.volumeMounts | list | `[]` | Additional volumeMounts to the argocd monorepo controller main container |
| controller.volumes | list | `[]` | Additional volumes to the argocd monorepo controller pod |
| createClusterRoles | bool | `true` | Create cluster roles for cluster-wide installation. |
| externalRedis.existingSecret | string | `""` | The name of an existing secret with Redis (must contain key `redis-password`. And should contain `redis-username` if username is not `default`) and Sentinel credentials. When it's set, the `externalRedis.username` and `externalRedis.password` parameters are ignored |
| externalRedis.host | string | `""` | External Redis server host |
| externalRedis.password | string | `""` | External Redis password |
| externalRedis.port | int | `6379` | External Redis server port |
| externalRedis.secretAnnotations | object | `{}` | External Redis Secret annotations |
| externalRedis.username | string | `""` | External Redis username |
| global.addPrometheusAnnotations | bool | `false` | Add Prometheus scrape annotations to all metrics services. This can be used as an alternative to the ServiceMonitors. |
| global.additionalLabels | object | `{}` | Common labels for the all resources |
| global.affinity.nodeAffinity.matchExpressions | list | `[]` | Default match expressions for node affinity |
| global.affinity.nodeAffinity.type | string | `"hard"` | Default node affinity rules. Either: `none`, `soft` or `hard` |
| global.affinity.podAntiAffinity | string | `"soft"` | Default pod anti-affinity rules. Either: `none`, `soft` or `hard` |
| global.certificateAnnotations | object | `{}` | Annotations for the all deployed Certificates |
| global.deploymentAnnotations | object | `{}` | Annotations for the all deployed Deployments |
| global.deploymentLabels | object | `{}` | Labels for the all deployed Deployments |
| global.deploymentStrategy | object | `{}` | Deployment strategy for the all deployed Deployments |
| global.dualStack.ipFamilies | list | `[]` | IP families that should be supported and the order in which they should be applied to ClusterIP as well. Can be IPv4 and/or IPv6. |
| global.dualStack.ipFamilyPolicy | string | `""` | IP family policy to configure dual-stack see [Configure dual-stack](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services) |
| global.env | list | `[]` | Environment variables to pass to all deployed Deployments |
| global.hostAliases | list | `[]` | Mapping between IP and hostnames that will be injected as entries in the pod's hosts files |
| global.image.imagePullPolicy | string | `"IfNotPresent"` | If defined, a imagePullPolicy applied to all Argo CD deployments |
| global.image.repository | string | `"quay.io/argoprojlabs/argocd-monorepo-controller"` | If defined, a repository applied to all Argo CD deployments |
| global.image.tag | string | `"v0.0.4"` | Overrides the global Argo CD image tag whose default is the chart appVersion |
| global.imagePullSecrets | list | `[]` | Secrets with credentials to pull images from a private registry |
| global.logging.format | string | `"text"` | Set the global logging format. Either: `text` or `json` |
| global.logging.level | string | `"info"` | Set the global logging level. One of: `debug`, `info`, `warn` or `error` |
| global.networkPolicy.create | bool | `false` | Create NetworkPolicy objects for all components |
| global.networkPolicy.defaultDenyIngress | bool | `false` | Default deny all ingress traffic |
| global.nodeSelector | object | `{"kubernetes.io/os":"linux"}` | Default node selector for all components |
| global.podAnnotations | object | `{}` | Annotations for the all deployed pods |
| global.podLabels | object | `{}` | Labels for the all deployed pods |
| global.priorityClassName | string | `""` | Default priority class for all components |
| global.revisionHistoryLimit | int | `3` | Number of old deployment ReplicaSets to retain. The rest will be garbage collected. |
| global.runtimeClassName | string | `""` | Runtime class name for all components |
| global.securityContext | object | `{}` (See [values.yaml]) | Toggle and define pod-level security context. |
| global.tolerations | list | `[]` | Default tolerations for all components |
| global.topologySpreadConstraints | list | `[]` | Default [TopologySpreadConstraints] rules for all components |
| nameOverride | string | `"argocd"` | Provide a name in place of `argocd` |
| openshift.enabled | bool | `false` | enables using arbitrary uid for argocd monorepo repo server |
| redisSecretInit.enabled | bool | `true` | Enable Redis secret initialization. If disabled, secret must be provisioned by alternative methods |
| redisSecretInit.image.imagePullPolicy | string | `""` (defaults to global.image.imagePullPolicy) | Image pull policy for the Redis secret-init Job |
| redisSecretInit.image.repository | string | `""` (defaults to global.image.repository) | Repository to use for the Redis secret-init Job |
| redisSecretInit.image.tag | string | `""` (defaults to global.image.tag) | Tag to use for the Redis secret-init Job |
| redisSecretInit.name | string | `"redis-secret-init"` | Redis secret-init name |
| repoServer.affinity | object | `{}` (defaults to global.affinity preset) | Assign custom [affinity] rules to the deployment |
| repoServer.automountServiceAccountToken | bool | `true` | Automount API credentials for the Service Account into the pod. |
| repoServer.autoscaling.behavior | object | `{}` | Configures the scaling behavior of the target in both Up and Down directions. |
| repoServer.autoscaling.enabled | bool | `false` | Enable Horizontal Pod Autoscaler ([HPA]) for the repo server |
| repoServer.autoscaling.maxReplicas | int | `5` | Maximum number of replicas for the repo server [HPA] |
| repoServer.autoscaling.metrics | list | `[]` | Configures custom HPA metrics for the Argo CD repo server Ref: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/ |
| repoServer.autoscaling.minReplicas | int | `1` | Minimum number of replicas for the repo server [HPA] |
| repoServer.autoscaling.targetCPUUtilizationPercentage | int | `50` | Average CPU utilization percentage for the repo server [HPA] |
| repoServer.autoscaling.targetMemoryUtilizationPercentage | int | `50` | Average memory utilization percentage for the repo server [HPA] |
| repoServer.certificateSecret.annotations | object | `{}` | Annotations to be added to argocd-repo-server-tls secret |
| repoServer.certificateSecret.ca | string | `""` | Certificate authority. Required for self-signed certificates. |
| repoServer.certificateSecret.crt | string | `""` | Certificate data. Must contain SANs of Repo service (ie: argocd-repo-server, argocd-repo-server.argo-cd.svc) |
| repoServer.certificateSecret.enabled | bool | `false` | Create argocd-repo-server-tls secret |
| repoServer.certificateSecret.key | string | `""` | Certificate private key |
| repoServer.certificateSecret.labels | object | `{}` | Labels to be added to argocd-repo-server-tls secret |
| repoServer.clusterRoleRules.enabled | bool | `false` | Enable custom rules for the Repo server's Cluster Role resource |
| repoServer.clusterRoleRules.rules | list | `[]` | List of custom rules for the Repo server's Cluster Role resource |
| repoServer.containerPorts.metrics | int | `8094` | Metrics container port |
| repoServer.containerPorts.server | int | `8091` | Repo server container port |
| repoServer.containerSecurityContext | object | See [values.yaml] | Repo server container-level security context |
| repoServer.copyutil.resources | object | `{}` | Resource limits and requests for the repo server copyutil initContainer |
| repoServer.deploymentAnnotations | object | `{}` | Annotations to be added to repo server Deployment |
| repoServer.deploymentLabels | object | `{}` | Labels for the repo server Deployment |
| repoServer.deploymentStrategy | object | `{}` | Deployment strategy to be added to the repo server Deployment |
| repoServer.dnsConfig | object | `{}` | [DNS configuration] |
| repoServer.dnsPolicy | string | `"ClusterFirst"` | Alternative DNS policy for Repo server pods |
| repoServer.emptyDir.sizeLimit | string | `""` (defaults not set if not specified i.e. no size limit) | EmptyDir size limit for repo server |
| repoServer.env | list | `[]` | Environment variables to pass to repo server |
| repoServer.envFrom | list | `[]` (See [values.yaml]) | envFrom to pass to repo server |
| repoServer.existingVolumes | object | `{}` | Volumes to be used in replacement of emptydir on default volumes |
| repoServer.extraArgs | list | `[]` | Additional command line arguments to pass to repo server |
| repoServer.extraContainers | list | `[]` | Additional containers to be added to the repo server pod |
| repoServer.hostNetwork | bool | `false` | Host Network for Repo server pods |
| repoServer.image.imagePullPolicy | string | `""` (defaults to global.image.imagePullPolicy) | Image pull policy for the repo server |
| repoServer.image.repository | string | `""` (defaults to global.image.repository) | Repository to use for the repo server |
| repoServer.image.tag | string | `""` (defaults to global.image.tag) | Tag to use for the repo server |
| repoServer.imagePullSecrets | list | `[]` (defaults to global.imagePullSecrets) | Secrets with credentials to pull images from a private registry |
| repoServer.initContainers | list | `[]` | Init containers to add to the repo server pods |
| repoServer.lifecycle | object | `{}` | Specify postStart and preStop lifecycle hooks for your argo-repo-server container |
| repoServer.livenessProbe.failureThreshold | int | `3` | Minimum consecutive failures for the [probe] to be considered failed after having succeeded |
| repoServer.livenessProbe.initialDelaySeconds | int | `10` | Number of seconds after the container has started before [probe] is initiated |
| repoServer.livenessProbe.periodSeconds | int | `10` | How often (in seconds) to perform the [probe] |
| repoServer.livenessProbe.successThreshold | int | `1` | Minimum consecutive successes for the [probe] to be considered successful after having failed |
| repoServer.livenessProbe.timeoutSeconds | int | `1` | Number of seconds after which the [probe] times out |
| repoServer.metrics.enabled | bool | `false` | Deploy metrics service |
| repoServer.metrics.service.annotations | object | `{}` | Metrics service annotations |
| repoServer.metrics.service.clusterIP | string | `""` | Metrics service clusterIP. `None` makes a "headless service" (no virtual IP) |
| repoServer.metrics.service.labels | object | `{}` | Metrics service labels |
| repoServer.metrics.service.portName | string | `"http-metrics"` | Metrics service port name |
| repoServer.metrics.service.servicePort | int | `8094` | Metrics service port |
| repoServer.metrics.service.type | string | `"ClusterIP"` | Metrics service type |
| repoServer.metrics.serviceMonitor.additionalLabels | object | `{}` | Prometheus ServiceMonitor labels |
| repoServer.metrics.serviceMonitor.annotations | object | `{}` | Prometheus ServiceMonitor annotations |
| repoServer.metrics.serviceMonitor.enabled | bool | `true` | Enable a prometheus ServiceMonitor |
| repoServer.metrics.serviceMonitor.honorLabels | bool | `false` | When true, honorLabels preserves the metric’s labels when they collide with the target’s labels. |
| repoServer.metrics.serviceMonitor.interval | string | `"30s"` | Prometheus ServiceMonitor interval |
| repoServer.metrics.serviceMonitor.metricRelabelings | list | `[]` | Prometheus [MetricRelabelConfigs] to apply to samples before ingestion |
| repoServer.metrics.serviceMonitor.namespace | string | `""` | Prometheus ServiceMonitor namespace |
| repoServer.metrics.serviceMonitor.relabelings | list | `[]` | Prometheus [RelabelConfigs] to apply to samples before scraping |
| repoServer.metrics.serviceMonitor.scheme | string | `""` | Prometheus ServiceMonitor scheme |
| repoServer.metrics.serviceMonitor.scrapeTimeout | string | `""` | Prometheus ServiceMonitor scrapeTimeout. If empty, Prometheus uses the global scrape timeout unless it is less than the target's scrape interval value in which the latter is used. |
| repoServer.metrics.serviceMonitor.selector | object | `{}` | Prometheus ServiceMonitor selector |
| repoServer.metrics.serviceMonitor.tlsConfig | object | `{}` | Prometheus ServiceMonitor tlsConfig |
| repoServer.name | string | `"monorepo-repo-server"` | Repo server name |
| repoServer.networkPolicy.create | bool | `false` (defaults to global.networkPolicy.create) | Default network policy rules used by repo server |
| repoServer.nodeSelector | object | `{}` (defaults to global.nodeSelector) | [Node selector] |
| repoServer.pdb.annotations | object | `{}` | Annotations to be added to repo server pdb |
| repoServer.pdb.enabled | bool | `true` | Deploy a [PodDisruptionBudget] for the repo server |
| repoServer.pdb.labels | object | `{}` | Labels to be added to repo server pdb |
| repoServer.pdb.maxUnavailable | string | `""` | Number of pods that are unavailable after eviction as number or percentage (eg.: 50%). |
| repoServer.pdb.minAvailable | string | `""` (defaults to 0 if not specified) | Number of pods that are available after eviction as number or percentage (eg.: 50%) |
| repoServer.podAnnotations | object | `{}` | Annotations to be added to repo server pods |
| repoServer.podLabels | object | `{}` | Labels to be added to repo server pods |
| repoServer.priorityClassName | string | `""` (defaults to global.priorityClassName) | Priority class for the repo server pods |
| repoServer.rbac | list | `[]` | Repo server rbac rules |
| repoServer.readinessProbe.failureThreshold | int | `3` | Minimum consecutive failures for the [probe] to be considered failed after having succeeded |
| repoServer.readinessProbe.initialDelaySeconds | int | `10` | Number of seconds after the container has started before [probe] is initiated |
| repoServer.readinessProbe.periodSeconds | int | `10` | How often (in seconds) to perform the [probe] |
| repoServer.readinessProbe.successThreshold | int | `1` | Minimum consecutive successes for the [probe] to be considered successful after having failed |
| repoServer.readinessProbe.timeoutSeconds | int | `1` | Number of seconds after which the [probe] times out |
| repoServer.replicas | int | `1` | The number of repo server pods to run |
| repoServer.resources | object | `{}` | Resource limits and requests for the repo server pods |
| repoServer.runtimeClassName | string | `""` (defaults to global.runtimeClassName) | Runtime class name for the repo server |
| repoServer.service.annotations | object | `{}` | Repo server service annotations |
| repoServer.service.labels | object | `{}` | Repo server service labels |
| repoServer.service.port | int | `8091` | Repo server service port |
| repoServer.service.portName | string | `"tcp-repo-server"` | Repo server service port name |
| repoServer.service.trafficDistribution | string | `""` | Traffic distribution preference for the repo server service. If the field is not set, the implementation will apply its default routing strategy. |
| repoServer.serviceAccount.annotations | object | `{}` | Annotations applied to created service account |
| repoServer.serviceAccount.automountServiceAccountToken | bool | `true` | Automount API credentials for the Service Account |
| repoServer.serviceAccount.create | bool | `false` | Create repo server service account |
| repoServer.serviceAccount.labels | object | `{}` | Labels applied to created service account |
| repoServer.serviceAccount.name | string | `""` | Repo server service account name |
| repoServer.terminationGracePeriodSeconds | int | `30` | terminationGracePeriodSeconds for container lifecycle hook |
| repoServer.tolerations | list | `[]` (defaults to global.tolerations) | [Tolerations] for use with node taints |
| repoServer.topologySpreadConstraints | list | `[]` (defaults to global.topologySpreadConstraints) | Assign custom [TopologySpreadConstraints] rules to the repo server |
| repoServer.useEphemeralHelmWorkingDir | bool | `true` | Toggle the usage of a ephemeral Helm working directory |
| repoServer.volumeMounts | list | `[]` | Additional volumeMounts to the repo server main container |
| repoServer.volumes | list | `[]` | Additional volumes to the repo server pod |
| server.name | string | `"server"` | Argo CD server name |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.9.1](https://github.com/norwoodj/helm-docs/releases/v1.9.1)
