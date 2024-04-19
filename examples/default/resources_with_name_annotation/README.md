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
kind: Database
metadata:
  name: database-test-functions
---
apiVersion: sql.gcp.upbound.io/v1beta1
kind: DatabaseInstance
metadata:
  annotations:
    crossplane.io/composition-resource-name: database_annotation_name
  generateName: database-test-functions-
  labels:
    crossplane.io/composite: database-test-functions
  name: database
  ownerReferences:
  - apiVersion: fn-demo.crossplane.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Database
    name: database-test-functions
    uid: ""
spec:
  forProvider:
    project: test-project
    settings:
    - databaseFlags:
      - name: log_checkpoints
        value: "off"
---
apiVersion: sql.gcp.upbound.io/v1beta1
kind: DatabaseInstance
metadata:
  annotations:
    crossplane.io/composition-resource-name: database
  generateName: database-test-functions-
  labels:
    crossplane.io/composite: database-test-functions
  name: database
  ownerReferences:
  - apiVersion: fn-demo.crossplane.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Database
    name: database-test-functions
    uid: ""
spec:
  forProvider:
    project: test-project
    settings:
    - databaseFlags:
      - name: log_checkpoints
        value: "on"
```
