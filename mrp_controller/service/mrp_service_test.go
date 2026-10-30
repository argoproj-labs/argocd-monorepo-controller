package service

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/mrp_controller/metrics"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/mocks"
	appmocks "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/typed/application/v1alpha1/mocks"
	applistermocks "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1/mocks"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	repomocks "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"

	"github.com/argoproj/argo-cd/v3/util/app/path"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/require"
)

/*const fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    ksonnet:
      environment: default
  destination:
    namespace: guestbook
    server: https://cluster-api.example.com
`
*/
// const fakeAppWithOperation = `
// apiVersion: argoproj.io/v1alpha1
// kind: Application
// metadata:
//   annotations:
//     argocd.argoproj.io/manifest-generate-paths: .
//   finalizers:
//   - resources-finalizer.argocd.argoproj.io
//   labels:
//     app.kubernetes.io/instance: guestbook
//   name: guestbook
//   namespace: codefresh
// operation:
//   initiatedBy:
//     automated: true
//   retry:
//     limit: 5
//   sync:
//     prune: true
//     revision: c732f4d2ef24c7eeb900e9211ff98f90bb646505
//     syncOptions:
//     - CreateNamespace=true
// spec:
//   destination:
//     namespace: guestbook
//     server: https://kubernetes.default.svc
//   project: default
//   source:
//     path: apps/guestbook
//     repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
//     targetRevision: HEAD
// `

const syncedAppWithoutHistory = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/trioapp-dev
  name: demo-trioapp-dev
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-dev
  project: default
  source:
    path: demo-applications/trioapp-dev
    repoURL: https://github.com/somehere/repo01.git
    targetRevision: dev
  syncPolicy:
    automated:
      allowEmpty: false
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-07-05T16:24:49Z"
    status: Healthy
  reconciledAt: "2025-07-06T15:57:23Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm
    namespace: demo-dev
    status: Synced
    version: v1
  sourceType: Helm
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-dev
      source:
        path: demo-applications/trioapp-dev
        repoURL: https://github.com/somehere/repo01.git
        targetRevision: dev
    revision: 2b571ad9ceaab7ed1e6225ca674e367f2d07e41d
    status: Synced
`

const syncedMSAppWithoutHistory = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/trioapp-dev
  name: demo-trioapp-dev
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-dev
  project: default
  sources:
  - path: demo-applications/try-ms02a
    repoURL: https://github.com/somehere/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms02b
    repoURL: https://github.com/somehere/cfrepo02.git
    targetRevision: dev
  syncPolicy:
    automated:
      allowEmpty: false
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-07-05T16:24:49Z"
    status: Healthy
  reconciledAt: "2025-07-06T15:57:23Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm
    namespace: demo-dev
    status: Synced
    version: v1
  sourceType: Helm
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-dev
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/somehere/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/somehere/cfrepo02.git
      targetRevision: dev
    revisions:
    - 2b571ad9ceaab7ed1e6225ca674e367f2d07e41d
    - e4d97f011da70c10a6ec176c37fe267bbb271975
    status: Synced
`

const runningAppWithSingleHistory1Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
    mrp-controller.argoproj.io/change-revision: 792822850fd2f6db63597533e16dfa27e6757dc5
    mrp-controller.argoproj.io/git-revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    app.kubernetes.io/instance: guestbook
  name: guestbook
  namespace: argocd
operation:
  initiatedBy:
    automated: true
  retry:
    limit: 5
  sync:
    prune: true
    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
    syncOptions:
    - CreateNamespace=true
spec:
  destination:
    namespace: guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: apps/guestbook
    repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
    targetRevision: HEAD
status:
  history:
  - deployStartedAt: "2024-06-20T19:35:36Z"
    deployedAt: "2024-06-20T19:35:44Z"
    id: 3
    initiatedBy: {}
    revision: 792822850fd2f6db63597533e16dfa27e6757dc5
    source:
      path: apps/guestbook
      repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
      targetRevision: HEAD
  operationState:
    operation:
      sync:
        prune: true
        revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
        syncOptions:
        - CreateNamespace=true
    phase: Running
    startedAt: "2024-06-20T19:47:34Z"
    syncResult:
      revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
      source:
        path: apps/guestbook
        repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
        targetRevision: HEAD
  sync:
    revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
    status: Running
`

const syncedAppWithSingleHistory1Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
    mrp-controller.argoproj.io/change-revision: 792822850fd2f6db63597533e16dfa27e6757dc5
    mrp-controller.argoproj.io/change-revisions: "[\"792822850fd2f6db63597533e16dfa27e6757dc5\"]"
    mrp-controller.argoproj.io/git-revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
    mrp-controller.argoproj.io/git-revisions: "[\"00d423763fbf56d2ea452de7b26a0ab20590f521\"]"
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    app.kubernetes.io/instance: guestbook
  name: guestbook
  namespace: argocd
operation:
  initiatedBy:
    automated: true
  retry:
    limit: 5
  sync:
    prune: true
    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646505
    syncOptions:
    - CreateNamespace=true
spec:
  destination:
    namespace: guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: apps/guestbook
    repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
    targetRevision: HEAD
status:
  history:
  - deployStartedAt: "2024-06-20T19:35:36Z"
    deployedAt: "2024-06-20T19:35:44Z"
    id: 3
    initiatedBy: {}
    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
    source:
      path: apps/guestbook
      repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
      targetRevision: HEAD
  operationState:
    operation:
      sync:
        prune: true
        revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
        syncOptions:
        - CreateNamespace=true
    phase: Synced
    startedAt: "2024-06-20T19:47:34Z"
    syncResult:
      revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
      source:
        path: apps/guestbook
        repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
        targetRevision: HEAD
  sync:
    revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
    status: Synced
`

const syncedAppWithSingleHistory2Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
    mrp-controller.argoproj.io/change-revision: 792822850fd2f6db63597533e16dfa27e6757dc5
    mrp-controller.argoproj.io/git-revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
  finalizers:
  - resources-finalizer.argocd.argoproj.io
  labels:
    app.kubernetes.io/instance: guestbook
  name: guestbook
  namespace: argocd
operation:
  initiatedBy:
    automated: true
  retry:
    limit: 5
  sync:
    prune: true
    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646505
    syncOptions:
    - CreateNamespace=true
spec:
  destination:
    namespace: guestbook
    server: https://kubernetes.default.svc
  project: default
  source:
    path: apps/guestbook
    repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
    targetRevision: HEAD
