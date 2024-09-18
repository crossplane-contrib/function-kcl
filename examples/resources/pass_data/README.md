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
kind: XSubnetwork
metadata:
  annotations:
    nobu.dev/app: someapp
    nobu.dev/cueified: "true"
  name: test-xrender
spec:
  forProvider:
    network: somenetwork
---
apiVersion: render.crossplane.io/v1beta1
kind: Result
message: updated xr ":XSubnetwork"
severity: SEVERITY_NORMAL
step: normal
```
