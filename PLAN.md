# cadence — Drop-in Replacement for robfig/cron

## Overview

**Target:** [robfig/cron](https://github.com/robfig/cron) v3 — the de facto Go cron scheduler
**Module:** `github.com/agentine/cadence`
**License:** MIT
**Go:** 1.21+
**Dependencies:** Zero

## Why Replace robfig/cron

- Last release: v3.0.1, January 2020 (6+ years ago)
- Last commit: January 2021 (5+ years ago)
- 14,083 stars, 5,452 importers on pkg.go.dev
- 166 open issues, 30+ unmerged PRs
- Known critical panics: TZ parsing (#554, #555), malformed cron expressions (#543)
- Known bugs: SkipIfStillRunning bypass (#551), double execution (#553), DST skips (#541)
- Sunday=7 not accepted (#522), violating standard cron convention
- No context.Context support for graceful job shutdown
- No testable clock interface
- No jitter/hash-based scheduling
- Single maintainer, no activity

## Architecture

cadence is a single-package Go library with three layers:

```
User API (Cron, Options, AddFunc/AddJob)
    ↓
Parser (cron expression → Schedule)
    ↓
Scheduler (goroutine-based run loop, entry management)
    ↓
Middleware Chain (JobWrapper pipeline)
    ↓
Job Execution
```

All scheduling is driven by the `Schedule.Next(time.Time) time.Time` interface — the scheduler sleeps until the nearest `Next` time, then executes matching entries.

## Public API Surface (100% robfig/cron v3 compatible)

### Interfaces

```go
type Job interface {
    Run()
}

type Schedule interface {
    Next(time.Time) time.Time
}

type ScheduleParser interface {
    Parse(spec string) (Schedule, error)
}

type Logger interface {
    Info(msg string, keysAndValues ...interface{})
    Error(err error, msg string, keysAndValues ...interface{})
}
```

### Core Types

```go
type Cron struct { /* unexported fields */ }

type Entry struct {
    ID         EntryID
    Schedule   Schedule
    Next       time.Time
    Prev       time.Time
    WrappedJob Job
    Job        Job
}

type EntryID int
type FuncJob func()
type Option func(*Cron)
type JobWrapper func(Job) Job
type ParseOption int
```

### Cron Methods

```go
func New(opts ...Option) *Cron
func (c *Cron) AddFunc(spec string, cmd func()) (EntryID, error)
func (c *Cron) AddJob(spec string, cmd Job) (EntryID, error)
func (c *Cron) Schedule(schedule Schedule, cmd Job) EntryID
func (c *Cron) Entries() []Entry
func (c *Cron) Entry(id EntryID) Entry
func (c *Cron) Location() *time.Location
func (c *Cron) Remove(id EntryID)
func (c *Cron) Start()
func (c *Cron) Run()
func (c *Cron) Stop() context.Context
```

### Options

```go
func WithLocation(loc *time.Location) Option
func WithParser(p ScheduleParser) Option
func WithSeconds() Option
func WithChain(wrappers ...JobWrapper) Option
func WithLogger(logger Logger) Option
```

### Parser

```go
const (
    Second         ParseOption = 1 << iota  // 1
    SecondOptional                           // 2
    Minute                                   // 4
    Hour                                     // 8
    Dom                                      // 16
    Month                                    // 32
    Dow                                      // 64
    DowOptional                              // 128
    Descriptor                               // 256
)

func NewParser(options ParseOption) Parser
func (p Parser) Parse(spec string) (Schedule, error)
func ParseStandard(standardSpec string) (Schedule, error)
```

### Schedule Implementations

```go
type SpecSchedule struct {
    Second, Minute, Hour, Dom, Month, Dow uint64
    Location *time.Location
}
func (s *SpecSchedule) Next(t time.Time) time.Time

type ConstantDelaySchedule struct {
    Delay time.Duration
}
func (s ConstantDelaySchedule) Next(t time.Time) time.Time
```

### Chain / Middleware

```go
func NewChain(wrappers ...JobWrapper) Chain
func (c Chain) Then(j Job) Job
func Recover(logger Logger) JobWrapper
func DelayIfStillRunning(logger Logger) JobWrapper
func SkipIfStillRunning(logger Logger) JobWrapper
```

### Logger Helpers

```go
func PrintfLogger(l interface{ Printf(string, ...interface{}) }) Logger
func VerbosePrintfLogger(l interface{ Printf(string, ...interface{}) }) Logger
var DefaultLogger Logger
var DiscardLogger Logger
```

### Predefined Descriptors

| Descriptor | Equivalent |
|-----------|------------|
| `@yearly` / `@annually` | `0 0 1 1 *` |
| `@monthly` | `0 0 1 * *` |
| `@weekly` | `0 0 * * 0` |
| `@daily` / `@midnight` | `0 0 * * *` |
| `@hourly` | `0 * * * *` |
| `@every <duration>` | `ConstantDelaySchedule` |

### Cron Expression Format

```
┌──────── second (0-59, optional)
│ ┌────── minute (0-59)
│ │ ┌──── hour (0-23)
│ │ │ ┌── day of month (1-31)
│ │ │ │ ┌ month (1-12 or JAN-DEC)
│ │ │ │ │ ┌ day of week (0-6 or SUN-SAT, 7=SUN)
│ │ │ │ │ │
* * * * * *
```

Special characters: `*` (all), `/` (step), `,` (list), `-` (range), `?` (any, DOM/DOW only)
Timezone prefix: `CRON_TZ=America/New_York` or `TZ=...`

DOM/DOW interaction:
- If either is `*`, use AND logic
- If both are explicit, use OR logic (standard cron behavior)

## Bug Fixes Over robfig/cron

1. **TZ panic** — validate timezone string before `time.LoadLocation`, return error instead of panic
2. **Missing fields panic** — validate field count after TZ extraction before parsing
3. **Invalid step values** — reject `*/0` and steps > field range
4. **SkipIfStillRunning bypass** — use `WrappedJob` consistently in all execution paths
5. **DST spring-forward** — execute skipped jobs immediately after the gap (configurable)
6. **Double execution** — use monotonic clock comparisons to prevent re-triggering
7. **Sunday=7** — accept 7 as alias for 0 (Sunday) per POSIX cron convention
8. **System clock jumps** — detect and compensate for NTP/manual clock changes

## New Features (backward-compatible extensions)

1. **`WithClock(clock Clock) Option`** — injectable clock interface for deterministic testing
2. **`WithContext(ctx context.Context) Option`** — parent context for graceful shutdown
3. **`AddFuncContext(spec string, cmd func(context.Context)) (EntryID, error)`** — context-aware job function
4. **`WithJitter(max time.Duration) Option`** — random jitter added to each execution time
5. **`Entry.Valid() bool`** — check if entry is still active
6. **`IsRunning() bool`** — check scheduler state

## Implementation Phases

### Phase 1: Parser & Schedule Engine
- Cron expression parser with all ParseOption flags
- SpecSchedule with bitset-based Next() calculation
- ConstantDelaySchedule
- Predefined descriptors (@yearly, @every, etc.)
- Timezone prefix parsing with safe validation
- DOM/DOW interaction logic (AND/OR)
- Comprehensive parser tests including all known edge cases

### Phase 2: Scheduler & Entry Management
- Cron struct with goroutine-based run loop
- AddFunc/AddJob/Schedule/Remove/Entries/Entry
- Start/Stop/Run lifecycle management
- Channel-based entry management (add/remove/snapshot)
- Stop() returning context.Context for job completion
- Clock interface for testability

### Phase 3: Middleware & Options
- Chain type with Then() composition
- Recover, DelayIfStillRunning, SkipIfStillRunning middleware
- All Option functions (WithLocation, WithParser, WithSeconds, WithChain, WithLogger)
- Logger interface with PrintfLogger/VerbosePrintfLogger
- New options: WithClock, WithContext, WithJitter

### Phase 4: Polish & Ship
- Full test suite (robfig/cron tests ported + new tests for bug fixes)
- Benchmarks comparing to robfig/cron
- Migration guide (drop-in import path change)
- pkg.go.dev documentation
- CI/CD pipeline
- Example programs