status:
  history:
  - deployStartedAt: "2024-06-20T18:30:00Z"
    deployedAt: "2024-06-20T18:30:01Z"
    id: 2
    initiatedBy: {}
    revision: 1af87672323345954554587665757e0999678678
    source:
      path: apps/guestbook
      repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
      targetRevision: HEAD
  - deployStartedAt: "2024-06-20T19:35:36Z"
    deployedAt: "2024-06-20T19:35:44Z"
    id: 3
    initiatedBy: {}
    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
    source:
      path: apps/guestbook
      repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
      targetRevision: HEAD
  operationState:
    operation:
      sync:
        prune: true
        revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
        syncOptions:
        - CreateNamespace=true
    phase: Synced
    startedAt: "2024-06-20T19:47:34Z"
    syncResult:
      revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
      source:
        path: apps/guestbook
        repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
        targetRevision: HEAD
  sync:
    revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
    status: Synced
`

const multiSourceAppAnnotations = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    valid: '["a","b", null]'
    empty-array: '[]'
    empty: ''
    invalid-json: '["fooo"'
    invalid-entries: '["fooo", "2"]'
    invalid-map: '{}'
    invalid-string: '"foo"'
  name: demo-ms-a
  namespace: argocd
`

const syncedMSAppWithSingleHistory1Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/try-ms02a;/demo-applications/try-ms02b
    mrp-controller.argoproj.io/change-revisions: '["HISTORY-2_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO01_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
    mrp-controller.argoproj.io/git-revisions:    '["HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
  name: demo-ms-a
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-ms-a
  project: default
  sources:
  - path: demo-applications/try-ms02a
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms02b
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms01a
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  - path: demo-applications/try-ms01b
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-08-10T12:01:35Z"
    status: Healthy
  history:
  - deployStartedAt: "2025-08-10T11:39:57Z"
    deployedAt: "2025-08-10T11:39:57Z"
    id: 3
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T11:52:03Z"
    deployedAt: "2025-08-10T11:52:04Z"
    id: 4
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T12:01:33Z"
    deployedAt: "2025-08-10T12:01:33Z"
    id: 5
    initiatedBy:
      automated: true
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  operationState:
    finishedAt: "2025-08-10T12:01:33Z"
    message: successfully synced (all tasks run)
    operation:
      initiatedBy:
        automated: true
      retry:
        limit: 5
      sync:
        prune: true
        revisions:
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        syncOptions:
        - PrunePropagationPolicy=foreground
        - Replace=false
        - PruneLast=false
        - Validate=true
        - CreateNamespace=true
        - ApplyOutOfSyncOnly=false
        - ServerSideApply=true
        - RespectIgnoreDifferences=false
    phase: Succeeded
    startedAt: "2025-08-10T12:01:33Z"
    syncResult:
      resources:
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02a serverside-applied
        name: config-cm-ms02a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02b serverside-applied
        name: config-cm-ms02b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01b serverside-applied
        name: config-cm-ms01b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01a serverside-applied
        name: config-cm-ms01a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      revision: ""
      revisions:
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
  reconciledAt: "2025-08-11T14:05:27Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm-ms01a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms01b
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02b
    namespace: demo-ms-a
    status: Synced
    version: v1
  sourceHydrator: {}
  sourceTypes:
  - Directory
  - Directory
  - Directory
  - Directory
  summary: {}
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-ms-a
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
    revisions:
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    status: Synced
`

const syncedMSAppWithSingleHistory1AnnotatedSourceChanged = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/try-ms02a;/demo-applications/try-ms02b
    mrp-controller.argoproj.io/change-revisions: '["HISTORY-2_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO01_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
    mrp-controller.argoproj.io/git-revisions:    '["HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
  name: demo-ms-a
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-ms-a
  project: default
  sources:
  - path: demo-applications/try-ms02a
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
    helm:
      valuesObject:
        foo:
          bar: app2
  - path: demo-applications/try-ms02b
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms01a
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  - path: demo-applications/try-ms01b
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-08-10T12:01:35Z"
    status: Healthy
  history:
  - deployStartedAt: "2025-08-10T11:39:57Z"
    deployedAt: "2025-08-10T11:39:57Z"
    id: 3
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
      helm:
        valuesObject:
          foo:
            bar: app
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T11:52:03Z"
    deployedAt: "2025-08-10T11:52:04Z"
    id: 4
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
      helm:
        valuesObject:
          foo:
            bar: app
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T12:01:33Z"
    deployedAt: "2025-08-10T12:01:33Z"
    id: 5
    initiatedBy:
      automated: true
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
      helm:
        valuesObject:
          foo:
            bar: app
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  operationState:
    finishedAt: "2025-08-10T12:01:33Z"
    message: successfully synced (all tasks run)
    operation:
      initiatedBy:
        automated: true
      retry:
        limit: 5
      sync:
        prune: true
        revisions:
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        syncOptions:
        - PrunePropagationPolicy=foreground
        - Replace=false
        - PruneLast=false
        - Validate=true
        - CreateNamespace=true
        - ApplyOutOfSyncOnly=false
        - ServerSideApply=true
        - RespectIgnoreDifferences=false
    phase: Succeeded
    startedAt: "2025-08-10T12:01:33Z"
    syncResult:
      resources:
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02a serverside-applied
        name: config-cm-ms02a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02b serverside-applied
        name: config-cm-ms02b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01b serverside-applied
        name: config-cm-ms01b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01a serverside-applied
        name: config-cm-ms01a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      revision: ""
      revisions:
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
        helm:
          valuesObject:
            foo:
              bar: app
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
  reconciledAt: "2025-08-11T14:05:27Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm-ms01a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms01b
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02b
    namespace: demo-ms-a
    status: Synced
    version: v1
  sourceHydrator: {}
  sourceTypes:
  - Directory
  - Directory
  - Directory
  - Directory
  summary: {}
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-ms-a
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
    revisions:
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    status: Synced
`

