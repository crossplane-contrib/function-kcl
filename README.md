# Crossplane Composition Functions using KCL

[![CI](https://github.com/crossplane/function-template-go/actions/workflows/ci.yml/badge.svg)](https://github.com/crossplane/function-template-go/actions/workflows/ci.yml)

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
```

## Quick Start Examples

See [here](./examples/resources/basic/)

## Expected Output

A KRM YAML list which means that each document must have an `apiVersion`, `kind`

## Guides for Developing KCL

Here's what you can do in the KCL script:

+ Return an error using `assert {condition}, {error_message}`.
+ Read the `DesiredCompositeResources` from `option("dxr")` (**Not yet implemented**).
+ Read the `ObservedCompositeResources` from `option("oxr")` (**Not yet implemented**).
+ Read the environment variables. e.g. `option("PATH")` (**Not yet implemented**).

## Library

You can directly use [KCL standard libraries](https://kcl-lang.io/docs/reference/model/overview) such as `regex.match`, `math.log`.

## Tutorial

+ See [here](https://kcl-lang.io/docs/reference/lang/tour) to study more features of KCL.
