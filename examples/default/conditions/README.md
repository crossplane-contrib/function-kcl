# Example Manifests

You can run your function locally and test it using `crossplane render`
with these example manifests.

```shell
# Run the function locally
$ go run . --insecure --debug
```

```shell
# Then, in another terminal, call it with these example manifests
$ crossplane render --verbose xr.yaml composition.yaml functions.yaml -r
---
---
apiVersion: example.crossplane.io/v1beta1
kind: XR
metadata:
  name: example
status:
  conditions:
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    reason: Available
    status: "True"
    type: Ready
  - lastTransitionTime: "2024-01-01T00:00:00Z"
    message: Encountered an error creating the database
    reason: FailedToCreate
    status: "False"
    type: DatabaseReady
```