const syncedMSAppWithSingleHistory2Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/try-ms02a;/demo-applications/try-ms02b
    mrp-controller.argoproj.io/change-revisions: '["HISTORY-2_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO01_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
    mrp-controller.argoproj.io/git-revisions:    '["HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
  name: demo-ms-a
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-ms-a
  project: default
  sources:
  - path: demo-applications/try-ms02a
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms02b
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms01a
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  - path: demo-applications/try-ms01b
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-08-10T12:01:35Z"
    status: Healthy
  history:
  - deployStartedAt: "2025-08-10T11:39:57Z"
    deployedAt: "2025-08-10T11:39:57Z"
    id: 3
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-2_REPO02_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    - HISTORY-1_REPO01_00000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T11:52:03Z"
    deployedAt: "2025-08-10T11:52:04Z"
    id: 4
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T12:01:33Z"
    deployedAt: "2025-08-10T12:01:33Z"
    id: 5
    initiatedBy:
      automated: true
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  operationState:
    finishedAt: "2025-08-10T12:01:33Z"
    message: successfully synced (all tasks run)
    operation:
      initiatedBy:
        automated: true
      retry:
        limit: 5
      sync:
        prune: true
        revisions:
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        syncOptions:
        - PrunePropagationPolicy=foreground
        - Replace=false
        - PruneLast=false
        - Validate=true
        - CreateNamespace=true
        - ApplyOutOfSyncOnly=false
        - ServerSideApply=true
        - RespectIgnoreDifferences=false
    phase: Succeeded
    startedAt: "2025-08-10T12:01:33Z"
    syncResult:
      resources:
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02a serverside-applied
        name: config-cm-ms02a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02b serverside-applied
        name: config-cm-ms02b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01b serverside-applied
        name: config-cm-ms01b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01a serverside-applied
        name: config-cm-ms01a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      revision: ""
      revisions:
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
  reconciledAt: "2025-08-11T14:05:27Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm-ms01a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms01b
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02b
    namespace: demo-ms-a
    status: Synced
    version: v1
  sourceHydrator: {}
  sourceTypes:
  - Directory
  - Directory
  - Directory
  - Directory
  summary: {}
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-ms-a
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
    revisions:
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    status: Synced
`

const syncedMSAppWithSingleHistory3Annotated = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd.argoproj.io/manifest-generate-paths: /demo-applications/try-ms02a;/demo-applications/try-ms02b
    mrp-controller.argoproj.io/change-revisions: '["HISTORY-2_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO01_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
    mrp-controller.argoproj.io/git-revisions:    '["HISTORY-1_REPO02_00000000000000000000000","HISTORY-1_REPO02_00000000000000000000000","CURRENT_REPO_01_000000000000000000000000","CURRENT_REPO_01_000000000000000000000000"]'
  name: demo-ms-a
  namespace: argocd
spec:
  destination:
    name: in-cluster
    namespace: demo-ms-a
  project: default
  sources:
  - path: demo-applications/try-ms02a
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms02b
    repoURL: https://github.com/dudinea/cfrepo02.git
    targetRevision: dev
  - path: demo-applications/try-ms01a
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  - path: demo-applications/try-ms01b
    repoURL: https://github.com/dudinea/cfrepo01.git
    targetRevision: main
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - PrunePropagationPolicy=foreground
    - Replace=false
    - PruneLast=false
    - Validate=true
    - CreateNamespace=true
    - ApplyOutOfSyncOnly=false
    - ServerSideApply=true
    - RespectIgnoreDifferences=false
status:
  controllerNamespace: argocd
  health:
    lastTransitionTime: "2025-08-10T12:01:35Z"
    status: Healthy
  history:
  - deployStartedAt: "2025-08-10T11:39:57Z"
    deployedAt: "2025-08-10T11:39:57Z"
    id: 3
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-2_REPO02_00000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T11:52:03Z"
    deployedAt: "2025-08-10T11:52:04Z"
    id: 4
    initiatedBy:
      username: admin
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  - deployStartedAt: "2025-08-10T12:01:33Z"
    deployedAt: "2025-08-10T12:01:33Z"
    id: 5
    initiatedBy:
      automated: true
    revisions:
    - HISTORY-1_REPO02_00000000000000000000000
    - HISTORY-1_REPO02_00000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    source:
      repoURL: ""
    sources:
    - path: demo-applications/try-ms02a
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms02b
      repoURL: https://github.com/dudinea/cfrepo02.git
      targetRevision: dev
    - path: demo-applications/try-ms01a
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
    - path: demo-applications/try-ms01b
      repoURL: https://github.com/dudinea/cfrepo01.git
      targetRevision: main
  operationState:
    finishedAt: "2025-08-10T12:01:33Z"
    message: successfully synced (all tasks run)
    operation:
      initiatedBy:
        automated: true
      retry:
        limit: 5
      sync:
        prune: true
        revisions:
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_02_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        - CURRENT_REPO_01_000000000000000000000000
        syncOptions:
        - PrunePropagationPolicy=foreground
        - Replace=false
        - PruneLast=false
        - Validate=true
        - CreateNamespace=true
        - ApplyOutOfSyncOnly=false
        - ServerSideApply=true
        - RespectIgnoreDifferences=false
    phase: Succeeded
    startedAt: "2025-08-10T12:01:33Z"
    syncResult:
      resources:
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02a serverside-applied
        name: config-cm-ms02a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms02b serverside-applied
        name: config-cm-ms02b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01b serverside-applied
        name: config-cm-ms01b
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      - group: ""
        hookPhase: Running
        kind: ConfigMap
        message: configmap/config-cm-ms01a serverside-applied
        name: config-cm-ms01a
        namespace: demo-ms-a
        status: Synced
        syncPhase: Sync
        version: v1
      revision: ""
      revisions:
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_02_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      - CURRENT_REPO_01_000000000000000000000000
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
  reconciledAt: "2025-08-11T14:05:27Z"
  resourceHealthSource: appTree
  resources:
  - kind: ConfigMap
    name: config-cm-ms01a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms01b
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02a
    namespace: demo-ms-a
    status: Synced
    version: v1
  - kind: ConfigMap
    name: config-cm-ms02b
    namespace: demo-ms-a
    status: Synced
    version: v1
  sourceHydrator: {}
  sourceTypes:
  - Directory
  - Directory
  - Directory
  - Directory
  summary: {}
  sync:
    comparedTo:
      destination:
        name: in-cluster
        namespace: demo-ms-a
      source:
        repoURL: ""
      sources:
      - path: demo-applications/try-ms02a
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms02b
        repoURL: https://github.com/dudinea/cfrepo02.git
        targetRevision: dev
      - path: demo-applications/try-ms01a
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
      - path: demo-applications/try-ms01b
        repoURL: https://github.com/dudinea/cfrepo01.git
        targetRevision: main
    revisions:
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_02_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    - CURRENT_REPO_01_000000000000000000000000
    status: Synced
`

