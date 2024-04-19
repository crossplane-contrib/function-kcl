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
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: 'Unready resources: instance'
    reason: Creating
    status: "False"
    type: Ready
---
apiVersion: ec2.aws.upbound.io/v1beta1
kind: Instance
metadata:
  annotations:
    crossplane.io/composition-resource-name: instance
    crossplane.io/external-name: custom-external-name
  generateName: example-xr-
  labels:
    crossplane.io/composite: example-xr
  name: instance
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
```
