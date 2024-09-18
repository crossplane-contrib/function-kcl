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
apiVersion: v1
kind: Pod
metadata:
  annotations:
    crossplane.io/composition-resource-name: ""
  generateName: example-
  labels:
    crossplane.io/composite: example
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
spec:
  containers:
  - name: main
```