// const syncedAppWithHistory = `
// apiVersion: argoproj.io/v1alpha1
// kind: Application
// metadata:
//
//	annotations:
//	  argocd.argoproj.io/manifest-generate-paths: .
//	finalizers:
//	- resources-finalizer.argocd.argoproj.io
//	labels:
//	  app.kubernetes.io/instance: guestbook
//	name: guestbook
//	namespace: argocd
//
// operation:
//
//	initiatedBy:
//	  automated: true
//	retry:
//	  limit: 5
//	sync:
//	  prune: true
//	  revision: c732f4d2ef24c7eeb900e9211ff98f90bb646505
//	  syncOptions:
//	  - CreateNamespace=true
//
// spec:
//
//	destination:
//	  namespace: guestbook
//	  server: https://kubernetes.default.svc
//	project: default
//	source:
//	  path: apps/guestbook
//	  repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
//	  targetRevision: HEAD
//
// status:
//
//	history:
//	- deployStartedAt: "2024-06-20T19:35:36Z"
//	  deployedAt: "2024-06-20T19:35:44Z"
//	  id: 3
//	  initiatedBy: {}
//	  revision: 792822850fd2f6db63597533e16dfa27e6757dc5
//	  source:
//	    path: apps/guestbook
//	    repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
//	    targetRevision: HEAD
//	- deployStartedAt: "2024-06-20T19:36:34Z"
//	  deployedAt: "2024-06-20T19:36:42Z"
//	  id: 4
//	  initiatedBy: {}
//	  revision: ee5373eb9814e247ec6944e8b8897a8ec2f8528e
//	  source:
//	    path: apps/guestbook
//	    repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
//	    targetRevision: HEAD
//	operationState:
//	  operation:
//	    sync:
//	      prune: true
//	      revision: c732f4d2ef24c7eeb900e9211ff98f90bb646506
//	      syncOptions:
//	      - CreateNamespace=true
//	  phase: Running
//	  startedAt: "2024-06-20T19:47:34Z"
//	  syncResult:
//	    revision: c732f4d2ef24c7eeb900e9211ff98f90bb646505
//	    source:
//	      path: apps/guestbook
//	      repoURL: https://github.com/pasha-codefresh/precisely-gitsource.git
//	      targetRevision: HEAD
//	sync:
//	  revision: 00d423763fbf56d2ea452de7b26a0ab20590f521
//	  status: Synced
//
// `
func Test_getArrayFromAnnotation(t *testing.T) {
	anapp := createTestApp(t, multiSourceAppAnnotations)
	logCtx := createLogCtx(anapp)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	arr := mrpService.getArrayFromAnnotation(anapp, logCtx, "valid")
	assert.NotNil(t, arr)
	assert.Len(t, arr, 3)
	assert.Equal(t, "a", arr[0])
	assert.Equal(t, "b", arr[1])
	assert.Empty(t, arr[2])
	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "empty-array")
	assert.NotNil(t, arr)
	assert.Empty(t, arr)

	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "empty")
	assert.Nil(t, arr)
	assert.Empty(t, arr)

	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "invalid-json")
	assert.Nil(t, arr)
	assert.Empty(t, arr)

	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "invalid-map")
	assert.Nil(t, arr)
	assert.Empty(t, arr)

	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "invalid-string")
	assert.Nil(t, arr)
	assert.Empty(t, arr)

	arr = mrpService.getArrayFromAnnotation(anapp, logCtx, "unknown-annotation")
	assert.Nil(t, arr)
	assert.Empty(t, arr)
}

func Test_GetSourceRevisionsSSWithoutHistory(t *testing.T) {
	anapp := createTestApp(t, syncedAppWithoutHistory)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	// changeRevision, gitRevision, currentRevision, previousRevision := getApplicationRevisions(anapp, -1)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 1)
	assert.Equal(t, "2b571ad9ceaab7ed1e6225ca674e367f2d07e41d", sourcesRevisions[0].currentRevision)
	assert.Empty(t, sourcesRevisions[0].previousRevision)
	assert.Empty(t, sourcesRevisions[0].gitRevision)
	assert.Empty(t, sourcesRevisions[0].changeRevision)
}

func Test_GetSourceRevisionsMSWithoutHistory(t *testing.T) {
	anapp := createTestApp(t, syncedMSAppWithoutHistory)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	// changeRevision, gitRevision, currentRevision, previousRevision := getApplicationRevisions(anapp, -1)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 2)
	assert.Equal(t, "2b571ad9ceaab7ed1e6225ca674e367f2d07e41d", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "e4d97f011da70c10a6ec176c37fe267bbb271975", sourcesRevisions[1].currentRevision)
	assert.Empty(t, sourcesRevisions[0].previousRevision)
	assert.Empty(t, sourcesRevisions[0].gitRevision)
	assert.Empty(t, sourcesRevisions[0].changeRevision)
}

func Test_GetSourceRevisionsSSWithHistory1Running(t *testing.T) {
	anapp := createTestApp(t, runningAppWithSingleHistory1Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 1)
	assert.Equal(t, "00d423763fbf56d2ea452de7b26a0ab20590f521", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "792822850fd2f6db63597533e16dfa27e6757dc5", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646506", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "792822850fd2f6db63597533e16dfa27e6757dc5", sourcesRevisions[0].previousRevision)
}

func Test_GetSourceRevisionsSSWithHistory1Synced(t *testing.T) {
	anapp := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 1)
	assert.Equal(t, "00d423763fbf56d2ea452de7b26a0ab20590f521", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "792822850fd2f6db63597533e16dfa27e6757dc5", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646506", sourcesRevisions[0].currentRevision)
	assert.Empty(t, sourcesRevisions[0].previousRevision)
}

func Test_GetSourceRevisionsSSWithHistory2Synced(t *testing.T) {
	anapp := createTestApp(t, syncedAppWithSingleHistory2Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 1)
	assert.Equal(t, "00d423763fbf56d2ea452de7b26a0ab20590f521", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "792822850fd2f6db63597533e16dfa27e6757dc5", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646506", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "1af87672323345954554587665757e0999678678", sourcesRevisions[0].previousRevision)
}

func Test_GetSourceRevisionsMSWithHistory(t *testing.T) {
	anapp := createTestApp(t, syncedMSAppWithSingleHistory1Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 4)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "HISTORY-2_REPO02_00000000000000000000000", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].previousRevision)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[1].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO01_00000000000000000000000", sourcesRevisions[2].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].gitRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].previousRevision)
}

