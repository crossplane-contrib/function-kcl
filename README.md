# function-template-go
[![CI](https://github.com/crossplane/function-template-go/actions/workflows/ci.yml/badge.svg)](https://github.com/crossplane/function-template-go/actions/workflows/ci.yml)

A template for writing a [composition function][functions] in [Go][go].

To learn how to use this template:

* [Follow the guide to writing a composition function in Go][function guide]
* [Learn about how composition functions work][functions]
* [Read the function-sdk-go package documentation][package docs]

If you just want to jump in and get started:

1. Replace `function-template-go` with your function's name in `go.mod`,
   `package/crossplane.yaml`, and any Go imports
1. Update `input/v1beta1/` to reflect your desired input (and run `go generate`)
1. Add your logic to `RunFunction` in `fn.go`
1. Add tests for your logic in `fn_test.go`
1. Update this file, `README.md`, to be about your function!

This template uses [Go][go], [Docker][docker], and the [Crossplane CLI][cli] to
build functions.

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime
```

[functions]: https://docs.crossplane.io/latest/concepts/composition-functions
[go]: https://go.dev
[function guide]: https://docs.crossplane.io/knowledge-base/guides/write-a-composition-function-in-go
[package docs]: https://pkg.go.dev/github.com/crossplane/function-sdk-go
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli