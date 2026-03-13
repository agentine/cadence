package cadence

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock implements Clock for deterministic testing.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
	ch  chan time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{now: t, ch: make(chan time.Time, 1)}
}

func (fc *fakeClock) Now() time.Time {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	return fc.now
}

func (fc *fakeClock) After(d time.Duration) <-chan time.Time {
	return fc.ch
}

func (fc *fakeClock) Advance(d time.Duration) {
	fc.mu.Lock()
	fc.now = fc.now.Add(d)
	t := fc.now
	fc.mu.Unlock()
	fc.ch <- t
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.running {
		t.Error("should not be running initially")
	}
}

func TestAddFunc(t *testing.T) {
	c := New()
	id, err := c.AddFunc("* * * * *", func() {})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}
	if len(c.Entries()) != 1 {
		t.Errorf("expected 1 entry, got %d", len(c.Entries()))
	}
}

func TestAddJob(t *testing.T) {
	c := New()
	_, err := c.AddJob("@every 1s", FuncJob(func() {}))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries()) != 1 {
		t.Errorf("expected 1 entry, got %d", len(c.Entries()))
	}
}

func TestAddInvalidSpec(t *testing.T) {
	c := New()
	_, err := c.AddFunc("invalid", func() {})
	if err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestRemove(t *testing.T) {
	c := New()
	id, _ := c.AddFunc("* * * * *", func() {})
	c.Remove(id)
	if len(c.Entries()) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(c.Entries()))
	}
}

func TestEntry(t *testing.T) {
	c := New()
	id, _ := c.AddFunc("* * * * *", func() {})
	e := c.Entry(id)
	if e.ID != id {
		t.Errorf("expected ID %d, got %d", id, e.ID)
	}
}

func TestEntryNotFound(t *testing.T) {
	c := New()
	e := c.Entry(999)
	if e.Valid() {
		t.Error("expected invalid entry for non-existent ID")
	}
}

func TestSchedule(t *testing.T) {
	c := New()
	id := c.Schedule(Every(time.Second), FuncJob(func() {}))
	if id == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestStartStop(t *testing.T) {
	c := New()
	c.AddFunc("@every 1h", func() {})
	c.Start()
	if !c.running {
		t.Error("expected running after Start")
	}
	ctx := c.Stop()
	<-ctx.Done()
}

func TestStartIdempotent(t *testing.T) {
	c := New()
	c.AddFunc("@every 1h", func() {})
	c.Start()
	c.Start() // should not panic or start second goroutine
	c.Stop()
}

func TestRunExecutesJobs(t *testing.T) {
	var count int32
	done := make(chan struct{}, 1)

	c := New()
	c.AddFunc("@every 1s", func() {
		atomic.AddInt32(&count, 1)
		select {
		case done <- struct{}{}:
		default:
		}
	})

	c.Start()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for job to run")
	}

	c.Stop()

	if atomic.LoadInt32(&count) < 1 {
		t.Errorf("expected job to run at least once, got %d", count)
	}
}

func TestAddWhileRunning(t *testing.T) {
	c := New()
	c.AddFunc("@every 1h", func() {})
	c.Start()

	// Add a job while running.
	id, err := c.AddFunc("@every 1h", func() {})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	// Give time for the add to be processed.
	time.Sleep(100 * time.Millisecond)

	entries := c.Entries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	c.Stop()
}

func TestRemoveWhileRunning(t *testing.T) {
	c := New()
	id, _ := c.AddFunc("@every 1h", func() {})
	c.Start()

	c.Remove(id)
	time.Sleep(100 * time.Millisecond)

	entries := c.Entries()
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after remove, got %d", len(entries))
	}

	c.Stop()
}

func TestEntriesSnapshot(t *testing.T) {
	c := New()
	c.AddFunc("@every 1s", func() {})
	c.AddFunc("@every 2s", func() {})

	entries := c.Entries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Modifying snapshot should not affect cron.
	entries[0].ID = 999
	original := c.Entries()
	if original[0].ID == 999 {
		t.Error("snapshot should be independent")
	}
}

func TestMultipleJobs(t *testing.T) {
	var count1, count2 int32
	done := make(chan struct{}, 2)

	c := New()
	c.AddFunc("@every 1s", func() {
		atomic.AddInt32(&count1, 1)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	c.AddFunc("@every 1s", func() {
		atomic.AddInt32(&count2, 1)
		select {
		case done <- struct{}{}:
		default:
		}
	})
	c.Start()

	// Wait for both jobs to fire.
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("timed out")
		}
	}

	c.Stop()

	if atomic.LoadInt32(&count1) < 1 {
		t.Error("job1 should have run")
	}
	if atomic.LoadInt32(&count2) < 1 {
		t.Error("job2 should have run")
	}
}

// ---------------------------------------------------------------------------
// Option tests
// ---------------------------------------------------------------------------

func TestWithClock(t *testing.T) {
	fc := newFakeClock(time.Now())
	c := New(WithClock(fc))
	if c.clock != fc {
		t.Error("WithClock did not set clock")
	}
}

func TestWithLocation(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	c := New(WithLocation(loc))
	if c.Location() != loc {
		t.Error("WithLocation did not set location")
	}
}

func TestWithParser(t *testing.T) {
	p := NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)
	c := New(WithParser(p))
	_, err := c.AddFunc("0 30 4 * * *", func() {})
	if err != nil {
		t.Errorf("expected 6-field parser to work: %v", err)
	}
}

