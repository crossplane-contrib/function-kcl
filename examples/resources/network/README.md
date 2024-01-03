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
apiVersion: fn-demo.crossplane.io/v1alpha1
kind: Network
metadata:
  name: network-test-functions
---
apiVersion: ec2.aws.upbound.io/v1beta1
kind: InternetGateway
metadata:
  annotations:
    crossplane.io/composition-resource-name: basic-
  generateName: network-test-functions-
  labels:
    crossplane.io/composite: network-test-functions
  ownerReferences:
  - apiVersion: fn-demo.crossplane.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Network
    name: network-test-functions
    uid: ""
spec:
  forProvider:
    region: eu-west-1
    vpcIdSelector:
      matchControllerRef: true
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource ":InternetGateway"
severity: SEVERITY_NORMAL
step: normal
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: created resource ":VPC"
severity: SEVERITY_NORMAL
step: normal
```
