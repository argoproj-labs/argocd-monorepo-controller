# Changelog

## v0.0.6 (2026-09-01)

### Features

- Multiple workers for handling applications updates
- Make timeout for handling application updates configurable
- Implement Revisions List requests cache
- Implement Diff Tree requests cache
- Add status label to the reconcilation metrics

### Fixes

- Add missing NetworkPolicy Helm template for connection to Redis
- Remove image tag from Helm values.yaml (use App version from Chart)
- Fix ARGOCD_MONOREPO_REPO_SERVER_LISTEN_METRICS_ADDRESS, ARGOCD_LOG_FORMAT_TIMESTAMP parameters using wrong CM
- Fix failure to find matching source in history in multisource apps

### Documentation

- Fix usage/configuration doc. generator to supports boolean params

### Other

- Bump golang dependencies
- Bump workflow actions versions
- Bump base docker image versions
- Update CI tools installers versions

## v0.0.5 (2026-01-04)

### Features

- Implement Argo CD Monorepo Repo Server Metrics

###  Bug Fixes

- Properly report and handle git rev-list errors
- Allow enabling grpc metrics for repo server were not really enabled

### Documentation

- Add metrics setup documentation
- ServiceMonitor manifests examples
- Add sample grafana dashboard
- Add quick installation instruction in ondex.md (@dudinea)


## v0.0.4 (2025-12-23)

### Bug Fixes

- do not clear git-revision field when it cannot be calculated
- fix incorrect handling of relative manifest paths for multisource apps
- git-revisions were not populated on ms apps without history
- log fields in the repo-server method
- update deps in go.sum and go.mod
- update mkdocs.yml for last changes


### Docs

- How to configure AppSet controller to ignore mrp annotations
- Add notification examples
- Move applicationset controller stuff to separete file


## v0.0.3 (2025-07-09)

- ci: publishing images to `quay.io/argoprojlabs/argocd-monorepo-controller`
- ci: add release workflow
- docs: add a sample notification trigger/template configuration

## v0.0.2 (2025-07-07)

### Bug fixes

- fix: missing network policy to allow redis connection
- fix: not filling change revision for new apps without history
- fix: change revision was reset to git revision when there were no
  files changed in the commits

### Tests

- test: add unit tests for monorepo controller service
- ci: restore running unit tests on CI

### Other Changes

- fix: fixes for excessive logging, some logging cleanup
- feat: Add option for disabling use of redis cache
- chore: Fix lots of issues found by linters
- ci: run lints on CI

## v0.0.1 (2025-07-01)

### First published version

- feat: Derived from the argo-cd master, revision ea31d17f5 
- feat: Appears to work, not optimized, no tests, no CI

