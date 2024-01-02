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
crossplane beta render xr.yaml composition.yaml functions.yaml -r
---
apiVersion: nopexample.org/v1
kind: XSubnetwork
metadata:
  name: test-xrender
---
apiVersion: nobu.dev/v1
kind: Bucket
metadata:
  annotations:
    crossplane.io/composition-resource-name: bucket
    nobu.dev/app: someapp
    nobu.dev/cueified: "true"
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  name: bucket
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XSubnetwork
    name: test-xrender
    uid: ""
spec:
  forProvider:
    policy: some-bucket-policy
---
apiVersion: nobu.dev/v1
kind: User
metadata:
  annotations:
    crossplane.io/composition-resource-name: iam-user
    nobu.dev/app: someapp
    nobu.dev/cueified: "true"
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  name: iam-user
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XSubnetwork
    name: test-xrender
    uid: ""
spec:
  forProvider:
    name: somename
---
apiVersion: nobu.dev/v1
kind: Role
metadata:
  annotations:
    crossplane.io/composition-resource-name: iam-role
    nobu.dev/app: someapp
    nobu.dev/cueified: "true"
  generateName: test-xrender-
  labels:
    crossplane.io/composite: test-xrender
  name: iam-role
  ownerReferences:
  - apiVersion: nopexample.org/v1
    blockOwnerDeletion: true
    controller: true
    kind: XSubnetwork
    name: test-xrender
    uid: ""
spec:
  forProvider:
    policy: some-role-policy
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "bucket:"
severity: SEVERITY_NORMAL
step: normal
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "iam-role:"
severity: SEVERITY_NORMAL
step: normal
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "iam-user:"
severity: SEVERITY_NORMAL
step: normal
```
