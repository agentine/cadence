# cadence

A zero-dependency Go cron scheduler — a drop-in replacement for
[`github.com/robfig/cron/v3`](https://github.com/robfig/cron).

Requires Go 1.21+.

---

## Installation

```sh
go get github.com/agentine/cadence
```

---

## Quickstart

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

## Migration from robfig/cron

Change the import path — the API is otherwise identical.

```diff
-import "github.com/robfig/cron/v3"
+import "github.com/agentine/cadence"
```

All types (`Cron`, `Entry`, `EntryID`, `Job`, `FuncJob`, `Schedule`,
`ScheduleParser`, `Chain`, `JobWrapper`, `Option`, `ParseOption`) and
middleware helpers (`Recover`, `SkipIfStillRunning`, `DelayIfStillRunning`)
retain the same names and signatures.

---

## API Reference

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
```

### Lifecycle

```go
c.Start()          // start in a background goroutine (non-blocking)
c.Run()            // start in the foreground (blocking)
ctx := c.Stop()    // signal shutdown; ctx is done when all jobs finish
<-ctx.Done()
```

### Inspecting entries

```go
entries := c.Entries()      // []Entry snapshot
entry   := c.Entry(id)      // single Entry (Entry.Valid() == false if not found)
c.Remove(id)                // remove by ID (safe while running)
loc     := c.Location()     // configured time.Location
```

### Cron expression format

Five-field standard format (default):

```
┌───── minute       (0–59)
│ ┌─── hour         (0–23)
│ │ ┌─ day-of-month (1–31)
│ │ │ ┌ month       (1–12 or JAN–DEC)
│ │ │ │ ┌ day-of-week (0–6 or SUN–SAT; 0 and 7 = Sunday)
│ │ │ │ │
* * * * *
```

Special characters: `*` (any), `,` (list), `-` (range), `/` (step).

Descriptors (when `Descriptor` option is enabled — default):

| Descriptor | Equivalent |
|-----------|-----------|
| `@yearly` / `@annually` | `0 0 1 1 *` |
| `@monthly` | `0 0 1 * *` |
| `@weekly` | `0 0 * * 0` |
| `@daily` / `@midnight` | `0 0 * * *` |
| `@hourly` | `0 * * * *` |
| `@every <duration>` | runs every `<duration>` (e.g. `@every 30s`) |

Timezone prefix: `TZ=America/New_York 0 9 * * *`

---

## Options

| Option | Description |
|--------|-------------|
| `WithLocation(loc)` | Set the scheduler's time zone (default: `time.Local`) |
| `WithSeconds()` | Enable 6-field cron format (second, minute, hour, dom, month, dow) |
| `WithParser(p)` | Use a custom `ScheduleParser` |
| `WithChain(wrappers...)` | Apply middleware to every job added to this scheduler |
| `WithLogger(logger)` | Set the `Logger` used by middleware |
| `WithClock(clock)` | Replace the real clock (useful for testing) |
| `WithContext(ctx)` | Set a parent context; cancelling it signals shutdown |
| `WithJitter(d)` | Add random jitter up to `d` to each job's next fire time |

---

## Middleware

Middleware wraps jobs to add cross-cutting behaviour. Apply per-job or globally
via `WithChain`.

### Built-in wrappers

```go
// Recover from panics and log them.
cadence.Recover(logger)

// Skip invocation if the previous run is still in progress.
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
wrappedJob := chain.Then(myJob)
id := c.Schedule(schedule, wrappedJob)
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

### Loggers

| Logger | Description |
|--------|-------------|
| `cadence.DefaultLogger` | Wraps `log.Default()` |
| `cadence.DiscardLogger` | Silently discards all output |
| `cadence.PrintfLogger(l)` | Wraps any `Printf`-style logger; logs errors only |
| `cadence.VerbosePrintfLogger(l)` | Like `PrintfLogger` but also logs Info messages |

---

## Testing with a fake clock

Implement the `Clock` interface to make scheduling deterministic in tests:

```go
type fakeClock struct {
    mu  sync.Mutex
    now time.Time
    ch  chan time.Time
}

func (fc *fakeClock) Now() time.Time { ... }
func (fc *fakeClock) After(d time.Duration) <-chan time.Time { return fc.ch }
func (fc *fakeClock) Advance(d time.Duration) { fc.ch <- fc.now.Add(d) }

c := cadence.New(cadence.WithClock(&fakeClock{now: time.Now()}))
```

---

## License

MIT — see [LICENSE](LICENSE).
