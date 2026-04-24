# Proposed Crossplane KCL Composition Structure

This guide describes a practical project layout for Crossplane + KCL compositions and a delivery flow that keeps runtime updates small and predictable.

## Why this structure

- Use **typed KCL schemas and provider objects**, not plain YAML string generation.
- Keep orchestration logic in `main.k` and reusable calculations in `params.k`.
- Keep validation rules in `asserts.k` so failures happen early with explicit messages.
- Validate behavior with focused `tests/*.k` suites before publishing a new module tag.
- Publish a versioned OCI artifact and update only the tag in `Composition`.

## Recommended layout

```text
machine-deployment/
  main.k
  dxr.k
  params.k
  asserts.k
  instances.k
  disks.k
  loadbalancer.k
  loadbalancer_rule.k
  usages.k
  nics.k
  vm_static_scrape.k
  kcl.mod
  params.yaml
  tests/
    main_render_test.k
    params_test.k
    instance_test.k
    disk_test.k
    loadbalancer_test.k
    loadbalancer_rule_test.k
    usages_test.k
    nics_test.k
    vm_static_scrape_test.k
```

## Core files and responsibilities

### `params.yaml` / `kcl.yaml` (local input emulation)

Use an input file to emulate Crossplane runtime payload during development. This lets you test rendering and assertions before deploying.

Core idea:

- emulate `oxr` (input XR from Crossplane)
- emulate `ctx` (data from previous functions, for example environment config)
- emulate `ocds` (observed composed resources already created)

Minimal structure:

```yaml
kcl_options:
  - key: params
    value:
      ocds: {}
      ctx:
        "apiextensions.crossplane.io/environment": {}
      oxr:
        metadata:
          name: machine-service
        spec: {}
```

From your real `params.yaml` examples:

- `oxr` includes XR spec, e.g. `additionalNICNetworks`, `os`, `size`, `replicas`, `ports`.
- `ctx."apiextensions.crossplane.io/environment"` includes AMI/network/project/zone config gathered by other functions.
- `ocds` can contain data from already created resources, for example:

```yaml
ocds:
  "playti-service-vip":
    Resource:
      status:
        atProvider:
          ipAddress: "1.2.3.4"
```

That enables local validation of DXR patch logic like:

```kcl
status.externalIP.ip = params.ocds[params._vip_name]?.Resource?.status?.atProvider?.ipAddress or "Provisioning..."
```

Run commands:

- If file name is `params.yaml`: `kcl run -Y params.yaml`
- If file name is `kcl.yaml`: `kcl run`

### `composition.yaml` (pipeline entrypoint)

Keep the composition pipeline short and explicit:

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: machine-deployment
spec:
  mode: Pipeline
  pipeline:
    - step: gather-env-config
      functionRef:
        name: function-environment-configs
    - step: render-machine-deployment
      functionRef:
        name: function-kcl
      input:
        apiVersion: krm.kcl.dev/v1alpha1
        kind: KCLInput
        spec:
          # target: Default (or omit target)
          source: oci://registry.example.com/machine-deployment?tag=0.2.0
```

### `main.k`

Entry-point that imports each resource module and returns one `items` list for Crossplane:

```kcl
import instances as instances
import disks as disks
import loadbalancer as loadbalancer
import loadbalancer_rule as loadbalancer_rule
import vm_static_scrape as vm_static_scrape
import usages as usages
import nics as nics
import dxr as dxr_patch

items = [
    *instances._listOfInstances
    *disks._listOfDisks
    *loadbalancer._listOfLoadbalancers
    *loadbalancer_rule._listOfLoadbalancerRules
    *vm_static_scrape._listOfVMStaticScrapes
    *usages._listOfUsages
    *nics._listOfNICs
    dxr_patch._dxr
]
```

### `dxr.k` (patch XR status)

Use a dedicated module to patch the desired XR status:

```kcl
import params as params

_dxr = {
    **params.dxr
    status.externalIP.ip = params.ocds[params._vip_name]?.Resource?.status?.atProvider?.ipAddress or "Provisioning..."
}
```

Important behavior:

- This works when `spec.target` is `Default` (or not set, because `Default` is implicit).
- This does **not** work with `spec.target: Resources`, because `Resources` mode only returns composed resources and does not patch XR fields.

### `params.k` (minimal context include example)

```kcl
_params = option("params")

