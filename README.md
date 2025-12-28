# ArgoCD Monorepo Controller

[![Documentation Status](https://readthedocs.org/projects/argocd-monorepo-controller/badge/?version=latest)](https://argocd-monorepo-controller-dev.readthedocs.io/en/latest/?badge=latest)
[![codecov](https://codecov.io/gh/dudinea/argocd-monorepo-controller/branch/dev/graph/badge.svg)](https://codecov.io/gh/dudinea/argocd-monorepo-controller)

## What is ArgoCD Monorepo Controller?

This controller is an ArgoCD addon that accurately tracks last commits
that actually changed the application (Change Revision). It is mostly
useful when several Applications are looking at different paths at
the same repository/branch (monorepos) .

## Documentation

To learn about the monorepo controller please go to the [Project documentation](https://argocd-monorepo-controller.readthedocs.io/en/latest/).

Please see also the original [Proposal](https://github.com/argoproj/argo-cd/issues/23366) for project motivation,
overall architecture and description the program functionality.

## What is its development status?

This is a newly created Argoproj-Labs project. It is WIP and it  may not
be fully  production ready yet.

_USE AT YOUR OWN RISK!_

## Installation

The controller should be installed into the namespace of an
existing ArgoCD instance (the `argocd` namespace  in most cases).

One quick way to try it is to use command like this:
```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/main/manifests/install.yaml
```

You can as well install it using kustomize an Helm, see the
[installation documentation](https://argocd-monorepo-controller.readthedocs.io/en/latest/install) for more information.


## Development 

The project is based on essentially the same Makefile and other ArgoCD
infrastructure, so ArgoCD Developer Documentation can be currently
used.

One quick way to build and run it locally is:

```
kubectl config set-context --current --namespace=argocd   # set current context to the argocd namespace
make cli-local                                            # build the program binary
make run                                                  # uses goreman to both monorepo controller and its repo-server
```

## Community

 You can reach the developers via:

* [The argocd-monorepo-controller Slack channel](https://cloud-native.slack.com/archives/C0A19KCEURY)
* [Github Issues](https://github.com/argoproj-labs/argocd-monorepo-controller/issues)

## FAQ

* Q: Why call it "Monorepo Controller"? It’s not just for monorepos!
  This issue can happen anytime another commit lands during the
  polling period, even if the file is not related to the generation
  of Application manifests.

  
  A: Good point! We picked the name because it's catchy and easy to
  remember. It also reflects the scenario where users encounter this
  problem most often: working within a monorepo.