func Test_GetSourceRevisionsMSWithHistorySwapped(t *testing.T) {
	anapp := createTestApp(t, syncedMSAppWithSingleHistory2Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 4)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "HISTORY-2_REPO02_00000000000000000000000", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].previousRevision)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[1].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO01_00000000000000000000000", sourcesRevisions[2].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].gitRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].previousRevision)
}

func Test_GetSourceRevisionsMSWithHistoryAdded(t *testing.T) {
	anapp := createTestApp(t, syncedMSAppWithSingleHistory3Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 4)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "HISTORY-2_REPO02_00000000000000000000000", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].previousRevision)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[1].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO01_00000000000000000000000", sourcesRevisions[2].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].gitRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].previousRevision)
}

func Test_GetSourceRevisionsMSWithHistorySourceChanged(t *testing.T) {
	anapp := createTestApp(t, syncedMSAppWithSingleHistory1AnnotatedSourceChanged)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourcesRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.NotNil(t, sourcesRevisions)
	assert.Len(t, sourcesRevisions, 4)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].gitRevision)
	assert.Equal(t, "HISTORY-2_REPO02_00000000000000000000000", sourcesRevisions[0].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[0].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[0].previousRevision)

	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].changeRevision)
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", sourcesRevisions[1].currentRevision)
	assert.Equal(t, "HISTORY-1_REPO02_00000000000000000000000", sourcesRevisions[1].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].gitRevision)
	assert.Equal(t, "HISTORY-1_REPO01_00000000000000000000000", sourcesRevisions[2].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[2].previousRevision)

	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].gitRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].changeRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].currentRevision)
	assert.Equal(t, "CURRENT_REPO_01_000000000000000000000000", sourcesRevisions[3].previousRevision)
}

func Test_GetApplicationRevisionsWithoutHistory(t *testing.T) {
	anapp := createTestApp(t, syncedAppWithoutHistory)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	logCtx := createLogCtx(anapp)
	sourceRevisions := mrpService.getSourcesRevisions(anapp, logCtx)
	assert.Len(t, sourceRevisions, 1)
	assert.Equal(t, "2b571ad9ceaab7ed1e6225ca674e367f2d07e41d", sourceRevisions[0].currentRevision)
	assert.Empty(t, sourceRevisions[0].previousRevision)
	assert.Empty(t, sourceRevisions[0].changeRevision)
	assert.Empty(t, sourceRevisions[0].gitRevision)
}

// func Test_CalculateRevision_no_paths(t *testing.T) {
// 	mrpService := newTestMRPService(t,
// 		&repomocks.Clientset{},
// 		&mocks.Interface{},
// 		&dbmocks.ArgoDB{})
// 	app := createTestApp(t, fakeApp)
// 	revision, err := mrpService.calculateChangeRevision(t.Context(), app, "", "")
// 	assert.Nil(t, revision)
// 	require.Error(t, err)
// 	assert.Equal(t, "rpc error: code = FailedPrecondition desc = manifest generation paths not set", err.Error())
// }

func Test_CalculateRevision(t *testing.T) {
	expectedRevision := "ffffffffffffffffffffffffffffffffffffffff"
	repo := appsv1.Repository{Repo: "myrepo"}
	app := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	db := createTestArgoDbForAppAndRepo(t, app, &repo)
	sourcePath := app.Spec.Source.Path
	changeRevisionRequest := repoapiclient.ChangeRevisionRequest{
		AppName:          app.GetName(),
		Namespace:        app.GetNamespace(),
		CurrentRevision:  "c732f4d2ef24c7eeb900e9211ff98f90bb646506",
		PreviousRevision: "",
		Paths:            GetSourceRefreshPaths(app, sourcePath),
		Repo:             &repo,
	}
	changeRevisionResponce := repoapiclient.ChangeRevisionResponse{}
	changeRevisionResponce.Revision = expectedRevision
	clientsetmock := createTestRepoclientForApp(t, &changeRevisionRequest, &changeRevisionResponce)
	mrpService := newTestMRPService(t, clientsetmock, &mocks.Interface{}, db)
	currentRevision, previousRevision := getRevisionsSingleSource(app)
	logCtx := logrus.WithFields(logrus.Fields{"application": app.Name, "appNamespace": app.Namespace})
	revision, err := mrpService.calculateChangeRevision(t.Context(), logCtx, app, currentRevision, previousRevision, app.Spec.Source.RepoURL, sourcePath)
	require.NoError(t, err)
	assert.NotNil(t, revision)
	assert.Equal(t, expectedRevision, *revision)
}

func Test_addPatchIfNeeded(t *testing.T) {
	key := "foo"
	val := "bar"
	val2 := "baz"
	key2 := "goo"

	annotations := map[string]string{}
	currentAnnotations := map[string]string{key: val}

	addPatchIfNeeded(annotations, currentAnnotations, key, val)
	assert.Empty(t, annotations)

	addPatchIfNeeded(annotations, currentAnnotations, key, val2)
	assert.Len(t, annotations, 1)
	patchVal, ok := annotations[key]
	assert.True(t, ok)
	assert.Equal(t, val2, patchVal)

	addPatchIfNeeded(annotations, currentAnnotations, key2, val)
	assert.Len(t, annotations, 2)
	patchVal, ok = annotations[key2]
	assert.True(t, ok)
	assert.Equal(t, val, patchVal)
}

func getPatchAnnotations(t *testing.T, patch map[string]any) map[string]string {
	t.Helper()
	assert.NotNil(t, patch)
	meta := patch["metadata"].(map[string]any)
	assert.NotNil(t, meta)
	annotations := meta["annotations"].(map[string]string)
	return annotations
}

func Test_makeAnnotationPatchNoChange(t *testing.T) {
	a := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)

	var changeRevisions []string
	err := json.Unmarshal([]byte(a.Annotations[CHANGE_REVISIONS_ANN]), &changeRevisions)
	require.NoError(t, err)

	var gitRevisions []string
	err = json.Unmarshal([]byte(a.Annotations[GIT_REVISIONS_ANN]), &gitRevisions)
	require.NoError(t, err)

	patch, err := mrpService.makeAnnotationPatch(a, a.Annotations[CHANGE_REVISION_ANN], changeRevisions,
		a.Annotations[GIT_REVISION_ANN], gitRevisions)
	require.NoError(t, err)
	assert.Nil(t, patch)
}

