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
apiVersion: example.crossplane.io/v1
kind: XR
metadata:
  name: example-xr
---
apiVersion: ec2.aws.upbound.io/v1beta1
kind: Instance
metadata:
  annotations:
    crossplane.io/composition-resource-name: basic-instance-us-east-1
  generateName: example-xr-
  labels:
    crossplane.io/composite: example-xr
  name: instance-us-east-1
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    ami: ami-0d9858aa3c6322f73
    instanceType: t2.micro
    region: us-east-1
---
apiVersion: ec2.aws.upbound.io/v1beta1
kind: Instance
metadata:
  annotations:
    crossplane.io/composition-resource-name: basic-instance-us-east-2
  generateName: example-xr-
  labels:
    crossplane.io/composite: example-xr
  name: instance-us-east-2
  ownerReferences:
  - apiVersion: example.crossplane.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example-xr
    uid: ""
spec:
  forProvider:
    ami: ami-0d9858aa3c6322f73
    instanceType: t2.micro
    region: us-east-2
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "instance-us-east-1:Instance"
severity: SEVERITY_NORMAL
step: normal
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource "instance-us-east-2:Instance"
severity: SEVERITY_NORMAL
step: normal
```
