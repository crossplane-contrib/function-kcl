package main

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

// The KCL native runtime accumulates off-heap memory across reconciles (it
// recompiles the whole KCL module on every call and the native service handle
// is a long-lived process-global singleton that is never reset). Left alone,
// function-kcl pods climb to their memory limit and get OOMKilled (exit 137),
// which violently drops in-flight reconciles -> "connection reset by peer" /
// DeadlineExceeded on heavy composites.
//
// The recycler turns those hard OOMKills into clean, scheduled restarts. A
// watchdog samples the process RSS (which includes the native off-heap
// allocations) plus optional reconcile-count / lifetime limits. When a limit is
// crossed it drains in-flight RunFunction calls and exits 0 so Kubernetes
// restarts the pod *between* reconciles instead of *during* one. With multiple
// replicas this is a rolling, self-healing recycle and removes the need for a
// manual rollout restart.
//
// This is a stability backstop; the render cache (see rendercache.go) avoids the
// recompile — and its leak increment — for byte-identical reconciles. The two
// are complementary: the backstop bounds any residual growth regardless of the
// leak's exact internals.

// recycleConfig is read from the environment so it can be tuned per deployment
// via container env vars, without a rebuild.
type recycleConfig struct {
	// maxRSSBytes recycles the process once its resident set size reaches this
	// many bytes. 0 disables the RSS trigger.
	maxRSSBytes uint64
	// maxRSSRatio is used to derive maxRSSBytes from the detected cgroup memory
	// limit when maxRSSBytes is not set explicitly.
	maxRSSRatio float64
	// maxReconciles recycles after this many RunFunction calls. 0 disables.
	maxReconciles uint64
	// maxLifetime recycles after the process has been up this long. 0 disables.
	maxLifetime time.Duration
	// checkInterval is how often the watchdog samples the triggers.
	checkInterval time.Duration
	// drainTimeout bounds how long we wait for in-flight calls to finish before
	// exiting anyway.
	drainTimeout time.Duration
}

const (
	envMaxRSSBytes   = "FUNCTION_KCL_MAX_RSS_BYTES"
	envMaxRSSRatio   = "FUNCTION_KCL_MAX_RSS_RATIO"
	envMaxReconciles = "FUNCTION_KCL_MAX_RECONCILES"
	envMaxLifetime   = "FUNCTION_KCL_MAX_LIFETIME"
	envCheckInterval = "FUNCTION_KCL_RECYCLE_CHECK_INTERVAL"
	envDrainTimeout  = "FUNCTION_KCL_RECYCLE_DRAIN_TIMEOUT"

	defaultMaxRSSRatio   = 0.85
	defaultCheckInterval = 30 * time.Second
	defaultDrainTimeout  = 15 * time.Second
)

// recycleConfigFromEnv builds the config from environment variables, applying
// defaults. If no explicit RSS limit is set, it derives one from the cgroup
// memory limit so the in-cluster default "just works" (recycle at 85% of the
// pod's memory limit, before the kubelet OOMKills at 100%).
func recycleConfigFromEnv() recycleConfig {
	cfg := recycleConfig{
		maxRSSRatio:   envFloat(envMaxRSSRatio, defaultMaxRSSRatio),
		maxRSSBytes:   envBytes(envMaxRSSBytes, 0),
		maxReconciles: envUint(envMaxReconciles, 0),
		maxLifetime:   envDuration(envMaxLifetime, 0),
		checkInterval: envDuration(envCheckInterval, defaultCheckInterval),
		drainTimeout:  envDuration(envDrainTimeout, defaultDrainTimeout),
	}
	if cfg.maxRSSBytes == 0 && cfg.maxRSSRatio > 0 {
		if limit, ok := cgroupMemoryLimit(); ok {
			cfg.maxRSSBytes = uint64(float64(limit) * cfg.maxRSSRatio)
		}
	}
	return cfg
}

// enabled reports whether any recycle trigger is active. When false the
// watchdog never fires, so behaviour is identical to upstream (useful for local
// runs and tests with no cgroup limit).
func (c recycleConfig) enabled() bool {
	return c.maxRSSBytes > 0 || c.maxReconciles > 0 || c.maxLifetime > 0
}

// recycler tracks in-flight RunFunction calls and drives the watchdog. The zero
// value and a nil *recycler are both safe no-ops, so existing tests that build a
// Function without one are unaffected.
type recycler struct {
	log   logging.Logger
	cfg   recycleConfig
	start time.Time

	// gate is held for reading for the duration of each RunFunction call.
	// Draining acquires it for writing, which blocks until all in-flight calls
	// release it and prevents new ones from starting.
	gate     sync.RWMutex
	inflight atomic.Int64
	count    atomic.Uint64
	draining atomic.Bool

	// Injectable for tests.
	readRSS func() (uint64, error)
	exit    func(int)
	now     func() time.Time
}

func newRecycler(log logging.Logger, cfg recycleConfig) *recycler {
	return &recycler{
		log:     log,
		cfg:     cfg,
		start:   time.Now(),
		readRSS: processRSS,
		exit:    os.Exit,
		now:     time.Now,
	}
}

// begin marks the start of a RunFunction call. It returns false if the process
// is draining, in which case the caller must reject the request so Crossplane
// retries it (against another replica or after the restart). A nil recycler
// always admits the call.
func (r *recycler) begin() bool {
	if r == nil {
		return true
	}
	if r.draining.Load() {
		return false
	}
	r.gate.RLock()
	r.inflight.Add(1)
	r.count.Add(1)
	return true
}

