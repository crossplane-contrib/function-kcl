# Crossplane Composition Functions using KCL

[![Go Report Card](https://goreportcard.com/badge/kcl-lang.io/crossplane-kcl)](https://goreportcard.com/report/kcl-lang.io/crossplane-kcl)
[![GoDoc](https://godoc.org/kcl-lang.io/crossplane-kcl?status.svg)](https://godoc.org/kcl-lang.io/crossplane-kcl)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://kcl-lang.io/crossplane-kcl/blob/main/LICENSE)

## Introduction

Crossplane KCL function allows developers to use [KCL](https://kcl-lang.io/) (a DSL) to write composite logic without the need for repeated packaging of crossplane functions, and we support package management and the [KRM KCL specification](https://github.com/kcl-lang/krm-kcl), which allows for OCI/Git source and the reuse of [KCL's module ecosystem](https://artifacthub.io/packages/search?org=kcl&sort=relevance&page=1).

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
+ Read the `ObservedCompositeResource` from `option("params").oxr`.
+ Read the `ObservedComposedResources` from `option("params").ocds`.
+ Read the `DesiredCompositeResource` from `option("params").dxr`.
+ Read the `DesiredComposedResources` from `option("params").dcds`.
+ Read the environment variables. e.g. `option("PATH")` (**Not yet implemented**).

## Library

You can directly use [KCL standard libraries](https://kcl-lang.io/docs/reference/model/overview) such as `regex.match`, `math.log`.

## Tutorial

+ See [here](https://kcl-lang.io/docs/reference/lang/tour) to study more features of KCL.