func Test_makeAnnotationPatch(t *testing.T) {
	changeRevision := "12345"
	changeRevisions := []string{changeRevision}
	gitRevision := "56789"
	gitRevisions := []string{gitRevision}

	a := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	mrpService := newTestMRPService(t, nil, &mocks.Interface{}, nil)
	patch, err := mrpService.makeAnnotationPatch(a, changeRevision, changeRevisions, gitRevision, gitRevisions)
	require.NoError(t, err)
	annotations := getPatchAnnotations(t, patch)
	assert.Len(t, annotations, 4)
	assert.Equal(t, changeRevision, annotations[CHANGE_REVISION_ANN])
	assert.Equal(t, gitRevision, annotations[GIT_REVISION_ANN])

	changeRevisionsBytes, err := json.Marshal(changeRevisions)
	require.NoError(t, err)
	assert.Equal(t, string(changeRevisionsBytes), annotations[CHANGE_REVISIONS_ANN])

	gitRevisionsBytes, err := json.Marshal(gitRevisions)
	require.NoError(t, err)
	assert.Equal(t, string(gitRevisionsBytes), annotations[GIT_REVISIONS_ANN])
}

func Test_ChangeRevision(t *testing.T) {
	expectedRevision := "ffffffffffffffffffffffffffffffffffffffff"
	repo := appsv1.Repository{Repo: "myrepo"}
	app := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	appClientMock := createTestAppClientForApp(t, app)
	db := createTestArgoDbForAppAndRepo(t, app, &repo)
	changeRevisionRequest := repoapiclient.ChangeRevisionRequest{
		AppName:          app.GetName(),
		Namespace:        app.GetNamespace(),
		CurrentRevision:  "c732f4d2ef24c7eeb900e9211ff98f90bb646506",
		PreviousRevision: "",
		Paths:            path.GetAppRefreshPaths(app),
		Repo:             &repo,
	}
	changeRevisionResponce := repoapiclient.ChangeRevisionResponse{}
	changeRevisionResponce.Revision = expectedRevision
	clientsetmock := createTestRepoclientForApp(t, &changeRevisionRequest, &changeRevisionResponce)
	mrpService := newTestMRPService(t, clientsetmock, appClientMock, db)
	err := mrpService.ChangeRevision(t.Context(), app)
	assert.NoError(t, err)
}

func Test_makeChangeRevisionPatch(t *testing.T) {
	expectedRevision := "ffffffffffffffffffffffffffffffffffffffff"
	repo := appsv1.Repository{Repo: "myrepo"}
	app := createTestApp(t, syncedAppWithSingleHistory1Annotated)
	appClientMock := createTestAppClientForApp(t, app)
	db := createTestArgoDbForAppAndRepo(t, app, &repo)
	changeRevisionRequest := repoapiclient.ChangeRevisionRequest{
		AppName:          app.GetName(),
		Namespace:        app.GetNamespace(),
		CurrentRevision:  "c732f4d2ef24c7eeb900e9211ff98f90bb646506",
		PreviousRevision: "",
		Paths:            path.GetAppRefreshPaths(app),
		Repo:             &repo,
	}
	changeRevisionResponce := repoapiclient.ChangeRevisionResponse{}
	changeRevisionResponce.Revision = expectedRevision
	clientsetmock := createTestRepoclientForApp(t, &changeRevisionRequest, &changeRevisionResponce)
	mrpService := newTestMRPService(t, clientsetmock, appClientMock, db)
	logCtx := logrus.WithFields(logrus.Fields{"application": app.Name, "appNamespace": app.Namespace})
	patch, err := mrpService.makeChangeRevisionPatch(t.Context(), logCtx, app)
	require.NoError(t, err)
	assert.NotNil(t, patch)
	annotations := getPatchAnnotations(t, patch)
	assert.Equal(t, expectedRevision, annotations[CHANGE_REVISION_ANN])
	assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646506", annotations[GIT_REVISION_ANN])
}

func Test_makeChangeRevisionPatchPartialFailure(t *testing.T) {
	expectedRevision := "ffffffffffffffffffffffffffffffffffffffff"
	repo := appsv1.Repository{Repo: "cfrepo02"}
	app := createTestApp(t, syncedMSAppWithSingleHistory1Annotated)
	appClientMock, _ := createTestAppInterfaceForApp(t, app)
	clientsetmock := createTestRepoclientForMSApp(t, app, &repo,
		changeRevisionReply{response: &repoapiclient.ChangeRevisionResponse{Revision: expectedRevision}},
		changeRevisionReply{err: status.Error(codes.Unavailable, "connection refused")},
	)
	mrpService := newTestMRPService(t, clientsetmock, appClientMock, createTestArgoDbForMSApp(t, app, &repo))

	patch, err := mrpService.makeChangeRevisionPatch(t.Context(), createLogCtx(app), app)

	// the failure of the second source is reported, and is worth retrying
	require.Error(t, err)
	require.ErrorContains(t, err, "source 1")
	assert.True(t, IsRetryable(err))

	// while the revisions calculated for the first source are kept
	annotations := getPatchAnnotations(t, patch)
	assert.Equal(t, expectedRevision, annotations[CHANGE_REVISION_ANN])
	assert.Equal(t, "CURRENT_REPO_02_000000000000000000000000", annotations[GIT_REVISION_ANN])

	var changeRevisions []string
	require.NoError(t, json.Unmarshal([]byte(annotations[CHANGE_REVISIONS_ANN]), &changeRevisions))
	assert.Equal(t, []string{
		expectedRevision,
		"HISTORY-1_REPO02_00000000000000000000000",
		"HISTORY-1_REPO01_00000000000000000000000",
		"CURRENT_REPO_01_000000000000000000000000",
	}, changeRevisions)

	// the git revision of the failed source must stay at the revision its
	// change revision was calculated for, so that it is calculated again
	var gitRevisions []string
	require.NoError(t, json.Unmarshal([]byte(annotations[GIT_REVISIONS_ANN]), &gitRevisions))
	assert.Equal(t, []string{
		"CURRENT_REPO_02_000000000000000000000000",
		"HISTORY-1_REPO02_00000000000000000000000",
		"CURRENT_REPO_01_000000000000000000000000",
		"CURRENT_REPO_01_000000000000000000000000",
	}, gitRevisions)
}