// end marks the completion of a RunFunction call.
func (r *recycler) end() {
	if r == nil {
		return
	}
	r.inflight.Add(-1)
	r.gate.RUnlock()
}

// start launches the watchdog goroutine if any trigger is enabled.
func (r *recycler) run() {
	if r == nil || !r.cfg.enabled() {
		return
	}
	r.log.Info("memory recycler enabled",
		"maxRSSBytes", r.cfg.maxRSSBytes,
		"maxReconciles", r.cfg.maxReconciles,
		"maxLifetime", r.cfg.maxLifetime.String(),
		"checkInterval", r.cfg.checkInterval.String())
	go r.watch()
}

func (r *recycler) watch() {
	ticker := time.NewTicker(r.cfg.checkInterval)
	defer ticker.Stop()
	for range ticker.C {
		if reason := r.shouldRecycle(); reason != "" {
			r.recycle(reason)
			return
		}
	}
}

// shouldRecycle returns a non-empty human-readable reason if any trigger has
// fired, else "".
func (r *recycler) shouldRecycle() string {
	if r.cfg.maxReconciles > 0 && r.count.Load() >= r.cfg.maxReconciles {
		return "reconcile count " + strconv.FormatUint(r.count.Load(), 10) +
			" >= " + strconv.FormatUint(r.cfg.maxReconciles, 10)
	}
	if r.cfg.maxLifetime > 0 && r.now().Sub(r.start) >= r.cfg.maxLifetime {
		return "lifetime " + r.now().Sub(r.start).Round(time.Second).String() +
			" >= " + r.cfg.maxLifetime.String()
	}
	if r.cfg.maxRSSBytes > 0 {
		rss, err := r.readRSS()
		if err != nil {
			r.log.Debug("cannot read process RSS", "error", err)
			return ""
		}
		if rss >= r.cfg.maxRSSBytes {
			return "RSS " + strconv.FormatUint(rss, 10) +
				" >= " + strconv.FormatUint(r.cfg.maxRSSBytes, 10) + " bytes"
		}
	}
	return ""
}

// recycle drains in-flight calls (bounded by drainTimeout) and exits cleanly so
// the orchestrator restarts the pod.
func (r *recycler) recycle(reason string) {
	if !r.draining.CompareAndSwap(false, true) {
		return
	}
	r.log.Info("recycling function-kcl to release native memory",
		"reason", reason, "inflight", r.inflight.Load())

	drained := make(chan struct{})
	go func() {
		r.gate.Lock() // waits for all in-flight RunFunction calls to return
		close(drained)
	}()

	select {
	case <-drained:
		r.log.Info("drained in-flight reconciles, exiting for clean restart")
	case <-time.After(r.cfg.drainTimeout):
		r.log.Info("drain timed out, forcing restart",
			"inflight", r.inflight.Load(), "timeout", r.cfg.drainTimeout.String())
	}
	r.exit(0)
}

// processRSS returns the resident set size of the current process in bytes by
// reading /proc/self/statm. RSS includes the KCL native (off-heap) allocations,
// which is exactly what we need to bound.
func processRSS() (uint64, error) {
	data, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, errInvalidStatm
	}
	residentPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return residentPages * uint64(os.Getpagesize()), nil
}

// cgroupMemoryLimit returns the container memory limit in bytes, trying cgroup
// v2 then v1. ok is false if no finite limit is set.
func cgroupMemoryLimit() (uint64, bool) {
	// cgroup v2
	if v, ok := readCgroupLimit("/sys/fs/cgroup/memory.max"); ok {
		return v, true
	}
	// cgroup v1
	if v, ok := readCgroupLimit("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok {
		return v, true
	}
	return 0, false
}

func readCgroupLimit(path string) (uint64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(data))
	if s == "" || s == "max" {
		return 0, false
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	// cgroup v1 reports a sentinel "unlimited" value close to max uint64; treat
	// anything implausibly large (>= 1 PiB) as no limit.
	if v == 0 || v >= 1<<50 {
		return 0, false
	}
	return v, true
}

var errInvalidStatm = &recycleError{"unexpected /proc/self/statm format"}

type recycleError struct{ msg string }

func (e *recycleError) Error() string { return e.msg }

// envUint, envFloat, envBytes and envDuration parse optional environment
// variables, falling back to def on absence or parse error.
func envUint(key string, def uint64) uint64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return def
	}
	return n
}

func envFloat(key string, def float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return def
	}
	return f
}

func envDuration(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return d
}

// envBytes parses a byte size that may carry a binary unit suffix (Ki, Mi, Gi)
// or be a plain integer count of bytes.
func envBytes(key string, def uint64) uint64 {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	s := strings.TrimSpace(v)
	mult := uint64(1)
	switch {
	case strings.HasSuffix(s, "Gi"):
		mult, s = 1<<30, strings.TrimSuffix(s, "Gi")
	case strings.HasSuffix(s, "Mi"):
		mult, s = 1<<20, strings.TrimSuffix(s, "Mi")
	case strings.HasSuffix(s, "Ki"):
		mult, s = 1<<10, strings.TrimSuffix(s, "Ki")
	}
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return def
	}
	return n * mult
}
