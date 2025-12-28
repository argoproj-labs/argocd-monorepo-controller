# Installation Methods

The controller should be installed in the same Kubernetes namespace
that Argo CD is running in (the `argocd` namespace in most cases).
The provided installation files plug the Monorepo Controller
components in the existing Argo CD installation, reusing its
configuration in the `argocd-cmd-params-cm` and `argocd-cm`
configmaps.

Potentially it is possible to run the controller from another
namespace, but it will require extra configuration and currently this
is not supported.

!!! note
    If you use ApplicationSet controller it is important to 
	[configure it to ignore Monorepo Controller Annotations](applicationsets.md)

## Method 1: Installing using provided plain manifests

We provide two installation manifests: 

* `install.yaml` - with cluster-wide permissions to watch Application
  manifests in any namespece
* `install-namespaced.yaml` - for a namespaced ArgoCD instance that
  will watch Application only in the ArgoCD namespace.

### Cluster-wide Installation

Apply the manifest in the ArgoCD namespace:

```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/stable/manifests/install.yaml
```

!!! warning 
    The installation manifests include `ClusterRoleBinding`
    resources that reference `argocd` namespace. If you are installing
    Argo CD into a different namespace then make sure to update the
    namespace reference in the `install.yaml` file.

### Namespaced Installation

Apply the manifest in the ArgoCD namespace:

```
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/stable/manifests/install-namespaced.yaml
```

## Method 2: Installing using kustomize

The Monorepo Controller manifests can also be installed using
Kustomize.  You may include the above manifests as a remote resource and
apply additional customizations using Kustomize patches.


For example

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: argocd
resources:
- https://raw.githubusercontent.com/argoproj-labs/argocd-monorepo-controller/refs/heads/stable/manifests/install.yaml
```

## Method 3: Installing using Helm


The Monorepo Controller manifests can also be installed using Helm. 

The Helm chart is maintained in the same Git repository as the Monorepo Controller itself
and released together with the Monorepo Controller.

In simple cases it can be installed with the command:

```shell
helm install <RELEASE-NAME> --namespace argocd "quay.io/eugened/argocd-monorepo-controller:<VERSION>"
```

* `<RELEASE-NAME>` - user selected release name
* `<VERSION>` - chart version, which is same as the version of the application release

In more complex cases one would need to customize the `values.yaml` file. 
See Helm Chart [Documentation](helm.md) for available options.


