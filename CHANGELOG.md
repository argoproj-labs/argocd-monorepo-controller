# Changelog

## v0.0.4 (2025-12-23)

- docs: relatively complete documentation on architecture, installation and configuration
- docs: better examples for notification configuration
- installation: Helm chart
- installation: fixes for installation manifests
- functionality: many bugfixes for core functionality, better logging and robsutness
- chore: version updates for dependencies
- tests: added codecov


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

