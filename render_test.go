package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"kcl-lang.io/krm-kcl/pkg/api"
	"kcl-lang.io/krm-kcl/pkg/api/v1alpha1"
	krmkio "kcl-lang.io/krm-kcl/pkg/kio"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/yaml"

	fkcl "github.com/crossplane-contrib/function-kcl/input/v1alpha1"
)

// renderViaPipeline is the pre-existing path: marshal the input to YAML and let
// the krm-kcl byte-stream pipeline parse it back. Kept here as the oracle the
// fast path must agree with.
func renderViaPipeline(t testing.TB, in *fkcl.KCLInput) []byte {
	t.Helper()
	b, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.NewBuffer(nil)
	if err := krmkio.NewPipeline(bytes.NewBuffer(b), out, false).Execute(); err != nil {
		t.Fatalf("krm-kcl pipeline: %v", err)
	}
	return out.Bytes()
}

// canonical renders KRM output into a stable, comparable form.
func canonical(t testing.TB, b []byte) string {
	t.Helper()
	nodes, err := (&kio.ByteReader{Reader: bytes.NewBuffer(b), OmitReaderAnnotations: true}).Read()
	if err != nil {
		t.Fatalf("reading KRM output: %v", err)
	}
	docs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		j, err := n.MarshalJSON()
		if err != nil {
			t.Fatalf("marshalling node: %v", err)
		}
		var v any
		if err := json.Unmarshal(j, &v); err != nil {
			t.Fatalf("unmarshalling node: %v", err)
		}
		c, err := json.Marshal(v) // re-marshal to sort map keys
		if err != nil {
			t.Fatalf("canonicalising node: %v", err)
		}
		docs = append(docs, string(c))
	}
	sort.Strings(docs)
	return strings.Join(docs, "\n")
}

func mustRaw(t testing.TB, v any) runtime.RawExtension {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return runtime.RawExtension{Raw: b}
}

// observed fabricates a composed resource whose status is padded to roughly
// padBytes, standing in for a real Kubernetes resource with a chatty status.
func observed(padBytes int) map[string]any {
	conds := []any{}
	for i := 0; len(fmt.Sprint(conds)) < padBytes && i < 500; i++ {
		conds = append(conds, map[string]any{
			"type":               fmt.Sprintf("Condition%d", i),
			"status":             "True",
			"reason":             "SomeReasonWithABitOfText",
			"message":            "a message of the kind controllers like to write repeatedly",
			"lastTransitionTime": "2024-01-01T00:00:00Z",
		})
	}
	return map[string]any{
		"resource": map[string]any{
			"Resource": map[string]any{
				"apiVersion": "example.org/v1",
				"kind":       "Thing",
				"metadata":   map[string]any{"name": "thing", "namespace": "default"},
				"spec":       map[string]any{"replicas": 3},
				"status":     map[string]any{"conditions": conds},
			},
			"Ready": "True",
		},
	}
}

func testInput(t testing.TB, source string, padBytes int) *fkcl.KCLInput {
	t.Helper()
	xr := map[string]any{
		"apiVersion": "example.org/v1",
		"kind":       "XR",
		"metadata":   map[string]any{"name": "xr", "namespace": "default"},
		"spec":       map[string]any{"replicas": 3, "name": "thing"},
		"status":     map[string]any{},
	}
	cds := observed(padBytes)

	in := &fkcl.KCLInput{}
	in.APIVersion = v1alpha1.KCLRunAPIVersion
	in.Kind = api.KCLRunKind
	in.Name = "test"
	in.Spec.Source = source
	in.Spec.Target = "Default"
	in.Spec.Params = map[string]runtime.RawExtension{
		"oxr":               mustRaw(t, xr),
		"dxr":               mustRaw(t, xr),
		"ocds":              mustRaw(t, cds),
		"dcds":              mustRaw(t, cds),
		"ctx":               mustRaw(t, map[string]any{"apiextensions.crossplane.io/environment": map[string]any{"region": "eu"}}),
		"extraResources":    mustRaw(t, map[string]any{}),
		"requiredResources": mustRaw(t, map[string]any{}),
	}
	return in
}

const (
	// Emits a resource built from params.
	srcEmit = `
oxr = option("params").oxr
items = [{
    apiVersion = "example.org/v1"
    kind = "Thing"
    metadata.name = str(oxr.spec.name)
    spec.replicas = int(oxr.spec.replicas)
}]
`
	// Reads the observed composed state.
	srcReadObserved = `
ocds = option("params")?.ocds or {}
ready = "resource" in ocds and str(ocds["resource"]?.Ready or "") == "True"
items = [{
    apiVersion = "example.org/v1"
    kind = "Thing"
    metadata.name = "thing"
    metadata.annotations = {"ready" = str(ready)}
}]
`
	// Emits nothing.
	srcEmpty = `
items = []
`
	// Reads the function context.
	srcContext = `
ctx = option("params").ctx
env = ctx?["apiextensions.crossplane.io/environment"] or {}
items = [{
    apiVersion = "example.org/v1"
    kind = "Thing"
    metadata.name = "thing"
    metadata.labels = {"region" = str(env?.region or "")}
}]
`
)

// TestRenderInlineMatchesPipeline is the core guarantee: for inline sources the
// fast path must produce exactly what the krm-kcl pipeline produces.
func TestRenderInlineMatchesPipeline(t *testing.T) {
	cases := map[string]string{
		"emit":          srcEmit,
		"read-observed": srcReadObserved,
		"empty":         srcEmpty,
		"context":       srcContext,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			in := testInput(t, src, 2000)

			want := canonical(t, renderViaPipeline(t, in))

			got, ok, err := renderInline(in)
			if err != nil {
				t.Fatalf("renderInline: %v", err)
			}
			if !ok {
				t.Fatal("renderInline declined an inline source")
			}
			if g := canonical(t, got); g != want {
				t.Errorf("output differs\n--- pipeline ---\n%s\n\n--- fast path ---\n%s", want, g)
			}
		})
	}
}

// TestRenderInlineDeclinesNonInlineSources: anything that is not inline code must
// fall back to the krm-kcl pipeline, which knows how to fetch it.
func TestRenderInlineDeclinesNonInlineSources(t *testing.T) {
	for _, src := range []string{
		"oci://ghcr.io/kcl-lang/set-annotations",
		"https://example.com/prog.k",
		"git::https://github.com/kcl-lang/krm-kcl",
		"github.com/kcl-lang/krm-kcl",
		"./local/prog.k",
	} {
		t.Run(src, func(t *testing.T) {
			in := testInput(t, src, 0)
			if _, ok, err := renderInline(in); ok || err != nil {
				t.Errorf("expected fall back to the pipeline, got ok=%v err=%v", ok, err)
			}
		})
	}
}

// BenchmarkRender shows the cost is dominated by the payload, not by the KCL.
func BenchmarkRender(b *testing.B) {
	for _, pad := range []int{0, 50_000, 200_000} {
		in := testInput(b, srcEmit, pad)
		size := len(mustMarshal(b, in)) / 1024

		b.Run(fmt.Sprintf("pipeline/input=%dKB", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				renderViaPipeline(b, in)
			}
		})
		b.Run(fmt.Sprintf("inline/input=%dKB", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, _, err := renderInline(in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func mustMarshal(t testing.TB, in *fkcl.KCLInput) []byte {
	t.Helper()
	b, err := yaml.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
