# Crossplane Composition Functions using KCL

[![Go Report Card](https://goreportcard.com/badge/github.com/crossplane-contrib/function-kcl)](https://goreportcard.com/report/github.com/crossplane-contrib/function-kcl)
[![GoDoc](https://godoc.org/github.com/crossplane-contrib/function-kcl?status.svg)](https://godoc.org/github.com/crossplane-contrib/function-kcl)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/crossplane-contrib/function-kcl/blob/main/LICENSE)

## Introduction

Crossplane KCL function allows developers to use [KCL](https://kcl-lang.io/) (a DSL) to write composite logic without the need for repeated packaging of crossplane functions, and we support package management and the [KRM KCL specification](https://github.com/kcl-lang/krm-kcl), which allows for OCI/Git source and the reuse of [KCL's module ecosystem](https://artifacthub.io/packages/search?org=kcl&sort=relevance&page=1).

Check out this [blog](https://blog.crossplane.io/function-kcl/) to learn more.

## Developing

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=kcllang/crossplane-kcl

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=kcllang/crossplane-kcl

# Push a function package to the registry
$ crossplane --verbose xpkg push -f package/*.xpkg xpkg.upbound.io/crossplane-contrib/function-kcl:v0.3.2
```

## Quick Start Examples and Debug Locally

See [here](./examples/resources/basic/)

## Install the KCL Function to Cluster

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: kcl-function
spec:
  package: xpkg.upbound.io/crossplane-contrib/function-kcl:v0.3.2
EOF
```

## Debugging the KCL Function in Cluster

Logs are emitted to the Function's pod logs. Look for the Function pod in `crossplane-system`.

### Levels

```shell
Info   # default
Debug  # run with --debug flag
```

## Expected Output

A KRM YAML list which means that each document must have an `apiVersion`, `kind`

## Guides for Developing KCL

Here's what you can do in the KCL script:

+ Return an error using `assert {condition}, {error_message}`.
+ Read the `ObservedCompositeResource` from `option("params").oxr`.
+ Read the `ObservedComposedResources` from `option("params").ocds`.
+ Read the `DesiredCompositeResource` from `option("params").dxr`.
+ Read the `DesiredComposedResources` from `option("params").dcds`.

## Library

You can directly use [KCL standard libraries](https://kcl-lang.io/docs/reference/model/overview) such as `regex.match`, `math.log`.

## Tutorial

+ See [here](https://kcl-lang.io/docs/reference/lang/tour) to study more features of KCL.
