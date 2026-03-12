// Package cadence implements a cron scheduler with a compatible API to
// robfig/cron v3.  It parses standard and extended cron expressions,
// manages entries in a goroutine-based run loop, and supports middleware
// chains for recovery, deduplication, and delay.
//
// Zero external dependencies.  Go 1.21+.
package cadence