oxr = _params.oxr
env: any = _params.ctx?["apiextensions.crossplane.io/environment"] or {}
ocds = _params.ocds
dxr = _params.dxr or {}

_vip_name = "{}-vip".format(oxr.metadata.name)
```

### `kcl.mod`

Module metadata and dependency lock-point:

- module name and semantic version
- KCL edition
- OCI/path dependencies for Crossplane/provider schemas

Schema modules are connected here (provider/Crossplane APIs used by typed objects in files like `instances.k`):

```toml
[dependencies]
crossplane = { oci = "oci://registry.example.com/kcl-lang/crossplane", tag = "1.17.3", version = "1.17.3" }
crossplane-provider-cloudstack = { oci = "oci://registry.example.com/crossplane-provider-cloudstack", tag = "0.0.1", version = "0.0.1" }
```

After changing `kcl.mod` dependencies, run:

```shell
kcl mod update
```

This matches the helper workflow from `kcl_command_helper.md` and ensures schema modules are downloaded/updated before `kcl run`, `kcl test`, or packaging.

### `instances.k` (typed schema/object example)

Use provider object types directly (schema-backed resources), not ad-hoc YAML maps:

```kcl
import crossplane_provider_cloudstack.v1alpha1 as cloudstack
import params as params

build_instance = lambda hostname: str, size: str, os: str, revision: str -> cloudstack.Instance {
    cloudstack.Instance {
        metadata: {
            name: hostname
            annotations: {
                "krm.kcl.dev/composition-resource-name" = hostname
            }
        }
        spec: {
            forProvider: {
                serviceOffering: size
                template: params._resolve_template(size, os)
            }
        }
    }
}
```

This approach gives stronger contracts than raw YAML generation:

- API fields are aligned to provider schemas.
- Refactoring is safer across modules.
- Tests can assert behavior at object level.

### `params.k`

Central place for:

- runtime inputs from `option("params")` (`oxr`, `ocds`, `dxr`, `ctx`)
- naming and hash helpers
- upgrade/stateful switches
- template resolution
- derived runtime flags (readiness, pause conditions)

### `asserts.k`

Validation and guardrails:

- duplicate names and required field checks
- unsupported scenarios (for example, blocked combinations)
- naming length budget constraints
- environment/template resolvability checks

Small example:

```kcl
import params as params

assert params._vip_name != "", "VIP resource name must not be empty"
```

### `tests/*.k`

Use targeted tests for each module plus integration-style render checks.

At minimum:

- one render-count test for final `items`
- parameter derivation tests (`params.k`)
- resource-generation tests per module
- regression tests for naming and upgrade behavior

Small example (`tests/main_render_test.k` style):

```kcl
import ..main as main

test_main_items_render = lambda {
    assert len(main.items) > 0, "main.items must not be empty"
    True
}
```

## Pipeline composition example (tag-based)

```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: machine-deployment
spec:
  mode: Pipeline
  compositeTypeRef:
    apiVersion: platform.example.io/v1alpha1
    kind: MachineDeployment
  pipeline:
    - step: gather-env-config
      functionRef:
        name: function-environment-configs
      input:
        apiVersion: environmentconfigs.fn.crossplane.io/v1beta1
        kind: Input
        spec:
          environmentConfigs:
            - type: Reference
              ref:
                name: core-details
            - type: Reference
              ref:
                name: images
    - step: render-machine-deployment
      functionRef:
        name: function-kcl
      input:
        apiVersion: krm.kcl.dev/v1alpha1
        kind: KCLInput
        spec:
          # Keep target as Default (or omit target) if you patch XR via dxr.k
          # target: Default
          source: oci://registry.example.com/machine-deployment?tag=0.2.0
    - step: auto-ready
      functionRef:
        name: function-auto-ready
```

## Delivery flow: package once, replace only tag

1. Build module artifact:

```shell
kcl mod update
kcl mod pkg --vendor --target build
```

2. Push as OCI tag:

```shell
kcl mod push --vendor "oci://registry.example.com/machine-deployment?tag=0.2.0"
```

3. Update only the `source` tag in `Composition`:

```yaml
spec:
  pipeline:
    - step: render-machine-deployment
      input:
        spec:
          source: oci://registry.example.com/machine-deployment?tag=0.2.1
```

With this model, no large in-cluster template replacement is needed. The pipeline always pulls a versioned module archive, and rollout is controlled by tag changes.
