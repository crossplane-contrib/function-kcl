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
apiVersion: nopexample.org/v1
kind: XNopResource
metadata:
  name: test-xrender
---
apiVersion: eks.nobu.dev/v1beta
kind: XNodepool
metadata:
  annotations:
    crossplane.io/composition-resource-name: name
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  name: name
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XNopResource
    name: test-xrender
    uid: ""
spec:
  parameters:
    autoscaling:
    - maxNodeCount: 1
      minNodeCount: 1
    clusterName: example-injections
    region: us-east-2
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "name:XNodepool"
severity: SEVERITY_NORMAL
step: normal
```
