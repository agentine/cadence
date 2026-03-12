package cadence

import (
	"context"
	"time"
)

// WithLocation sets the time location for the scheduler.
func WithLocation(loc *time.Location) Option {
	return func(c *Cron) {
		c.location = loc
	}
}

// WithParser sets the cron spec parser.
func WithParser(p ScheduleParser) Option {
	return func(c *Cron) {
		c.parser = p
	}
}

// WithSeconds enables a 6-field cron format (with seconds).
func WithSeconds() Option {
	return WithParser(NewParser(
		Second | Minute | Hour | Dom | Month | Dow | Descriptor,
	))
}

// WithChain sets the job wrapper chain.
func WithChain(wrappers ...JobWrapper) Option {
	return func(c *Cron) {
		c.chain = NewChain(wrappers...)
	}
}

// WithLogger sets the logger for the scheduler.
func WithLogger(logger Logger) Option {
	return func(c *Cron) {
		c.logger = logger
	}
}

// WithClock sets the clock for the scheduler.
func WithClock(clock Clock) Option {
	return func(c *Cron) {
		c.clock = clock
	}
}

// WithContext sets a parent context for the scheduler. All running jobs
// share this context, and cancelling it signals shutdown.
func WithContext(ctx context.Context) Option {
	return func(c *Cron) {
		c.cancel()
		c.ctx, c.cancel = context.WithCancel(ctx)
	}
}

// WithJitter adds a random jitter of up to d to each job's Next time,
// spreading load across a time window.
func WithJitter(d time.Duration) Option {
	return func(c *Cron) {
		c.jitter = d
	}
}
