package cadence

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Test logger that captures output
// ---------------------------------------------------------------------------

type testLogger struct {
	mu     sync.Mutex
	infos  []string
	errors []string
}

func (l *testLogger) Info(msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}

func (l *testLogger) Error(err error, msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, fmt.Sprintf("%s: %v", msg, err))
}

func (l *testLogger) errorCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errors)
}

func (l *testLogger) infoCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.infos)
}

// ---------------------------------------------------------------------------
// Recover middleware tests
// ---------------------------------------------------------------------------

func TestRecover_NoPanic(t *testing.T) {
	logger := &testLogger{}
	var called int32
	job := Recover(logger)(FuncJob(func() {
		atomic.StoreInt32(&called, 1)
	}))
	job.Run()
	if atomic.LoadInt32(&called) != 1 {
		t.Error("job should have run")
	}
	if logger.errorCount() != 0 {
		t.Error("should not have logged error")
	}
}

func TestRecover_Panic(t *testing.T) {
	logger := &testLogger{}
	job := Recover(logger)(FuncJob(func() {
		panic("test panic")
	}))
	// Should not panic.
	job.Run()
	if logger.errorCount() != 1 {
		t.Errorf("expected 1 error, got %d", logger.errorCount())
	}
}

// ---------------------------------------------------------------------------
// SkipIfStillRunning middleware tests
// ---------------------------------------------------------------------------

func TestSkipIfStillRunning(t *testing.T) {
	logger := &testLogger{}
	var running int32
	started := make(chan struct{})
	done := make(chan struct{})

	job := SkipIfStillRunning(logger)(FuncJob(func() {
		atomic.AddInt32(&running, 1)
		started <- struct{}{}
		<-done
		atomic.AddInt32(&running, -1)
	}))

	// Start first run in background.
	go job.Run()
	<-started

	// Second run should be skipped.
	job.Run()
	if logger.infoCount() != 1 {
		t.Errorf("expected 1 skip info, got %d", logger.infoCount())
	}

	// Release first run.
	done <- struct{}{}
}

// ---------------------------------------------------------------------------
// DelayIfStillRunning middleware tests
// ---------------------------------------------------------------------------

func TestDelayIfStillRunning(t *testing.T) {
	logger := &testLogger{}
	var order []int
	var mu sync.Mutex
	gate := make(chan struct{})

	job := DelayIfStillRunning(logger)(FuncJob(func() {
		mu.Lock()
		order = append(order, len(order)+1)
		mu.Unlock()
		select {
		case <-gate:
		case <-time.After(time.Second):
		}
	}))

	// Start first run.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		job.Run()
	}()
	// Give first run time to start.
	time.Sleep(50 * time.Millisecond)

	// Start second run — should block until first completes.
	go func() {
		defer wg.Done()
		job.Run()
	}()

	// Release first run.
	gate <- struct{}{}
	time.Sleep(50 * time.Millisecond)
	// Release second run.
	gate <- struct{}{}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("expected sequential [1, 2], got %v", order)
	}
}

// ---------------------------------------------------------------------------
// Chain tests
// ---------------------------------------------------------------------------

func TestChain_Multiple(t *testing.T) {
	var calls []string
	wrapper1 := func(j Job) Job {
		return FuncJob(func() {
			calls = append(calls, "before1")
			j.Run()
			calls = append(calls, "after1")
		})
	}
	wrapper2 := func(j Job) Job {
		return FuncJob(func() {
			calls = append(calls, "before2")
			j.Run()
			calls = append(calls, "after2")
		})
	}

	chain := NewChain(wrapper1, wrapper2)
	job := chain.Then(FuncJob(func() {
		calls = append(calls, "job")
	}))
	job.Run()

	expected := []string{"before1", "before2", "job", "after2", "after1"}
	if len(calls) != len(expected) {
		t.Fatalf("got %v, want %v", calls, expected)
	}
	for i := range expected {
		if calls[i] != expected[i] {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], expected[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Logger tests
// ---------------------------------------------------------------------------

func TestPrintfLogger(t *testing.T) {
	var msgs []string
	l := &fakeStdLogger{msgs: &msgs}
	logger := PrintfLogger(l)
	logger.Info("should be silent")
	if len(msgs) != 0 {
		t.Error("PrintfLogger should not log Info")
	}
	logger.Error(fmt.Errorf("boom"), "test error")
	if len(msgs) != 1 {
		t.Errorf("expected 1 error log, got %d", len(msgs))
	}
}

func TestVerbosePrintfLogger(t *testing.T) {
	var msgs []string
	l := &fakeStdLogger{msgs: &msgs}
	logger := VerbosePrintfLogger(l)
	logger.Info("hello", "key", "value")
	if len(msgs) != 1 {
		t.Errorf("expected 1 info log, got %d", len(msgs))
	}
	logger.Error(fmt.Errorf("boom"), "test error")
	if len(msgs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(msgs))
	}
}

func TestDiscardLogger(t *testing.T) {
	// Should not panic.
	DiscardLogger.Info("hello")
	DiscardLogger.Error(fmt.Errorf("boom"), "error")
}

type fakeStdLogger struct {
	msgs *[]string
}

func (f *fakeStdLogger) Printf(format string, args ...interface{}) {
	*f.msgs = append(*f.msgs, fmt.Sprintf(format, args...))
}

// ---------------------------------------------------------------------------
// Option tests
// ---------------------------------------------------------------------------

func TestWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := New(WithContext(ctx))
	c.AddFunc("@every 1h", func() {})
	c.Start()

	// Cancel the parent context.
	cancel()
	// The cron's internal context should also be cancelled.
	select {
	case <-c.ctx.Done():
		// expected
	case <-time.After(time.Second):
		t.Error("expected cron context to be cancelled")
	}
	c.Stop()
}

func TestWithJitter(t *testing.T) {
	c := New(WithJitter(100 * time.Millisecond))
	if c.jitter != 100*time.Millisecond {
		t.Errorf("jitter = %v", c.jitter)
	}
}

func TestApplyJitter_Zero(t *testing.T) {
	c := New()
	now := time.Now()
	result := c.applyJitter(now)
	if !result.Equal(now) {
		t.Error("zero jitter should not modify time")
	}
}

func TestApplyJitter_NonZero(t *testing.T) {
	c := New(WithJitter(time.Second))
	now := time.Now()
	result := c.applyJitter(now)
	diff := result.Sub(now)
	if diff < 0 || diff > time.Second {
		t.Errorf("jitter out of range: %v", diff)
	}
}

func TestWithLogger(t *testing.T) {
	logger := &testLogger{}
	c := New(WithLogger(logger))
	if c.logger != logger {
		t.Error("logger not set")
	}
}
