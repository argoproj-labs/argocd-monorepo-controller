# Argo CD Monorepo Controller

[![Documentation Status](https://readthedocs.org/projects/argocd-monorepo-controller-dev/badge/?version=latest)](https://argocd-monorepo-controller-dev.readthedocs.io/en/latest/?badge=latest)
[![codecov](https://codecov.io/gh/argoproj-labs/argocd-monorepo-controller/branch/master/graph/badge.svg)](https://codecov.io/gh/argoproj-labs/argocd-monorepo-controller)


## Introduction

This controller is an ArgoCD addon that accurately tracks last commits
that actually changed the application (Change Revision). It is usefull
when several Applications are looking at different paths at the same
repository/branch (monorepos).

!!!warning "A Note on the Current Status"
    Argo CD Image Updater is under active development.
    You are welcome to test it out on non-critical environments, and of
    course to contribute.

## Installation

The controller should be installed into the namespace of an
existing ArgoCD instance (the `argocd` namespace  in most cases).

One quick way to try it is to use command like this:
```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/main/manifests/install.yaml
```

Or use `kustomize` to install kustomization from
https://github.com/argoproj-labs/argocd-monorepo-controller/tree/main/manifests

## Configuring notifications

See sample triggers and templates in samples/notifications.


## Development 

The project is based on essencially the same Makefile and other 
Argocd infrastructure, so Argocd Developer Documentation 
can be currently used.

One quick way to build and run it locally is:

```
kubectl config set-context --current --namespace=argocd   # set current context to the argocd namespace
make cli-local                                            # build the program binary
make run                                                  # uses goreman to both monorepo controller and its repo-server
```


## Community

 You can reach the developers via the following channels:

* Q & A : [Github Discussions](https://github.com/argoproj-labs/argocd-monorepo-controller/discussions)  [TBD]
* Chat : [The monorepo-controller Slack channel](https://argoproj.github.io/community/join-slack)  [TBD]
* [Github Issues](https://github.com/argoproj-labs/argocd-monorepo-controller/issues)

