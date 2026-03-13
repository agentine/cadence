package cadence

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// New creates a new Cron scheduler with the given options.
func New(opts ...Option) *Cron {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Cron{
		entries:  nil,
		parser:   StandardParser,
		nextID:   0,
		location: time.Local,
		clock:    realClock{},
		running:  false,
		stop:     make(chan struct{}),
		add:      make(chan *Entry),
		remove:   make(chan EntryID),
		snapshot: make(chan chan []Entry),
		chain:    NewChain(),
		logger:   DiscardLogger,
		ctx:      ctx,
		cancel:   cancel,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// AddFunc adds a function to the cron schedule.
func (c *Cron) AddFunc(spec string, cmd func()) (EntryID, error) {
	return c.AddJob(spec, FuncJob(cmd))
}

// AddJob adds a Job to the cron schedule.
func (c *Cron) AddJob(spec string, job Job) (EntryID, error) {
	schedule, err := c.parser.Parse(spec)
	if err != nil {
		return 0, err
	}
	return c.Schedule(schedule, job), nil
}

// Schedule adds a Job with a pre-built Schedule.
func (c *Cron) Schedule(schedule Schedule, job Job) EntryID {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	entry := &Entry{
		ID:         id,
		Schedule:   schedule,
		Job:        job,
		WrappedJob: c.chain.Then(job),
	}

	if c.running {
		c.mu.Unlock()
		c.add <- entry
	} else {
		c.entries = append(c.entries, entry)
		c.mu.Unlock()
	}
	return id
}

// Remove removes the entry with the given ID.
func (c *Cron) Remove(id EntryID) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		c.remove <- id
	} else {
		c.removeEntry(id)
		c.mu.Unlock()
	}
}

// Entries returns a snapshot of the current entries.
func (c *Cron) Entries() []Entry {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		replyCh := make(chan []Entry, 1)
		c.snapshot <- replyCh
		return <-replyCh
	}
	snap := c.entrySnapshot()
	c.mu.Unlock()
	return snap
}

// Entry returns the entry with the given ID, or an empty Entry if not found.
func (c *Cron) Entry(id EntryID) Entry {
	for _, e := range c.Entries() {
		if e.ID == id {
			return e
		}
	}
	return Entry{}
}

// Start starts the cron scheduler in a background goroutine.
func (c *Cron) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return
	}
	c.running = true
	go c.run()
}

// Run starts the cron scheduler in the foreground (blocking).
func (c *Cron) Run() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	c.run()
}

// Stop signals the cron scheduler to stop. The returned context is
// done when all currently-running jobs have completed.
func (c *Cron) Stop() context.Context {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return c.ctx
	}
	c.running = false
	c.mu.Unlock()
	c.stop <- struct{}{}
	return c.ctx
}

// Location returns the scheduler's time location.
func (c *Cron) Location() *time.Location {
	return c.location
}

// IsRunning reports whether the scheduler is running.
func (c *Cron) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// AddFuncContext adds a context-aware function to the cron schedule.
// The function receives the scheduler's context when executed.
func (c *Cron) AddFuncContext(spec string, cmd func(context.Context)) (EntryID, error) {
	return c.AddJob(spec, &contextFuncJob{fn: cmd})
}

// ---------------------------------------------------------------------------
// Run loop
// ---------------------------------------------------------------------------

func (c *Cron) run() {
	now := c.clock.Now().In(c.location)
	for _, entry := range c.entries {
		entry.Next = c.applyJitter(entry.Schedule.Next(now))
	}

	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		c.cancel()
	}()

	for {
		sort.Slice(c.entries, func(i, j int) bool {
			return c.entries[i].Next.Before(c.entries[j].Next)
		})

		var timer <-chan time.Time
		if len(c.entries) == 0 {
			// No entries: wait for add or stop.
			timer = nil
		} else {
			d := c.entries[0].Next.Sub(c.clock.Now().In(c.location))
			if d < 0 {
				d = 0
			}
			timer = c.clock.After(d)
		}

		select {
		case now = <-timer:
			now = now.In(c.location)
			// Run all entries whose time has come.
			for _, entry := range c.entries {
				if entry.Next.After(now) || entry.Next.IsZero() {
					break
				}
				wg.Add(1)
				go func(e *Entry) {
					defer wg.Done()
					if cj, ok := e.WrappedJob.(ContextualJob); ok {
						cj.RunContext(c.ctx)
					} else {
						e.WrappedJob.Run()
					}
				}(entry)
				entry.Prev = entry.Next
				entry.Next = c.applyJitter(entry.Schedule.Next(now))
				// Prevent double execution on backward clock jumps: ensure
				// Next is strictly after Prev.
				if !entry.Next.After(entry.Prev) {
					entry.Next = c.applyJitter(entry.Schedule.Next(entry.Prev))
				}
			}

		case newEntry := <-c.add:
			now = c.clock.Now().In(c.location)
			newEntry.Next = c.applyJitter(newEntry.Schedule.Next(now))
			c.entries = append(c.entries, newEntry)

		case id := <-c.remove:
			c.removeEntry(id)

		case replyCh := <-c.snapshot:
			replyCh <- c.entrySnapshot()

		case <-c.stop:
			return
		}
	}
}

func (c *Cron) removeEntry(id EntryID) {
	for i, e := range c.entries {
		if e.ID == id {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			return
		}
	}
}

func (c *Cron) entrySnapshot() []Entry {
	entries := make([]Entry, len(c.entries))
	for i, e := range c.entries {
		entries[i] = *e
	}
	return entries
}

// applyJitter adds a random jitter of up to c.jitter to the given time.
func (c *Cron) applyJitter(t time.Time) time.Time {
	if c.jitter <= 0 {
		return t
	}
	return t.Add(time.Duration(rand.Int63n(int64(c.jitter))))
}
