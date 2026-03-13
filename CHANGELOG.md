# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-13

Initial release of **cadence** — a zero-dependency cron scheduler for Go that replaces [robfig/cron](https://github.com/robfig/cron) with race-free scheduling, DST-aware execution, and a richer middleware API.

### Added

- **Cron expression parser** (`parser.go`) — standard 5-field (`min hour dom month dow`) and 6-field (`sec min hour dom month dow`) cron expressions; named schedules (`@yearly`, `@monthly`, `@weekly`, `@daily`, `@hourly`, `@every <duration>`); `ParseStandard` and `Parse` entry points.
- **`Schedule` interface** — `Next(time.Time) time.Time`; implemented by `SpecSchedule` (cron expression), `ConstantDelaySchedule` (`@every`), and custom user types.
- **`Cron` scheduler** (`cron.go`, `schedule.go`) — `New(opts...)`, `AddFunc(spec, func) (EntryID, error)`, `AddJob(spec, Job) (EntryID, error)`, `Schedule(schedule, job) EntryID`, `Remove(EntryID)`, `Entries() []Entry`, `Entry(EntryID) Entry`, `Start()`, `Stop() context.Context`, `Run()`.
- **`ContextualJob`** — `FuncJob` wraps `func()` for use with `AddJob`; `ContextualJob` wraps `func(context.Context)` for cooperative cancellation via `Stop()`.
- **`IsRunning()`** — returns true if the scheduler is currently running.
- **Middleware** (`middleware.go`) — `JobWrapper` / `Chain` composable middleware; built-in `Recover` (panic recovery with logging), `DelayIfStillRunning` (skip-if-busy double-execution prevention), `SkipIfStillRunning` (drop the run if job hasn't finished).
- **Logger** (`logger.go`) — `Logger` interface with `Printf`; `PrintfLogger(l)` adapter for `*log.Logger`; `VerbosePrintfLogger` for debug output; `DiscardLogger` for silence.
- **Options** (`option.go`) — `WithLocation(loc)`, `WithSeconds()`, `WithLogger(logger)`, `WithParser(parser)`, `WithChain(wrappers...)`.
- **DST handling** — spring-forward gaps detected using `time.Truncate` + `time.Add`; scheduled hours in a DST gap are skipped rather than causing an infinite loop.
- **Race-free** — `sync.Mutex` protects `nextID`, `running` flag, `entries` slice, and `stop`/`snapshot` channels. Verified clean under `go test -race`.
- **Benchmarks** (`bench_test.go`) — scheduler throughput and `Next()` computation benchmarks.
- **47 tests** with `-race` across cron parsing, scheduling, middleware, logger, and options.

### Fixed

- Race condition on `nextID` in concurrent `Schedule()` calls — now incremented under mutex.
- Race condition on `running` flag — reads and writes serialized under mutex in `Start()`, `Stop()`, and `IsRunning()`.
- Race condition on `stop` channel — channel replaced atomically to prevent send-on-closed-channel panic.
- DST spring-forward infinite loop — `time.Date` backward jump in gap caused infinite loop; rewrote hour loop with `Truncate` + `Add`.
- Double-execution prevention — `DelayIfStillRunning` and `SkipIfStillRunning` middleware correctly prevents concurrent re-entry.
- `WithContext` option — scheduler respects `ctx.Done()` for graceful shutdown.
