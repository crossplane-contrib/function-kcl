package main

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

func newTestRecycler(cfg recycleConfig) *recycler {
	r := newRecycler(logging.NewNopLogger(), cfg)
	r.exit = func(int) {} // never really exit during tests
	return r
}

func TestNilRecyclerAdmits(t *testing.T) {
	var r *recycler
	if !r.begin() {
		t.Fatal("nil recycler must admit calls")
	}
	r.end() // must not panic
}

func TestBeginRejectsWhenDraining(t *testing.T) {
	r := newTestRecycler(recycleConfig{})
	if !r.begin() {
		t.Fatal("expected admit before draining")
	}
	r.end()

	r.draining.Store(true)
	if r.begin() {
		t.Fatal("expected reject while draining")
	}
}

func TestRecycleDrainsInflightThenExits(t *testing.T) {
	r := newTestRecycler(recycleConfig{drainTimeout: 2 * time.Second})

	exited := make(chan int, 1)
	r.exit = func(code int) { exited <- code }

	// Simulate an in-flight call holding the gate.
	if !r.begin() {
		t.Fatal("expected admit")
	}

	recycleReturned := make(chan struct{})
	go func() {
		r.recycle("test")
		close(recycleReturned)
	}()

	// recycle must not exit while a call is in flight.
	select {
	case <-exited:
		t.Fatal("exited before in-flight call finished")
	case <-time.After(150 * time.Millisecond):
	}

	// New calls are rejected once draining started.
	if r.begin() {
		t.Fatal("expected reject during drain")
	}

	// Finish the in-flight call; drain should now complete and exit cleanly.
	r.end()

	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("recycle did not exit after drain")
	}
	<-recycleReturned
}

func TestRecycleDrainTimeout(t *testing.T) {
	r := newTestRecycler(recycleConfig{drainTimeout: 100 * time.Millisecond})
	exited := make(chan int, 1)
	r.exit = func(code int) { exited <- code }

	// Hold a call in-flight and never release it; recycle must still exit via
	// the drain timeout.
	if !r.begin() {
		t.Fatal("expected admit")
	}
	defer r.end()

	go r.recycle("stuck")

	select {
	case code := <-exited:
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("recycle did not force-exit on drain timeout")
	}
}

func TestRecycleIsIdempotent(t *testing.T) {
	r := newTestRecycler(recycleConfig{drainTimeout: time.Second})
	var exits int
	var mu sync.Mutex
	r.exit = func(int) { mu.Lock(); exits++; mu.Unlock() }

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.recycle("race") }()
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if exits != 1 {
		t.Fatalf("expected exactly one exit, got %d", exits)
	}
}

func TestShouldRecycleTriggers(t *testing.T) {
	cases := []struct {
		name   string
		cfg    recycleConfig
		setup  func(*recycler)
		expect bool
	}{
		{
			name:   "rss over limit",
			cfg:    recycleConfig{maxRSSBytes: 1000},
			setup:  func(r *recycler) { r.readRSS = func() (uint64, error) { return 2000, nil } },
			expect: true,
		},
		{
			name:   "rss under limit",
			cfg:    recycleConfig{maxRSSBytes: 1000},
			setup:  func(r *recycler) { r.readRSS = func() (uint64, error) { return 500, nil } },
			expect: false,
		},
		{
			name:   "reconcile count reached",
			cfg:    recycleConfig{maxReconciles: 3},
			setup:  func(r *recycler) { r.count.Store(3) },
			expect: true,
		},
		{
			name: "lifetime reached",
			cfg:  recycleConfig{maxLifetime: time.Hour},
			setup: func(r *recycler) {
				base := r.start
				r.now = func() time.Time { return base.Add(2 * time.Hour) }
			},
			expect: true,
		},
		{
			name:   "nothing enabled",
			cfg:    recycleConfig{},
			setup:  func(*recycler) {},
			expect: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRecycler(tc.cfg)
			tc.setup(r)
			if got := r.shouldRecycle() != ""; got != tc.expect {
				t.Fatalf("shouldRecycle()=%v, want %v", got, tc.expect)
			}
		})
	}
}

func TestEnabled(t *testing.T) {
	if (recycleConfig{}).enabled() {
		t.Fatal("empty config must be disabled")
	}
	if !(recycleConfig{maxRSSBytes: 1}).enabled() {
		t.Fatal("rss trigger must enable")
	}
	if !(recycleConfig{maxReconciles: 1}).enabled() {
		t.Fatal("reconcile trigger must enable")
	}
	if !(recycleConfig{maxLifetime: time.Second}).enabled() {
		t.Fatal("lifetime trigger must enable")
	}
}

func TestEnvBytes(t *testing.T) {
	cases := map[string]uint64{
		"1024":   1024,
		"1Ki":    1 << 10,
		"6Gi":    6 << 30,
		"512Mi":  512 << 20,
		"bad":    7, // falls back to def
		"  2Gi ": 2 << 30,
	}
	for in, want := range cases {
		t.Setenv("X_TEST_BYTES", in)
		if got := envBytes("X_TEST_BYTES", 7); got != want {
			t.Errorf("envBytes(%q)=%d, want %d", in, got, want)
		}
	}
}

func TestReadCgroupLimitRejectsSentinels(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := dir + "/" + name
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, ok := readCgroupLimit(write("max", "max")); ok {
		t.Error(`"max" must be treated as no limit`)
	}
	if _, ok := readCgroupLimit(write("huge", "9223372036854771712")); ok {
		t.Error("v1 unlimited sentinel must be treated as no limit")
	}
	v, ok := readCgroupLimit(write("real", "8589934592"))
	if !ok || v != 8<<30 {
		t.Errorf("real limit: got %d ok=%v, want %d true", v, ok, uint64(8<<30))
	}
}
