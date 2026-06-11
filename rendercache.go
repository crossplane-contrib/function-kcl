package main

import (
	"container/list"
	"crypto/sha256"
	"sync"
	"sync/atomic"
	"time"
)

// A Crossplane composition function is deterministic: the same RunFunctionRequest
// must always yield the same response. function-kcl recompiles and re-executes
// the whole KCL module on every reconcile, which is both the CPU hot spot
// (~4 cores under load) and the driver of the native (off-heap) memory leak.
//
// In steady state most reconciles are no-op re-syncs: Crossplane periodically
// reconciles converged composites with byte-identical inputs. The render cache
// short-circuits those — it memoises the KCL pipeline output keyed on the exact
// bytes fed to the pipeline (source + dependencies + all params + config), so an
// identical reconcile returns the cached output without touching the KCL native
// runtime at all. That cuts the CPU peg and avoids a recompile (hence a leak
// increment) on every cache hit.
//
// It does NOT help when inputs genuinely change every reconcile (e.g. a composite
// actively churning during a rollout) — those still recompile. The graceful
// recycler (recycle.go) is the backstop that bounds any residual growth.
//
// Safety: caching is sound because the function is deterministic over its input,
// and the cache key is the complete serialized input. It is opt-in via
// FUNCTION_KCL_RENDER_CACHE_SIZE (entries; 0 = disabled) with an optional TTL.

type renderCache struct {
	mu    sync.Mutex
	max   int
	ttl   time.Duration
	ll    *list.List // front = most recently used
	items map[string]*list.Element

	hits   atomic.Uint64
	misses atomic.Uint64

	now func() time.Time // injectable for tests
}

type renderCacheEntry struct {
	key    string
	value  []byte
	stored time.Time
}

const (
	envRenderCacheSize = "FUNCTION_KCL_RENDER_CACHE_SIZE"
	envRenderCacheTTL  = "FUNCTION_KCL_RENDER_CACHE_TTL"
)

// newRenderCacheFromEnv returns a cache configured from the environment, or nil
// when disabled (size <= 0). A nil *renderCache is a safe no-op.
func newRenderCacheFromEnv() *renderCache {
	return newRenderCache(int(envUint(envRenderCacheSize, 0)), envDuration(envRenderCacheTTL, 0))
}

// newRenderCache returns a cache holding up to max entries with an optional TTL,
// or nil when max <= 0 (disabled).
func newRenderCache(max int, ttl time.Duration) *renderCache {
	if max <= 0 {
		return nil
	}
	return &renderCache{
		max:   max,
		ttl:   ttl,
		ll:    list.New(),
		items: make(map[string]*list.Element, max),
		now:   time.Now,
	}
}

func (c *renderCache) enabled() bool { return c != nil }

// key derives the cache key from the serialized pipeline input.
func (c *renderCache) key(input []byte) string {
	sum := sha256.Sum256(input)
	return string(sum[:])
}

// lookup returns a cached pipeline output for the given input, if present and
// not expired. The returned slice is read-only for callers. Nil/disabled caches
// always miss.
func (c *renderCache) lookup(input []byte) ([]byte, bool) {
	if c == nil {
		return nil, false
	}
	k := c.key(input)
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[k]
	if !ok {
		c.misses.Add(1)
		return nil, false
	}
	ent := el.Value.(*renderCacheEntry)
	if c.ttl > 0 && c.now().Sub(ent.stored) >= c.ttl {
		c.ll.Remove(el)
		delete(c.items, k)
		c.misses.Add(1)
		return nil, false
	}
	c.ll.MoveToFront(el)
	c.hits.Add(1)
	return ent.value, true
}

// store records a pipeline output for the given input, evicting the least
// recently used entry when over capacity. A defensive copy of output is kept.
func (c *renderCache) store(input, output []byte) {
	if c == nil {
		return
	}
	k := c.key(input)
	cp := make([]byte, len(output))
	copy(cp, output)

	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[k]; ok {
		ent := el.Value.(*renderCacheEntry)
		ent.value = cp
		ent.stored = c.now()
		c.ll.MoveToFront(el)
		return
	}
	el := c.ll.PushFront(&renderCacheEntry{key: k, value: cp, stored: c.now()})
	c.items[k] = el
	for c.ll.Len() > c.max {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*renderCacheEntry).key)
	}
}

// stats returns hit/miss counters for logging.
func (c *renderCache) stats() (hits, misses uint64) {
	if c == nil {
		return 0, 0
	}
	return c.hits.Load(), c.misses.Load()
}
