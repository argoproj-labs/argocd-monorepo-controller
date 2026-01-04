# Monorepo Controller Metrics

Both Monorepo Controller and Monorepo Repo Server expose Prometheus metrics

## Argocd Monorepo Controler Metrics

| Metric                                           | Type      | Description                                                                |
|--------------------------------------------------|:---------:|----------------------------------------------------------------------------|
| `monorepo_app_info`                              | gauge     | Information about application.                                             |
| `monorepo_app_reconcile`                         | histogram | Application reconciliation performance in seconds.                         |
| `monorepo_repo_server_request_total`             | counter   | Number of repo server requests executed during application reconciliation. |
| `monorepo_repo_server_request_duration`          | histogram | Repo server requests duration.                                             |
| `monorepo_kubectl_requests_total`                | counter   | Number of kubectl request results",                                        |
| `monorepo_kubectl_request_size_bytes`            | histogram | Size of kubectl requests                                                   |
| `monorepo_kubectl_response_size_bytes`           | histogram | Size of kubectl responses                                                  |
| `monorepo_kubectl_rate_limiter_duration_seconds` | histogram | Kubectl rate limiter latency                                               |
| `monorepo_kubectl_request_duration_seconds`      | histogram | Request latency in seconds                                                 |



## ArgoCD Monorepo Repo Server Metrics

| Metric                                                | Type      | Description                                              |
|-------------------------------------------------------|:---------:|----------------------------------------------------------|
| `monorepo_getchangerevision_request_total`            | counter   | Number of GetChangeRevision requests executed.           |
| `monorepo_getchangerevision_request_duration_seconds` | histogram | GetChangeRevision requests duration seconds.             |
| `monorepo_git_request_total`                          | counter   | Number of git requests performed by repo server          |
| `monorepo_git_revlist_fail_total`                     | counter   | Number of git rev-list requests failures by repo server  |
| `monorepo_git_difftree_fail_total`                    | counter   | Number of git diff-tree requests failures by repo server |
| `monorepo_git_request_duration_seconds`               | histogram | Git requests duration seconds                            |
| `monorepo_repo_pending_request_total`                 | counter   | Number of pending requests requiring repository lock     |
| `monorepo_redis_request_total`                        | counter   | Number of redis requests executed.                       |
| `monorepo_redis_request_duration_seconds`             | histogram | Redis requests duration seconds.                         |

The server does not expose the gRPC metrics by default.  Those metrics can be enabled using `ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM=true`
 environment variable.


## Prometheus Operator

If using Prometheus Operator, the following ServiceMonitor [example manifests](https://github.com/argoproj-labs/argocd-monorepo-controller/tree/main/samples/metrics) can be used.  Add a namespace where Argo CD is
installed and change `metadata.labels.release` to the name of label selected by your Prometheus.

```yaml
{!docs/argocd-monorepo-controller-sm.yaml!}
```

```yaml
{!docs/argocd-monorepo-controller-sm.yaml!}
```

## Grafana Dashboard


You can find an example Grafana dashboard [here](https://github.com/argoproj-labs/argocd-monorepo-controller/blob/main/samples/dashboards/grafana-dashboard.json).

