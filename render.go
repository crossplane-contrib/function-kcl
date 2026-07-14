package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kcl-lang.io/cli/pkg/options"
	"kcl-lang.io/kpm/pkg/client"
	"kcl-lang.io/krm-kcl/pkg/edit"
	"kcl-lang.io/krm-kcl/pkg/source"
	"sigs.k8s.io/kustomize/kyaml/kio"

	fkcl "github.com/crossplane-contrib/function-kcl/input/v1alpha1"
)

// The default render path serializes the whole KCLInput to YAML, hands it to the
// krm-kcl byte-stream pipeline, which parses it back into kyaml RNodes, and then
// re-serializes those RNodes to JSON to build the KCL top-level arguments:
//
//	JSON (RawExtension) -> YAML -> parse -> RNode -> JSON -> KCL
//
// We already hold JSON, and KCL wants JSON. Everything in between is conversion,
// and for a composition whose observed state is large (a Kubernetes resource with
// a fat status subresource) it dominates the cost of a RunFunction call, along
// with the GC pressure from the allocation churn it creates.
//
// renderInline skips it: it assembles the KCL arguments straight from the bytes
// we already have and invokes the KCL runtime directly. It handles the inline
// `source` case, which is the common one for Crossplane compositions; anything
// else (oci://, git, http, a local path) falls back to the krm-kcl pipeline.

// renderInline runs the KCL program without the YAML round trip. ok is false when
// the input is not something this path handles, in which case the caller must
// fall back to the krm-kcl pipeline.
func renderInline(in *fkcl.KCLInput) (out []byte, ok bool, err error) {
	if !isInlineSource(in.Spec.Source) {
		return nil, false, nil
	}

	// Resolve dependencies the same way krm-kcl's KCLRun.Transform does.
	var dependencies []string
	if in.Spec.Dependencies != "" {
		cli, err := client.NewKpmClient()
		if err != nil {
			return nil, true, err
		}
		if dependencies, err = edit.LoadDepListFromConfig(cli, in.Spec.Dependencies); err != nil {
			return nil, true, err
		}
	}

	dir, err := os.MkdirTemp("", "kcl-sandbox")
	if err != nil {
		return nil, true, err
	}
	defer os.RemoveAll(dir)

	prog := filepath.Join(dir, "prog.k")
	if err := os.WriteFile(prog, []byte(in.Spec.Source), 0o600); err != nil {
		return nil, true, err
	}

	args, err := kclArguments(in)
	if err != nil {
		return nil, true, err
	}

	buf := bytes.NewBuffer(nil)
	opts := options.NewRunOptions()
	opts.NoStyle = true
	opts.Entries = []string{prog}
	opts.Arguments = args
	opts.Writer = buf
	if len(dependencies) > 0 {
		opts.ExternalPackages = dependencies
	}
	if c := &in.Spec.Config; c != nil {
		opts.Debug = c.Debug
		opts.DisableNone = c.DisableNone
		opts.Overrides = c.Overrides
		opts.PathSelectors = c.PathSelectors
		opts.Settings = c.Settings
		opts.ShowHidden = c.ShowHidden
		opts.SortKeys = c.SortKeys
		opts.StrictRangeCheck = c.StrictRangeCheck
		opts.Vendor = c.Vendor
		opts.Arguments = append(opts.Arguments, c.Arguments...)
	}

	if err := opts.Complete([]string{}); err != nil {
		return nil, true, err
	}
	if err := opts.Validate(); err != nil {
		return nil, true, err
	}
	if err := opts.Run(); err != nil {
		return nil, true, err
	}

	// KCL emits every top-level variable; krm-kcl's contract is that the resources
	// live under `items`. Unwrap exactly as SimpleTransformer.Transform does. This
	// operates on the output, which is small — the saving is all on the input side.
	nodes, err := (&kio.ByteReader{Reader: buf, OmitReaderAnnotations: true}).Read()
	if err != nil {
		return nil, true, err
	}
	items, _, err := edit.UnwrapResources(nodes)
	if err != nil {
		return nil, true, err
	}

	res := bytes.NewBuffer(nil)
	if err := (&kio.ByteWriter{Writer: res}).Write(items); err != nil {
		return nil, true, err
	}
	return res.Bytes(), true, nil
}

// renderKey returns bytes that uniquely identify a render: source, dependencies,
// params, config and target all live in the input. It is JSON rather than YAML
// because the params are already JSON, so this is a single cheap pass.
func renderKey(in *fkcl.KCLInput) ([]byte, error) { return json.Marshal(in) }

// isInlineSource mirrors the fallthrough branch of krm-kcl's SourceToTempEntry:
// anything that is not a recognised remote or local location is inline KCL code.
func isInlineSource(src string) bool {
	return !source.IsOCI(src) &&
		!source.IsLocal(src) &&
		!source.IsRemoteUrl(src) &&
		!source.IsGit(src) &&
		!source.IsVCSDomain(src)
}

// kclArguments builds the KCL top-level arguments directly from the input. The
// params are already JSON (runtime.RawExtension), so the payload is copied rather
// than re-encoded.
func kclArguments(in *fkcl.KCLInput) ([]string, error) {
	// functionConfig is the KCLRun itself. RawExtension marshals as raw JSON, so
	// this is a single pass over the payload.
	fc, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	params, err := paramsJSON(in)
	if err != nil {
		return nil, err
	}

	var rl bytes.Buffer
	rl.WriteString(`{"apiVersion":"config.kubernetes.io/v1","kind":"ResourceList","items":[],"functionConfig":`)
	rl.Write(fc)
	rl.WriteByte('}')

	env, err := envJSON()
	if err != nil {
		return nil, err
	}

	return []string{
		"resource_list=" + rl.String(),
		"items=[]",
		"params=" + string(params),
		"PATH=" + os.Getenv("PATH"),
		"env=" + string(env),
	}, nil
}

// paramsJSON assembles {"oxr":<raw>,"dxr":<raw>,...} from the raw JSON we hold.
// Keys are sorted so the result is deterministic.
func paramsJSON(in *fkcl.KCLInput) ([]byte, error) {
	keys := make([]string, 0, len(in.Spec.Params))
	for k := range in.Spec.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		if raw := in.Spec.Params[k].Raw; len(raw) > 0 {
			b.Write(raw)
		} else {
			b.WriteString("{}")
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func envJSON() ([]byte, error) {
	m := make(map[string]string, len(os.Environ()))
	for _, e := range os.Environ() {
		if kv := strings.SplitN(e, "=", 2); len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	return json.Marshal(m)
}
