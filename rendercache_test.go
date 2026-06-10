package main

import (
	"context"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"

	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
)

// realRenderCache builds an enabled cache directly so tests don't juggle env vars.
func realRenderCache(max int, ttl time.Duration) *renderCache {
	return newRenderCache(max, ttl)
}

func TestNilRenderCacheIsNoOp(t *testing.T) {
	var c *renderCache
	if c.enabled() {
		t.Fatal("nil cache must be disabled")
	}
	if _, ok := c.lookup([]byte("x")); ok {
		t.Fatal("nil cache must miss")
	}
	c.store([]byte("x"), []byte("y")) // must not panic
	if h, m := c.stats(); h != 0 || m != 0 {
		t.Fatalf("nil cache stats must be zero, got %d/%d", h, m)
	}
}

func TestRenderCacheHitMiss(t *testing.T) {
	c := realRenderCache(8, 0)
	in := []byte("source+params")
	if _, ok := c.lookup(in); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.store(in, []byte("rendered"))
	got, ok := c.lookup(in)
	if !ok || string(got) != "rendered" {
		t.Fatalf("expected hit 'rendered', got %q ok=%v", got, ok)
	}
	if h, m := c.stats(); h != 1 || m != 1 {
		t.Fatalf("expected 1 hit 1 miss, got %d/%d", h, m)
	}
}

func TestRenderCacheStoresCopy(t *testing.T) {
	c := realRenderCache(8, 0)
	in := []byte("k")
	out := []byte("abc")
	c.store(in, out)
	out[0] = 'X' // mutate caller's buffer after store
	got, _ := c.lookup(in)
	if string(got) != "abc" {
		t.Fatalf("cache must keep a copy, got %q", got)
	}
}

func TestRenderCacheLRUEviction(t *testing.T) {
	c := realRenderCache(2, 0)
	c.store([]byte("a"), []byte("1"))
	c.store([]byte("b"), []byte("2"))
	// touch "a" so "b" becomes least-recently-used
	if _, ok := c.lookup([]byte("a")); !ok {
		t.Fatal("a should be present")
	}
	c.store([]byte("c"), []byte("3")) // evicts "b"
	if _, ok := c.lookup([]byte("b")); ok {
		t.Fatal("b should have been evicted")
	}
	if _, ok := c.lookup([]byte("a")); !ok {
		t.Fatal("a should still be present")
	}
	if _, ok := c.lookup([]byte("c")); !ok {
		t.Fatal("c should be present")
	}
}

func TestRenderCacheTTL(t *testing.T) {
	c := realRenderCache(8, time.Minute)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	c.store([]byte("k"), []byte("v"))
	now = now.Add(30 * time.Second)
	if _, ok := c.lookup([]byte("k")); !ok {
		t.Fatal("entry should be live before TTL")
	}
	now = now.Add(31 * time.Second) // total 61s > 60s
	if _, ok := c.lookup([]byte("k")); ok {
		t.Fatal("entry should be expired after TTL")
	}
}

func TestRenderCacheUpdateExisting(t *testing.T) {
	c := realRenderCache(8, 0)
	c.store([]byte("k"), []byte("v1"))
	c.store([]byte("k"), []byte("v2"))
	got, _ := c.lookup([]byte("k"))
	if string(got) != "v2" {
		t.Fatalf("expected updated value v2, got %q", got)
	}
	if c.ll.Len() != 1 {
		t.Fatalf("expected single entry after update, got %d", c.ll.Len())
	}
}

// TestRunFunctionCacheHitIsIdentical proves a cached reconcile produces the
// exact same response as the uncached one, and that the second identical call
// is served from cache.
func TestRunFunctionCacheHitIsIdentical(t *testing.T) {
	req := &fnv1.RunFunctionRequest{
		Meta: &fnv1.RequestMeta{Tag: "hello"},
		Input: resource.MustStructJSON(`{
			"apiVersion": "krm.kcl.dev/v1alpha1",
			"kind": "KCLInput",
			"metadata": {"name": "basic"},
			"spec": {
				"target": "Default",
				"source": "{\n    apiVersion: \"example.org/v1\"\n    kind: \"Generated\"\n metadata.annotations = {\"krm.kcl.dev/composition-resource-name\": \"custom-composition-resource-name\"}\n}"
			}
		}`),
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructJSON(`{"apiVersion":"example.org/v1","kind":"XR"}`),
			},
		},
	}

	f := &Function{log: logging.NewNopLogger(), cache: realRenderCache(16, 0)}

	first, err := f.RunFunction(context.Background(), req)
	if err != nil {
		t.Fatalf("first RunFunction: %v", err)
	}
	second, err := f.RunFunction(context.Background(), req)
	if err != nil {
		t.Fatalf("second RunFunction: %v", err)
	}

	if diff := cmp.Diff(first, second, protocmp.Transform()); diff != "" {
		t.Fatalf("cached response differs from uncached (-first +second):\n%s", diff)
	}
	if h, m := f.cache.stats(); h != 1 || m != 1 {
		t.Fatalf("expected exactly 1 hit and 1 miss, got %d hits %d misses", h, m)
	}
}
