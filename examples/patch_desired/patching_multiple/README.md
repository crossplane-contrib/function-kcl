# Example Manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane beta render xr.yaml composition.yaml functions.yaml -r
---
apiVersion: nopexample.org/v1
kind: XSubnetwork
metadata:
  name: test-xrender
---
apiVersion: v1
kind: XR
metadata:
  annotations:
    crossplane.io/composition-resource-name: bucket2
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  name: bucket2
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XSubnetwork
    name: test-xrender
    uid: ""
spec:
  forProvider:
    network: some-override-network2
---
apiVersion: v1
kind: XR
metadata:
  annotations:
    crossplane.io/composition-resource-name: bucket1
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XSubnetwork
    name: test-xrender
    uid: ""
spec:
  forProvider:
    network: some-override-network1
```
