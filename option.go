package cadence

import "time"

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
