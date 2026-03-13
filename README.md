# cadence

A zero-dependency Go cron scheduler — a drop-in replacement for
[`github.com/robfig/cron/v3`](https://github.com/robfig/cron).

Requires Go 1.21+. Zero external dependencies.

---

## Why cadence

`robfig/cron` v3 has not had a release since January 2020 and its last commit
was January 2021. In that time 166 issues and 30+ pull requests have
accumulated, including several critical bugs that affect production workloads:

| Bug | Impact |
|-----|--------|
| TZ prefix panics (#554, #555) | Cron panics instead of returning an error for unknown time zones |
| Sunday=7 rejected (#522) | POSIX cron convention `0 0 * * 7` fails to parse |
| SkipIfStillRunning bypass (#551) | Concurrent invocations slip through the guard |
| Double execution (#553) | Backward clock jumps re-fire jobs |
| DST spring-forward (#541) | Jobs scheduled in a DST gap loop forever |
| Missing-field panic (#543) | Malformed expressions panic instead of erroring |

cadence fixes all of the above, adds a testable `Clock` interface, optional
`context.Context` propagation, per-scheduler jitter, and `AddFuncContext` for
cooperative cancellation — while keeping the public API 100% compatible with
robfig/cron v3.

---

## Installation

```sh
go get github.com/agentine/cadence
```

---

## Quick start

```go
package main

import (
    "fmt"
    "github.com/agentine/cadence"
)

func main() {
    c := cadence.New()

    c.AddFunc("@every 1m", func() {
        fmt.Println("runs every minute")
    })

    c.AddFunc("0 9 * * *", func() {
        fmt.Println("runs at 09:00 every day")
    })

    c.Start()
    // ... application runs ...
    ctx := c.Stop()
    <-ctx.Done() // wait for running jobs to finish
}
```

---

## Cron expression syntax

### Five-field format (default)

```
┌───── minute       (0–59)
│ ┌─── hour         (0–23)
│ │ ┌─ day-of-month (1–31)
│ │ │ ┌ month       (1–12 or JAN–DEC)
│ │ │ │ ┌ day-of-week (0–6 or SUN–SAT; 0 and 7 both mean Sunday)
│ │ │ │ │
* * * * *
```

### Six-field format (with seconds)

Enable with `WithSeconds()`. The seconds field is prepended:

```
┌──────── second      (0–59)
│ ┌────── minute      (0–59)
│ │ ┌──── hour        (0–23)
│ │ │ ┌── day-of-month (1–31)
│ │ │ │ ┌ month        (1–12 or JAN–DEC)
│ │ │ │ │ ┌ day-of-week (0–6 or SUN–SAT; 0 and 7 both mean Sunday)
│ │ │ │ │ │
* * * * * *
```

### Field syntax

| Syntax | Meaning | Example |
|--------|---------|---------|
| `*` | all values | `*` in hour = every hour |
| `?` | any value (DOM/DOW alias for `*`) | `?` |
| `N` | exact value | `5` |
| `N-M` | inclusive range | `9-17` |
| `*/N` | step across all values | `*/5` in minute = 0,5,10,... |
| `N-M/S` | step within range | `0-30/10` = 0,10,20,30 |
| `a,b,c` | comma-separated list | `1,15` in DOM = 1st and 15th |

### Month names

`JAN` `FEB` `MAR` `APR` `MAY` `JUN` `JUL` `AUG` `SEP` `OCT` `NOV` `DEC`
(case-insensitive, three-letter abbreviations)

### Weekday names

`SUN` `MON` `TUE` `WED` `THU` `FRI` `SAT`
(case-insensitive, three-letter abbreviations)

Both `0` and `7` are accepted as Sunday, matching POSIX cron convention.

### DOM/DOW interaction

- If either DOM or DOW is unrestricted (`*`), **AND** logic is used.
- If both DOM and DOW are explicitly set, **OR** logic is used — the job
  fires on any day matching *either* constraint (standard cron behaviour).

### Timezone prefix

A per-schedule timezone can be embedded at the start of the spec string.
Both `TZ=` and `CRON_TZ=` are accepted:

```
TZ=America/New_York 0 9 * * *
CRON_TZ=Europe/London 30 8 * * MON-FRI
```

An unrecognised timezone name returns an error instead of panicking (unlike
robfig/cron).

---

## Predefined descriptors

| Descriptor | Equivalent | Description |
|-----------|-----------|-------------|
| `@yearly` / `@annually` | `0 0 1 1 *` | Once a year, January 1st at midnight |
| `@monthly` | `0 0 1 * *` | Once a month, first day at midnight |
| `@weekly` | `0 0 * * 0` | Once a week, Sunday at midnight |
| `@daily` / `@midnight` | `0 0 * * *` | Once a day at midnight |
| `@hourly` | `0 * * * *` | Once an hour at minute 0 |
| `@every <duration>` | — | Every fixed interval (e.g. `@every 30s`, `@every 1h30m`) |

---

## API reference

### Creating a scheduler

```go
c := cadence.New(opts...)
```

### Adding jobs

```go
// Parse a cron spec and register a function.
id, err := c.AddFunc("* * * * *", func() { /* ... */ })

// Register any Job implementation.
id, err := c.AddJob("@every 5m", myJob)

// Use a pre-built Schedule directly (no parsing, no error).
id := c.Schedule(cadence.Every(5*time.Minute), myJob)

// Register a context-aware function (receives the scheduler's context).
id, err := c.AddFuncContext("0 * * * *", func(ctx context.Context) {
    select {
    case <-ctx.Done():
        return // scheduler is stopping
    default:
        doWork()
    }
})
```

### Lifecycle

```go
c.Start()              // start in a background goroutine (non-blocking)
c.Run()                // start in the foreground (blocking)
ctx := c.Stop()        // signal shutdown; ctx is done when all jobs finish
<-ctx.Done()
running := c.IsRunning() // true if the scheduler goroutine is active
```

### Inspecting entries

```go
entries := c.Entries()       // []Entry snapshot, sorted by next fire time
entry   := c.Entry(id)       // single Entry; Entry.Valid() == false if not found
c.Remove(id)                 // remove by ID (safe while running)
loc     := c.Location()      // configured *time.Location
```

### Entry fields

```go
type Entry struct {
    ID         EntryID   // unique identifier
    Schedule   Schedule  // the underlying schedule
    Next       time.Time // next fire time
    Prev       time.Time // last fire time (zero if never run)
    WrappedJob Job       // job after middleware has been applied
    Job        Job       // original job as submitted
}

func (e Entry) Valid() bool // true if entry has a non-zero ID
```

---

## Schedule interface and Every

Any type implementing `Schedule` can be used directly with `c.Schedule(...)`:

```go
type Schedule interface {
    Next(time.Time) time.Time
}
```

`Every` creates a `ConstantDelaySchedule` — each invocation fires a fixed
duration after the previous one completes:

```go
id := c.Schedule(cadence.Every(10*time.Second), myJob)
```

Durations shorter than one second are rounded up to one second.

---

## Options

| Option | Description |
|--------|-------------|
| `WithLocation(loc *time.Location)` | Set the scheduler's time zone (default: `time.Local`) |
| `WithSeconds()` | Enable 6-field cron format; seconds field is first |
| `WithParser(p ScheduleParser)` | Use a custom `ScheduleParser` |
| `WithChain(wrappers ...JobWrapper)` | Apply middleware to every job added to this scheduler |
| `WithLogger(logger Logger)` | Set the `Logger` used by middleware and the scheduler |
| `WithClock(clock Clock)` | Replace the real clock (useful for deterministic tests) |
| `WithContext(ctx context.Context)` | Parent context; cancelling it signals shutdown |
| `WithJitter(d time.Duration)` | Add random jitter in `[0, d)` to each job's next fire time |

---

## Middleware

Middleware wraps jobs to add cross-cutting behaviour. Apply it per-job via
`Chain` or globally to every job via `WithChain`.

### Built-in wrappers

```go
// Recover from panics and log the panic value and stack trace.
cadence.Recover(logger)

// Skip the invocation if the previous run is still in progress.
cadence.SkipIfStillRunning(logger)

// Queue the next invocation until the current one finishes.
cadence.DelayIfStillRunning(logger)
```

### Per-job via Chain

```go
chain := cadence.NewChain(
    cadence.Recover(logger),
    cadence.SkipIfStillRunning(logger),
)
id := c.Schedule(schedule, chain.Then(myJob))
```

### Global via WithChain

```go
c := cadence.New(
    cadence.WithChain(
        cadence.Recover(cadence.DefaultLogger),
        cadence.SkipIfStillRunning(cadence.DefaultLogger),
    ),
)
```

### Custom middleware

```go
func Logging(logger cadence.Logger) cadence.JobWrapper {
    return func(j cadence.Job) cadence.Job {
        return cadence.FuncJob(func() {
            logger.Info("job starting")
            j.Run()
            logger.Info("job done")
        })
    }
}
```

---

## Logger interface

```go
type Logger interface {
    Info(msg string, keysAndValues ...interface{})
    Error(err error, msg string, keysAndValues ...interface{})
}
```

| Logger | Description |
|--------|-------------|
| `cadence.DefaultLogger` | Wraps `log.Default()`; logs errors only |
| `cadence.DiscardLogger` | Silently discards all output |
| `cadence.PrintfLogger(l)` | Wraps any `Printf`-style logger; logs errors only |
| `cadence.VerbosePrintfLogger(l)` | Like `PrintfLogger` but also logs `Info` messages |

---

## Parser

The parser is configurable via `ParseOption` bit flags. `NewParser` composes
them:

```go
// Standard 5-field with descriptors (the default).
p := cadence.NewParser(
    cadence.Minute | cadence.Hour | cadence.Dom |
    cadence.Month | cadence.Dow | cadence.Descriptor,
)

// 6-field with optional seconds.
p := cadence.NewParser(
    cadence.SecondOptional | cadence.Minute | cadence.Hour |
    cadence.Dom | cadence.Month | cadence.Dow | cadence.Descriptor,
)

schedule, err := p.Parse("*/5 * * * *")
```

`ParseStandard` is a convenience wrapper for the default 5-field parser:

```go
schedule, err := cadence.ParseStandard("0 9 * * MON-FRI")
```

### ParseOption constants

| Constant | Meaning |
|----------|---------|
| `Second` | Seconds field, required |
| `SecondOptional` | Seconds field, optional (accepted if present) |
| `Minute` | Minutes field, required |
| `Hour` | Hours field, required |
| `Dom` | Day-of-month field, required |
| `Month` | Month field, required |
| `Dow` | Day-of-week field, required |
| `DowOptional` | Day-of-week field, optional |
| `Descriptor` | Enable `@yearly`, `@every`, and other descriptors |

---

## Clock interface and testing

Implement `Clock` to make scheduling deterministic in tests:

```go
type Clock interface {
    Now() time.Time
    After(d time.Duration) <-chan time.Time
}
```

Example fake clock:

```go
type fakeClock struct {
    mu  sync.Mutex
    now time.Time
    ch  chan time.Time
}

func (fc *fakeClock) Now() time.Time                        { fc.mu.Lock(); defer fc.mu.Unlock(); return fc.now }
func (fc *fakeClock) After(d time.Duration) <-chan time.Time { return fc.ch }
func (fc *fakeClock) Advance(d time.Duration) {
    fc.mu.Lock()
    fc.now = fc.now.Add(d)
    t := fc.now
    fc.mu.Unlock()
    fc.ch <- t
}

c := cadence.New(cadence.WithClock(&fakeClock{now: time.Now(), ch: make(chan time.Time, 1)}))
```

---

## Bug fixes over robfig/cron v3

| # | robfig/cron issue | Fix in cadence |
|---|-------------------|----------------|
| 1 | TZ prefix panics on unknown timezone (#554, #555) | `time.LoadLocation` error is returned, not panicked |
| 2 | Malformed expressions panic on missing fields (#543) | Field count validated after TZ extraction; returns error |
| 3 | `*/0` and zero-step expressions accepted silently | Step values ≤ 0 or exceeding field range are rejected with an error |
| 4 | `SkipIfStillRunning` bypass (#551) | Scheduler consistently uses `WrappedJob` in all execution paths |
| 5 | DST spring-forward infinite loop (#541) | Hour loop uses `Truncate`+`Add`; jobs in a DST gap fire at the first instant after the gap |
| 6 | Double execution on backward clock jumps (#553) | `Next` is forced strictly after `Prev` using monotonic comparison |
| 7 | Sunday=7 rejected (#522) | Value `7` in the day-of-week field is normalised to `0` |
| 8 | System clock jumps | Scheduler detects negative sleep duration and clamps to zero |

---

## Migration from robfig/cron v3

Change the import path. The API is otherwise identical:

```diff
-import "github.com/robfig/cron/v3"
+import "github.com/agentine/cadence"
```

All types — `Cron`, `Entry`, `EntryID`, `Job`, `FuncJob`, `Schedule`,
`ScheduleParser`, `Chain`, `JobWrapper`, `Option`, `ParseOption` — and all
middleware helpers — `Recover`, `SkipIfStillRunning`, `DelayIfStillRunning` —
retain the same names and signatures.

For large codebases a one-liner replacement works:

```sh
find . -type f -name '*.go' \
  -exec sed -i '' 's|github.com/robfig/cron/v3|github.com/agentine/cadence|g' {} +
```

### New additions (backward-compatible)

These are new in cadence and have no equivalent in robfig/cron:

| Addition | Description |
|----------|-------------|
| `WithClock(clock)` | Injectable clock for deterministic tests |
| `WithContext(ctx)` | Parent context; cancellation signals shutdown |
| `WithJitter(d)` | Random jitter on each fire time |
| `AddFuncContext(spec, func(context.Context))` | Context-aware job function |
| `IsRunning() bool` | Query scheduler state |
| `Entry.Valid() bool` | Check if an entry ID is still registered |

---

## License

MIT — see [LICENSE](LICENSE).
