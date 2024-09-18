# Example Manifests

You can run your function locally and test it using `crossplane render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render xr.yaml composition.yaml functions.yaml -r
---
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
---
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  annotations:
    crossplane.io/composition-resource-name: provider-helm
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: provider-helm
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
spec:
  serviceAccountTemplate:
    metadata:
      name: provider-helm
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  annotations:
    crossplane.io/composition-resource-name: provider-helm-cluster-admin
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: provider-helm-cluster-admin
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: provider-helm-cluster-admin
subjects:
- kind: ServiceAccount
  name: provider-helm
  namespace: crossplane-system
---
apiVersion: helm.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  annotations:
    crossplane.io/composition-resource-name: helm-provider
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: helm-provider
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
spec:
  credentials:
    source: InjectedIdentity
```