func Test_ChangeRevisionPartialFailure(t *testing.T) {
	expectedRevision := "ffffffffffffffffffffffffffffffffffffffff"
	repo := appsv1.Repository{Repo: "cfrepo02"}
	app := createTestApp(t, syncedMSAppWithSingleHistory1Annotated)
	appClientMock, appIntMock := createTestAppInterfaceForApp(t, app)
	appIntMock.On("Patch", t.Context(), app.Name, types.MergePatchType,
		mock.MatchedBy(func(_ any) bool { return true }),
		metav1.PatchOptions{}).Return(app, nil).Once()
	clientsetmock := createTestRepoclientForMSApp(t, app, &repo,
		changeRevisionReply{response: &repoapiclient.ChangeRevisionResponse{Revision: expectedRevision}},
		changeRevisionReply{err: status.Error(codes.Unavailable, "connection refused")},
	)
	mrpService := newTestMRPService(t, clientsetmock, appClientMock, createTestArgoDbForMSApp(t, app, &repo))

	err := mrpService.ChangeRevision(t.Context(), app)

	// what could be calculated is patched, the failure is still reported
	require.Error(t, err)
	assert.True(t, IsRetryable(err))
	appIntMock.AssertExpectations(t)
}

func Test_makeChangeRevisionPatchAllSourcesFailed(t *testing.T) {
	repo := appsv1.Repository{Repo: "cfrepo02"}
	app := createTestApp(t, syncedMSAppWithSingleHistory1Annotated)
	appClientMock, _ := createTestAppInterfaceForApp(t, app)
	clientsetmock := createTestRepoclientForMSApp(t, app, &repo,
		changeRevisionReply{err: status.Error(codes.Unavailable, "connection refused")},
		changeRevisionReply{err: status.Error(codes.Unavailable, "connection refused")},
	)
	mrpService := newTestMRPService(t, clientsetmock, appClientMock, createTestArgoDbForMSApp(t, app, &repo))

	patch, err := mrpService.makeChangeRevisionPatch(t.Context(), createLogCtx(app), app)

	require.Error(t, err)
	require.ErrorContains(t, err, "source 0")
	require.ErrorContains(t, err, "source 1")
	assert.True(t, IsRetryable(err))

	// None of the per-source revisions changed, so they are not part of the
	// patch at all. Only the single source annotations, which this application
	// does not have yet, are added, holding the revisions of the first source
	// as they were before the failed attempt.
	annotations := getPatchAnnotations(t, patch)
	assert.Equal(t, map[string]string{
		CHANGE_REVISION_ANN: "HISTORY-2_REPO02_00000000000000000000000",
		GIT_REVISION_ANN:    "HISTORY-1_REPO02_00000000000000000000000",
	}, annotations)
}

func newTestMetricsServer(t *testing.T, dbMock *dbmocks.ArgoDB) *metrics.MetricsServer {
	t.Helper()
	healthcheck := func(_ *http.Request) error { return nil }
	var appConditions []string
	var appLabels []string
	canProcessApp := func(_ any) bool { return false }
	appLister := &applistermocks.ApplicationLister{}
	result, err := metrics.NewMetricsServer("0.0.0.0:8091",
		appLister,     /*AppLister*/
		canProcessApp, /*AppFilter*/
		healthcheck,   /*healthCheck */
		appLabels,     /* appLabels */
		appConditions, /* appCondition */
		dbMock,
	)
	assert.NoError(t, err)
	return result
}

func newTestMRPService(t *testing.T, repoClientMock *repomocks.Clientset,
	applicationClientsetMock *mocks.Interface,
	dbMock *dbmocks.ArgoDB,
) *mrpService {
	t.Helper()
	return &mrpService{
		applicationClientset: applicationClientsetMock,
		repoClientset:        repoClientMock,
		db:                   dbMock,
		logger:               logrus.New(),
		metricsServer:        newTestMetricsServer(t, dbMock),
	}
}

func createLogCtx(app *appsv1.Application) *logrus.Entry {
	return logrus.WithFields(logrus.Fields{"application": app.Name, "appNamespace": app.Namespace})
}

func createTestApp(t *testing.T, testApp string, opts ...func(app *appsv1.Application)) *appsv1.Application {
	t.Helper()
	var app appsv1.Application
	err := yaml.Unmarshal([]byte(testApp), &app)
	if err != nil {
		panic(err)
	}
	for i := range opts {
		opts[i](&app)
	}
	return &app
}

func createTestRepoclientForApp(t *testing.T,
	changeRevisionRequest *repoapiclient.ChangeRevisionRequest,
	changeRevisionResponse *repoapiclient.ChangeRevisionResponse,
) *repomocks.Clientset {
	t.Helper()
	clientsetmock := repomocks.Clientset{}
	clientmock := repomocks.RepoServerServiceClient{}
	clientmock.On("GetChangeRevision", t.Context(), changeRevisionRequest).
		Return(changeRevisionResponse, nil).Once()
	clientsetmock.RepoServerServiceClient = &clientmock
	return &clientsetmock
}

func createTestAppClientForApp(t *testing.T, app *appsv1.Application) *mocks.Interface {
	t.Helper()
	appintMock := &appmocks.ApplicationInterface{}
	appintMock.On("Get", t.Context(), app.Name, metav1.GetOptions{}).Return(app, nil)
	appintMock.On("Patch", t.Context(), app.Name, types.MergePatchType,
		// FIXME: test for the patch
		mock.MatchedBy(func(_ any) bool { return true }),
		metav1.PatchOptions{}).Return(app, nil)

	av1alpha1 := &appmocks.ArgoprojV1alpha1Interface{}
	av1alpha1.On("Applications", app.Namespace).Return(appintMock)

	mock := &mocks.Interface{}
	mock.On("ArgoprojV1alpha1").Return(av1alpha1)
	return mock
}

func createTestArgoDbForAppAndRepo(t *testing.T, app *appsv1.Application, repo *appsv1.Repository) *dbmocks.ArgoDB {
	t.Helper()
	db := dbmocks.ArgoDB{}
	db.On("GetRepository", t.Context(), app.Spec.Source.RepoURL, app.Spec.Project).
		Return(repo, nil).Once()
	return &db
}

