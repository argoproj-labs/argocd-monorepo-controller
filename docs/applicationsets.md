## Updating ApplicationSet Controller Configuration

By default, the ApplicationSet controller will remove any annotations
added by the Monorepo Controller. This triggers the Monorepo
Controller to immediately attempt to restore them, leading to a
"conflict loop" between the two controllers.

To prevent this, you must configure the ApplicationSet controller to
ignore specific annotations. It is recommended to do this at the
global level using the
`applicationsetcontroller.global.preserved.annotations` parameter in
the argocd-cmd-params-cm ConfigMap:

```yaml
data:
  applicationsetcontroller.global.preserved.annotations: "mrp-controller.argoproj.io/change-revision,mrp-controller.argoproj.io/change-revisions,mrp-controller.argoproj.io/git-revision,mrp-controller.argoproj.io/git-revisions"
```

If you use Helm to install ArgoCD, you should configure this in your `values.yaml` file under the `configs.params` section:

```yaml
configs:
  params:
    applicationsetcontroller.global.preserved.annotations: "mrp-controller.argoproj.io/change-revision,mrp-controller.argoproj.io/change-revisions,mrp-controller.argoproj.io/git-revision,mrp-controller.argoproj.io/git-revisions"
```

While you can also configure this for specific ApplicationSets using
the `preservedFields` property, a global configuration is generally
better. Without a global setting, any ApplicationSet that lacks this
`preservedFields` property, but utilizes the manifest-generate-path
annotation, will cause the controllers to enter a continuous
add/delete loop.

For more details, see the ArgoCD documentation: [Preserving changes made to an Applications annotations and labels](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Controlling-Resource-Modification/#preserving-changes-made-to-an-applications-annotations-and-labels).
