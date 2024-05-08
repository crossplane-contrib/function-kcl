# Example Manifests

You can run your function locally and test it using `crossplane beta render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane beta render xr.yaml composition.yaml functions.yaml \
      --observed-resources=existing-observed-resources.yaml
---
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
---
metadata:
  annotations:
    crossplane.io/composition-resource-name: ocds
  generateName: example-
  labels:
    crossplane.io/composite: example
  name: ocds
  ownerReferences:
  - apiVersion: example.crossplane.io/v1beta1
    blockOwnerDeletion: true
    controller: true
    kind: XR
    name: example
    uid: ""
spec:
  ocds:
    test-user:
      ConnectionDetails: {}
      Resource:
        apiVersion: iam.aws.upbound.io/v1beta1
        kind: User
        metadata:
          annotations:
            crossplane.io/composition-resource-name: test-user
          generateName: example-
          labels:
            crossplane.io/composite: example
            dummy: foo
            testing.upbound.io/example-name: test-user
          name: test-user
          ownerReferences:
          - apiVersion: example.crossplane.io/v1beta1
            blockOwnerDeletion: true
            controller: true
            kind: XR
            name: example
            uid: ""
        spec:
          forProvider: {}
  user_metadata:
    annotations:
      crossplane.io/composition-resource-name: test-user
    generateName: example-
    labels:
      crossplane.io/composite: example
      dummy: foo
      testing.upbound.io/example-name: test-user
    name: test-user
    ownerReferences:
    - apiVersion: example.crossplane.io/v1beta1
      blockOwnerDeletion: true
      controller: true
      kind: XR
      name: example
      uid: ""
```
