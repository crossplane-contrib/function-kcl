# Crossplane Composition Functions using KCL

[![Go Report Card](https://goreportcard.com/badge/github.com/crossplane-contrib/function-kcl)](https://goreportcard.com/report/github.com/crossplane-contrib/function-kcl)
[![GoDoc](https://godoc.org/github.com/crossplane-contrib/function-kcl?status.svg)](https://godoc.org/github.com/crossplane-contrib/function-kcl)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/crossplane-contrib/function-kcl/blob/main/LICENSE)

## Introduction

Crossplane KCL function allows developers to use [KCL](https://kcl-lang.io/) (a DSL) to write composite logic without the need for repeated packaging of crossplane functions, and we support package management and the [KRM KCL specification](https://github.com/kcl-lang/krm-kcl), which allows for OCI/Git source and the reuse of [KCL's module ecosystem](https://artifacthub.io/packages/search?org=kcl&sort=relevance&page=1).

Check out these following blogs to learn more. 

+ [Using KCL Programming Language to Write Crossplane Composition Functions](https://blog.crossplane.io/function-kcl/)
+ [KCL: The Game-Changer for Crossplane Composition Building](https://blog.upbound.io/kcl-benefits-crossplane-composition-building)

Here's a simple example:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: example
spec:
  compositeTypeRef:
    apiVersion: example.crossplane.io/v1beta1
    kind: XR
  mode: Pipeline
  pipeline:
    - step: basic
      functionRef:
        name: function-kcl
      input:
        apiVersion: krm.kcl.dev/v1alpha1
        kind: KCLInput
        source: |
          # Read the XR
          oxr = option("params").oxr
          # Patch the XR with the status field
          dxr = {
              **option("params").dxr
              status.dummy = "cool-status"
          }
          # Construct a bucket
          bucket = {
              apiVersion = "s3.aws.upbound.io/v1beta1"
              kind = "Bucket"
              metadata.annotations: {
                  "krm.kcl.dev/composition-resource-name" = "bucket"
              }
              spec.forProvider.region = option("oxr").spec.region
          }
          # Return the bucket and patched XR
          items = [bucket, dxr]
    - step: automatically-detect-ready-composed-resources
      functionRef:
        name: function-auto-ready
```

## Install the KCL Function to Cluster

```yaml
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: kcl-function
spec:
  package: xpkg.upbound.io/crossplane-contrib/function-kcl:latest
EOF
```

## Using this Function

### Source Support

To use a `KCLInput` as the function config, the KCL source must be specified in the `source` field. Additional parameters can be specified in the `params` field. The params field supports any complex data structure as long as it can be represented in YAML. Besides, the function can load KCL codes from inline source, OCI source, Git source and FileSystem source.

+ Inline source example

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    {
        apiVersion = "s3.aws.upbound.io/v1beta1"
        kind = "Bucket"
        metadata.annotations: {
            "krm.kcl.dev/composition-resource-name" = "bucket"
        }
        spec.forProvider.region = option("oxr").spec.region
    }
```

+ OCI source example

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: oci://ghcr.io/kcl-lang/crossplane-xnetwork-kcl-function
```

+ Git source example

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: github.com/kcl-lang/modules/crossplane-xnetwork-kcl-function
```

+ FileSystem source example

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: ./path/to/kcl/file.k
```

> Note: You can't run the FileSystem example using `crossplane render` because it loads templates from a ConfigMap in the cluster. You can create a `ConfigMap` with the templates using the following command.

```shell
kubectl create configmap templates --from-file=templates.k -n crossplane-system
```

This `ConfigMap` will be mounted to the function pod and the templates will be available in the `/templates` directory. See the following function config for details.

```yaml
apiVersion: pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-kcl
spec:
  package: xpkg.upbound.io/crossplane-contrib/function-kcl:latest
  runtimeConfigRef:
    name: mount-templates
---
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  name: mount-templates
spec:
  deploymentTemplate:
    spec:
      selector: {}
      template:
        spec:
          containers:
            - name: package-runtime
              volumeMounts:
                - mountPath: /templates
                  name: templates
                  readOnly: true
          volumes:
            - name: templates
              configMap:
                name: templates
```

### Read the Function Requests and Values through the `option` Function

+ Read the [`ObservedCompositeResource`](https://docs.crossplane.io/latest/concepts/composition-functions/#observed-state) from `option("params").oxr`.
+ Read the [`ObservedComposedResources`](https://docs.crossplane.io/latest/concepts/composition-functions/#observed-state) from `option("params").ocds`.
+ Read the [`DesiredCompositeResource`](https://docs.crossplane.io/latest/concepts/composition-functions/#desired-state) from `option("params").dxr`.
+ Read the [`DesiredComposedResources`](https://docs.crossplane.io/latest/concepts/composition-functions/#desired-state) from `option("params").dcds`.
+ Read the [`function pipeline's context`](https://docs.crossplane.io/latest/concepts/composition-functions/#function-pipeline-context) from `option("params").ctx`.
+ Return an error using `assert {condition}, {error_message}`.
+ Log variable values using the function `print(variable)` and it will be output to the stdout of the function pod.
+ Read the PATH variables. e.g. `option("PATH")`.
+ Read the environment variables. e.g. `option("env")`.

### Custom Parameters

You can define your custom parameters in the `params` field and use `option("params").custom_key` to get the `custom_value`.

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  params:
    custom_key: custom_value
  source: oci://ghcr.io/kcl-lang/crossplane-xnetwork-kcl-function
```

### Source Credentials

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: kcl-xnetwork
spec:
  params:
    annotations:
      krm.kcl.dev/allow-insecure-source: "true" # For localhost OCI registry
  source: oci://ghcr.io/kcl-lang/crossplane-xnetwork-kcl-function
  credentials: # If private OCI registry
    url: https://<oci-host-url> # or KCL_SRC_URL environment variable
    username: <username> # or KCL_SRC_USERNAME environment variable
    password: <password> # or KCL_SRC_PASSWORD environment variable
```

### Run Config

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
spec:
  source: oci://ghcr.io/kcl-lang/crossplane-xnetwork-kcl-function
  config: # See [pkg/api/ConfigSpec]
    vendor: true
    sortKeys: true
    disableNone: true
    # omit other fields
```

### Dependencies

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
spec:
  # Set the dependencies are the external dependencies for the KCL code.
  # The format of the `dependencies` field is same as the [dependencies]` in the `kcl.mod` file
  dependencies:
    k8s = "1.31"
  source: |
    import k8s.api.core.v1 as k8core

    k8core.Pod {
        spec: k8core.PodSpec{
            containers: [{
                name = "main"
            }]
        }
    }
```

### Expect Output

A KRM YAML list means that each document must have an `apiVersion`, `kind` through the `items` field or a single YAML output.

+ Using the `items` field

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    items = [{
        apiVersion: "ec2.aws.upbound.io/v1beta1"
        kind: "Instance"
        metadata.name = "instance1"
        spec.forProvider.region: "us-east-2"
    }, {
        apiVersion: "ec2.aws.upbound.io/v1beta1"
        kind: "Instance"
        metadata.name = "instance2"
        spec.forProvider.region: "us-east-2"
    }]
```

+ Single YAML output

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    {
        apiVersion: "ec2.aws.upbound.io/v1beta1"
        kind: "Instance"
        metadata.name = "instance"
        spec.forProvider.ami: "ami-0d9858aa3c6322f73"
        spec.forProvider.instanceType: "t2.micro"
        spec.forProvider.region: "us-east-2"
    }
```

> Note that when returning multiple resources, we need to set different `metadata.name` or `metadata.annotations."krm.kcl.dev/composition-resource-name" ` to distinguish between different resources in the composition functions.

### Target Support

The KCL function can target various types of objects:

+ `Default`: create new resources and set fields on the XR.
+ `Resources`: create new resources.
+ `PatchDesired`: set fields on existing DesiredComposed Resources.
+ `PatchResources`: set fields on existing resources fields. These resources will then be added to the desired resources map.
+ `XR`: set fields on the XR.

This is controlled by fields on the `KCInput`

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  # default: Default
  target: Default | PatchDesired | PatchResources | Resources | XR
  source: |
    # Omit the source field
    ...
```

### Extract Data from a Specific Composed Resource

To extract data from a specific composed resource by using the resource name, we can use the `option("params").ocds` variable, `ocds` is a mapping that its key is the resource name and its value is the [`observed composed resource`](https://pkg.go.dev/github.com/crossplane/function-sdk-go@v0.2.0/resource#ObservedComposed) like [the example](./examples/default/read_ocds_resource/composition.yaml).

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: show-ocds
spec:
  source: |
    {
        metadata.name = "ocds"
        spec.ocds = option("params").ocds
        spec.user_kind = option("params").ocds["test-user"]?.Resource.Kind
        spec.user_metadata = option("params").ocds["test-user"]?.Resource.metadata
        spec.user_status = option("params").ocds["test-user"]?.Resource.status
    }
```

### Composite Resource Connection Details

To return desired composite resource connection details, include a KCL config that produces the special CompositeConnectionDetails resource like [the example](./examples/default/connection_details/composition.yaml):

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    details = {
        apiVersion: "meta.krm.kcl.dev/v1alpha1"
        kind: "CompositeConnectionDetails"
        data: {
            "connection-secret-key": "connection-secret-value"
        }
    }
    # Omit other composite logics.
    # Input the details resource into the return resource list.
    items = [
        details
        # Omit other return resources.
    ]
```

> Note: The value of the connection secret value must be base64 encoded. This is already the case if you are referencing a key from a managed resource's connectionDetails field. However, if you want to include a connection secret value from somewhere else, you will need to use the `base64.encode` function:

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    import base64

    # Omit other logic
    ocds = option("params").ocds
    details = {
        apiVersion: "meta.krm.kcl.dev/v1alpha1"
        kind: "CompositeConnectionDetails"
        data: {
            "server-endpoint" = base64.encode(ocds["my-server"].Resource.status.atProvider.endpoint)
        }
    }
```

To mark a desired composed resource as ready, use the `krm.kcl.dev/ready` annotation:

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    # Omit other logic
    user = {
        apiVersion: "iam.aws.upbound.io/v1beta1"
        kind: "User"
        metadata.name = "test-user"
        metadata.annotations: {
            "krm.kcl.dev/ready": "True"
        }
    }
```

### Extra resources
By defining one or more special `ExtraResources`, you can ask Crossplane to retrieve additional resources from the local cluster and make them available to your templates. See the [docs](https://github.com/crossplane/crossplane/blob/main/design/design-doc-composition-functions-extra-resources.md) for more information.

> With ExtraResources, you can fetch cluster-scoped resources, but not namespaced resources such as claims.
> If you need to get a composite resource via its claim name you can use `matchLabels` with `crossplane.io/claim-name: <claimname>`

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    # Omit other logic
    details = {
        apiVersion: "meta.krm.kcl.dev/v1alpha1"
        kind: "ExtraResources"
        requirements = {
            foo = {
                apiVersion: "example.com/v1beta1",
                kind: "Foo",
                matchLabels: {
                    "foo": "bar"
                }
            },
            bar = {
                apiVersion: "example.com/v1beta1",
                kind: "Bar",
                matchName: "my-bar"
            }
        }
    }

    # Omit other composite logics.
    items = [
        details
        # Omit other return resources.
    ]
```
You can retrieve the extra resources either via labels with `matchLabels` or via name with `matchName: somename`.

This will result in Crossplane receiving the requested resources and make them available with the following format.
```yaml
foo:
- Resource:
    apiVersion: example.com/v1beta1
    kind: Foo
    metadata:
      labels:
        foo: bar
    # Omitted for brevity
- Resource:
    apiVersion: example.com/v1beta1
    kind: Foo
    metadata:
      labels:
        foo: bar
    # Omit for brevity
bar:
- Resource:
    apiVersion: example.com/v1beta1
    kind: Bar
    metadata:
      name: my-bar
    # Omitted for brevity
```
You can access the retrieved resources in your code like this:
```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    er = option("params")?.extraResources
    
    name = er?.bar[0]?.Resource?.metadata?.name or ""
    # Omit other logic
```

### Patching the XR status field

You can read the XR, patch it with the status field and return the new patched XR in the `item` result like this

```yaml
apiVersion: krm.kcl.dev/v1alpha1
kind: KCLInput
metadata:
  name: basic
spec:
  source: |
    # Read the XR
    dxr = option("params").dxr
    # Patch the XR with the status field
    dxr.status.dummy = "cool-status"
    items = [dxr] # Omit other resources
```

## Library

You can directly use [KCL standard libraries](https://kcl-lang.io/docs/reference/model/overview) such as `regex.match`, `math.log`.

## Tutorial

See [here](https://kcl-lang.io/docs/reference/lang/tour) to study more features such as conditions and loops in KCL.

## Examples

More examples can be found [here](./examples/)

## Debugging the KCL Function in Cluster

Logs are emitted to the Function's pod logs. Look for the Function pod in `crossplane-system`.

### Levels

```shell
Info   # default
Debug  # run with --debug flag
```

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
$ crossplane --verbose xpkg push -f package/*.xpkg xpkg.upbound.io/crossplane-contrib/function-kcl:latest
```