// func Test_ChangeRevision(r *testing.T) {
// 	r.Run("history list is empty", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		current, previous := acrService.getRevisions(r.Context(), createTestApp(fakeApp))
// 		assert.Equal(t, "", current)
// 		assert.Equal(t, "", previous)
// 	})
// }

// func Test_getRevisions(r *testing.T) {
// 	r.Run("history list is empty", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		current, previous := acrService.getRevisions(r.Context(), createTestApp(fakeApp))
// 		assert.Equal(t, "", current)
// 		assert.Equal(t, "", previous)
// 	})

// 	r.Run("history list is empty, but operation happens right now", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		current, previous := acrService.getRevisions(r.Context(), createTestApp(fakeAppWithOperation))
// 		assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646505", current)
// 		assert.Equal(t, "", previous)
// 	})

// 	r.Run("history list contains only one element, also sync result is here", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		current, previous := acrService.getRevisions(r.Context(), createTestApp(syncedAppWithSingleHistory))
// 		assert.Equal(t, "c732f4d2ef24c7eeb900e9211ff98f90bb646505", current)
// 		assert.Equal(t, "", previous)
// 	})

// 	r.Run("application is synced", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		app := createTestApp(syncedAppWithHistory)
// 		current, previous := acrService.getRevisions(r.Context(), app)
// 		assert.Equal(t, app.Status.OperationState.SyncResult.Revision, current)
// 		assert.Equal(t, app.Status.History[len(app.Status.History)-2].Revision, previous)
// 	})

// 	r.Run("application sync is in progress", func(t *testing.T) {
// 		acrService := newTestACRService(&mocks.ApplicationClient{})
// 		app := createTestApp(syncedAppWithHistory)
// 		app.Status.Sync.Status = "Syncing"
// 		current, previous := acrService.getRevisions(r.Context(), app)
// 		assert.Equal(t, app.Operation.Sync.Revision, current)
// 		assert.Equal(t, app.Status.History[len(app.Status.History)-1].Revision, previous)
// 	})
// }

// func Test_ChangeRevision(r *testing.T) {
// 	r.Run("Change revision", func(t *testing.T) {
// 		client := &mocks.ApplicationClient{}
// 		client.On("GetChangeRevision", mock.Anything, mock.Anything).Return(&appclient.ChangeRevisionResponse{
// 			Revision: ptr.To("new-revision"),
// 		}, nil)
// 		acrService := newTestACRService(client)
// 		app := createTestApp(syncedAppWithHistory)

// 		err := acrService.ChangeRevision(r.Context(), app)
// 		require.NoError(t, err)

// 		app, err = acrService.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(r.Context(), app.Name, metav1.GetOptions{})
// 		require.NoError(t, err)

// 		assert.Equal(t, "new-revision", app.Status.OperationState.Operation.Sync.ChangeRevision)
// 	})

// 	r.Run("Change revision already exists", func(t *testing.T) {
// 		client := &mocks.ApplicationClient{}
// 		client.On("GetChangeRevision", mock.Anything, mock.Anything).Return(&appclient.ChangeRevisionResponse{
// 			Revision: ptr.To("new-revision"),
// 		}, nil)

// 		logger, logHook := test2.NewNullLogger()

// 		acrService := newTestACRService(client)
// 		acrService.logger = logger

// 		app := createTestApp(syncedAppWithHistory)

// 		err := acrService.ChangeRevision(r.Context(), app)
// 		require.NoError(t, err)

// 		app, err = acrService.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(r.Context(), app.Name, metav1.GetOptions{})
// 		require.NoError(t, err)

// 		assert.Equal(t, "new-revision", app.Status.OperationState.Operation.Sync.ChangeRevision)

// 		err = acrService.ChangeRevision(r.Context(), app)

// 		require.NoError(t, err)

// 		lastLogEntry := logHook.LastEntry()
// 		if lastLogEntry == nil {
// 			t.Fatal("No log entry")
// 		}

// 		require.Equal(t, "Change revision already calculated for application guestbook", lastLogEntry.Message)
// 	})
// }

// createTestAppInterfaceForApp mocks reading the application, and gives the
// caller access to the mocked application resource interface, so that patching
// can be mocked or asserted on.
func createTestAppInterfaceForApp(t *testing.T, app *appsv1.Application) (*mocks.Interface, *appmocks.ApplicationInterface) {
	t.Helper()
	appintMock := &appmocks.ApplicationInterface{}
	appintMock.On("Get", t.Context(), app.Name, metav1.GetOptions{}).Return(app, nil)

	av1alpha1 := &appmocks.ArgoprojV1alpha1Interface{}
	av1alpha1.On("Applications", app.Namespace).Return(appintMock)

	clientsetMock := &mocks.Interface{}
	clientsetMock.On("ArgoprojV1alpha1").Return(av1alpha1)
	return clientsetMock, appintMock
}

type changeRevisionReply struct {
	response *repoapiclient.ChangeRevisionResponse
	err      error
}

// createTestRepoclientForMSApp mocks the repo server for the sources of the
// multi-source test application whose git revision has changed. The requests
// for those sources are identical, so the replies are consumed in the order
// they are given.
func createTestRepoclientForMSApp(t *testing.T, app *appsv1.Application,
	repo *appsv1.Repository, replies ...changeRevisionReply,
) *repomocks.Clientset {
	t.Helper()
	changeRevisionRequest := &repoapiclient.ChangeRevisionRequest{
		AppName:          app.GetName(),
		Namespace:        app.GetNamespace(),
		CurrentRevision:  "CURRENT_REPO_02_000000000000000000000000",
		PreviousRevision: "HISTORY-1_REPO02_00000000000000000000000",
		Paths:            GetSourceRefreshPaths(app, app.Spec.Sources[0].Path),
		Repo:             repo,
	}
	clientmock := &repomocks.RepoServerServiceClient{}
	for _, reply := range replies {
		clientmock.On("GetChangeRevision", t.Context(), changeRevisionRequest).
			Return(reply.response, reply.err).Once()
	}
	return &repomocks.Clientset{RepoServerServiceClient: clientmock}
}

func createTestArgoDbForMSApp(t *testing.T, app *appsv1.Application, repo *appsv1.Repository) *dbmocks.ArgoDB {
	t.Helper()
	db := dbmocks.ArgoDB{}
	db.On("GetRepository", t.Context(), app.Spec.Sources[0].RepoURL, app.Spec.Project).
		Return(repo, nil)
	return &db
}
