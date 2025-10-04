# Example Manifests

You can run your function locally and test it using `crossplane render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render --verbose xr.yaml composition.yaml functions.yaml -r --extra-resources extra_resources_namespaced.yaml
---
---
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: 'Unready resources: another-awesome-dev-bucket, my-awesome-dev-bucket'
    reason: Creating
    status: "False"
    type: Ready
---
apiVersion: example/v1alpha1
kind: Foo
metadata:
  annotations:
    crossplane.io/composition-resource-name: another-awesome-dev-bucket
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: another-awesome-dev-bucket
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
---
apiVersion: example/v1alpha1
kind: Foo
metadata:
  annotations:
    crossplane.io/composition-resource-name: my-awesome-dev-bucket
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: my-awesome-dev-bucket
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
```