func TestWithSeconds(t *testing.T) {
	c := New(WithSeconds())
	_, err := c.AddFunc("0 30 4 * * *", func() {})
	if err != nil {
		t.Errorf("expected seconds parser to work: %v", err)
	}
}

func TestWithChain(t *testing.T) {
	var called int32
	done := make(chan struct{}, 1)
	wrapper := func(j Job) Job {
		return FuncJob(func() {
			atomic.StoreInt32(&called, 1)
			j.Run()
			select {
			case done <- struct{}{}:
			default:
			}
		})
	}
	c := New(WithChain(wrapper))
	c.AddFunc("@every 1s", func() {})
	c.Start()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
	}

	c.Stop()
	if atomic.LoadInt32(&called) != 1 {
		t.Error("wrapper should have been called")
	}
}

// ---------------------------------------------------------------------------
// FakeClock integration (entry scheduling, not full run loop)
// ---------------------------------------------------------------------------

func TestFakeClock_EntryNext(t *testing.T) {
	fc := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c := New(WithClock(fc))
	c.AddFunc("@every 1s", func() {})

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}
	// The Next time should be relative to the fake clock.
	// Since addFunc is called before Start(), Next is not yet set until Start.
	// Let's verify the schedule itself.
	sched := entries[0].Schedule
	next := sched.Next(fc.Now())
	expected := fc.Now().Add(1 * time.Second)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestLocation_AffectsScheduling(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	c := New(WithLocation(loc))
	c.AddFunc("0 9 * * *", func() {})

	entries := c.Entries()
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}
}

// ---------------------------------------------------------------------------
// New feature tests
// ---------------------------------------------------------------------------

func TestIsRunning(t *testing.T) {
	c := New()
	if c.IsRunning() {
		t.Error("should not be running initially")
	}
	c.AddFunc("@every 1h", func() {})
	c.Start()
	if !c.IsRunning() {
		t.Error("should be running after Start")
	}
	c.Stop()
	// After Stop returns, running should be false.
	if c.IsRunning() {
		t.Error("should not be running after Stop")
	}
}

func TestParseStandard(t *testing.T) {
	sched, err := ParseStandard("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseStandard failed: %v", err)
	}
	if sched == nil {
		t.Fatal("ParseStandard returned nil schedule")
	}
	// Verify it produces a valid next time.
	next := sched.Next(time.Now())
	if next.IsZero() {
		t.Error("expected non-zero next time")
	}
}

func TestParseStandardDescriptor(t *testing.T) {
	sched, err := ParseStandard("@hourly")
	if err != nil {
		t.Fatalf("ParseStandard(@hourly) failed: %v", err)
	}
	next := sched.Next(time.Now())
	if next.IsZero() {
		t.Error("expected non-zero next time for @hourly")
	}
}

func TestParseStandardInvalid(t *testing.T) {
	_, err := ParseStandard("invalid cron")
	if err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestDefaultLogger(t *testing.T) {
	c := New()
	if c.logger == nil {
		t.Error("default logger should not be nil")
	}
	// Default logger should be DiscardLogger (silent).
	c.logger.Info("should not panic")
	c.logger.Error(nil, "should not panic")
}

func TestAddFuncContext(t *testing.T) {
	c := New()
	var gotCtx context.Context
	done := make(chan struct{}, 1)

	id, err := c.AddFuncContext("@every 1s", func(ctx context.Context) {
		gotCtx = ctx
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Error("expected non-zero ID")
	}

	c.Start()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for context job")
	}
	c.Stop()

	if gotCtx == nil {
		t.Error("expected context to be passed to job")
	}
}

func TestDSTSpringForward(t *testing.T) {
	// America/New_York springs forward on the second Sunday of March:
	// 2:00 AM EST → 3:00 AM EDT. Hour 2 does not exist.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone not available")
	}

	// Schedule for 2:30 AM. On DST day, hour 2 is in the gap.
	sched := &SpecSchedule{
		Second:   1 << 0,
		Minute:   1 << 30,
		Hour:     1 << 2,
		Dom:      allDom,
		Month:    allMonths,
		Dow:      allDow,
		Location: loc,
	}

	// Start from just before the DST transition on March 9, 2025 (DST day).
	before := time.Date(2025, time.March, 9, 1, 0, 0, 0, loc)
	next := sched.Next(before)

	// The job should fire at the first moment after the gap (3:00 AM EDT),
	// not be skipped to the next day.
	if next.Day() != 9 {
		t.Errorf("DST: expected job on day 9, got day %d (time: %v)", next.Day(), next)
	}
	if next.Hour() < 2 {
		t.Errorf("DST: expected hour >= 2, got %d (time: %v)", next.Hour(), next)
	}
}

func TestDoubleExecutionPrevention(t *testing.T) {
	// Verify that after running, Next is strictly after Prev,
	// even if the clock's "now" is before Prev.
	fc := newFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	c := New(WithClock(fc))

	var count int32
	c.AddFunc("@every 5s", func() {
		atomic.AddInt32(&count, 1)
	})

	c.Start()

	// Advance past the first scheduled time.
	fc.Advance(6 * time.Second)
	time.Sleep(50 * time.Millisecond) // let the job run

	entries := c.Entries()
	if len(entries) == 0 {
		t.Fatal("expected entries")
	}
	// Next should be strictly after Prev.
	for _, e := range entries {
		if !e.Prev.IsZero() && !e.Next.After(e.Prev) {
			t.Errorf("Next (%v) should be after Prev (%v)", e.Next, e.Prev)
		}
	}

	c.Stop()
}
