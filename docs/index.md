# Argo CD Monorepo Controller

[![Documentation Status](https://readthedocs.org/projects/argocd-monorepo-controller/badge/?version=latest)](https://argocd-monorepo-controller-dev.readthedocs.io/en/latest/?badge=latest)

## Introduction

This controller is an ArgoCD addon that accurately tracks last commits
that actually changed the application (Change Revision). It is useful
when several Applications are looking at different paths at the same
repository/branch (monorepos).

!!!warning "A Note on the Current Status"
    Argo CD Monorepo Controller is under active development.
    You are welcome to test it out on non-critical environments, and of
    course, to contribute.

## How It Works

It is a Kubernetes controller which continuously monitors ArgoCD
applications, handles changes in their Sync state [Git Revision](terminology.md#git-revision),
calculates [Change Revisions](terminology.md#change-revision) and records them in annotations to
the Application Manifests.

The controller usually runs in the same namespace as Argo CD and
reuses its configuration to be able to connect to Git repositories and
to monitor Application manifests in proper namespaces.

For more information see [Architecture Overview](architecture.md) as well as 
the original [Proposal](https://github.com/argoproj/argo-cd/issues/23366) for creation of the project.

## Installation

The controller should be installed into the namespace of an
existing ArgoCD instance (the `argocd` namespace  in most cases).

One quick way to try it is to use command like this:
```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/main/manifests/install.yaml
```

For additional details and other installation methods see our [installation documentation](install.md).

## Configuring notifications

See sample triggers and templates in
[samples/notifications](https://github.com/argoproj-labs/argocd-monorepo-controller/tree/main/samples/notifications).
For additional details see the [notifications configuration guide](install.md).

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

* [The monorepo-controller Slack channel](https://cloud-native.slack.com/archives/C0A19KCEURY) 
* [Github Issues](https://github.com/argoproj-labs/argocd-monorepo-controller/issues)

