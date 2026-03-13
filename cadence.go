package cadence

import (
	"context"
	"sync"
	"time"
)

// Job is the interface that wraps the Run method.
// Any type implementing Run can be scheduled.
type Job interface {
	Run()
}

// FuncJob is a function adapter for the Job interface.
type FuncJob func()

// Run calls the underlying function.
func (f FuncJob) Run() { f() }

// ContextualJob is an optional interface for jobs that accept a context.
// If a job implements ContextualJob, the scheduler calls RunContext with
// its context instead of Run.
type ContextualJob interface {
	RunContext(context.Context)
}

// contextFuncJob adapts a context-aware function to both Job and ContextualJob.
type contextFuncJob struct {
	fn func(context.Context)
}

func (j *contextFuncJob) Run()                          { j.fn(context.Background()) }
func (j *contextFuncJob) RunContext(ctx context.Context) { j.fn(ctx) }

// Schedule describes a recurring time schedule.
type Schedule interface {
	// Next returns the next activation time after the given time.
	// Next is invoked initially and after each run.
	Next(time.Time) time.Time
}

// ScheduleParser is an interface for cron spec parsers.
type ScheduleParser interface {
	Parse(spec string) (Schedule, error)
}

// Logger is the interface for structured logging used by middleware
// and the scheduler.
type Logger interface {
	// Info logs an informational message with optional key-value pairs.
	Info(msg string, keysAndValues ...interface{})
	// Error logs an error message with optional key-value pairs.
	Error(err error, msg string, keysAndValues ...interface{})
}

// EntryID identifies a scheduled entry. Zero is never used as a valid ID.
type EntryID int

// Entry represents a scheduled job in the cron table.
type Entry struct {
	// ID is the unique identifier assigned when the entry is added.
	ID EntryID

	// Schedule determines when the entry should run.
	Schedule Schedule

	// Next is the time the entry will next run, or the zero time if unset.
	Next time.Time

	// Prev is the time the entry last ran, or the zero time if never.
	Prev time.Time

	// WrappedJob is the job after middleware has been applied.
	WrappedJob Job

	// Job is the original, unwrapped job as submitted by the caller.
	Job Job
}

// Valid reports whether the entry is live (has a non-zero ID).
func (e Entry) Valid() bool {
	return e.ID != 0
}

// Clock is an interface for time operations, allowing the scheduler
// to be tested deterministically.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// realClock implements Clock using the standard time package.
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time  { return time.After(d) }

// Option configures a Cron instance.
type Option func(*Cron)

// JobWrapper decorates a Job with additional behaviour (middleware).
type JobWrapper func(Job) Job

// ParseOption customises how the Parser interprets cron specs.
type ParseOption int

const (
	Second         ParseOption = 1 << iota // Seconds field, required
	SecondOptional                         // Seconds field, optional
	Minute                                 // Minutes field, required
	Hour                                   // Hours field, required
	Dom                                    // Day-of-month field, required
	Month                                  // Month field, required
	Dow                                    // Day-of-week field, required
	DowOptional                            // Day-of-week field, optional
	Descriptor                             // Support @yearly, @every, etc.
)

// Chain is an ordered list of JobWrappers that decorates submitted jobs.
type Chain struct {
	wrappers []JobWrapper
}

// NewChain creates a Chain from the given wrappers.
func NewChain(wrappers ...JobWrapper) Chain {
	return Chain{wrappers: wrappers}
}

// Then wraps the given Job with all wrappers in the chain.
func (c Chain) Then(j Job) Job {
	for i := len(c.wrappers) - 1; i >= 0; i-- {
		j = c.wrappers[i](j)
	}
	return j
}

// Cron manages a set of entries, running each at the time specified by
// its schedule.
type Cron struct {
	entries   []*Entry
	parser    ScheduleParser
	nextID    EntryID
	location  *time.Location
	clock     Clock
	running   bool
	mu        sync.Mutex // protects nextID, running, entries (when not running)
	stop      chan struct{}
	add       chan *Entry
	remove    chan EntryID
	snapshot  chan chan []Entry
	logger    Logger
	chain     Chain
	jitter    time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
}
