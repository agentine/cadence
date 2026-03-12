package cadence

import (
	"testing"
	"time"
)

// BenchmarkParsing benchmarks parsing a standard 5-field cron spec.
func BenchmarkParsing(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := StandardParser.Parse("30 4 1 * *")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParsingDescriptor benchmarks parsing a descriptor spec.
func BenchmarkParsingDescriptor(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := StandardParser.Parse("@every 5m")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParsingWithSeconds benchmarks parsing a 6-field spec.
func BenchmarkParsingWithSeconds(b *testing.B) {
	p := NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse("0 30 4 1 * *")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNext benchmarks computing the next schedule time for a SpecSchedule.
func BenchmarkNext(b *testing.B) {
	sched, err := StandardParser.Parse("30 4 1 * *")
	if err != nil {
		b.Fatal(err)
	}
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sched.Next(t)
	}
}

// BenchmarkNextEvery benchmarks computing next for a ConstantDelaySchedule.
func BenchmarkNextEvery(b *testing.B) {
	sched := Every(5 * time.Minute)
	t := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sched.Next(t)
	}
}

// BenchmarkAddFunc benchmarks adding a function to a stopped Cron.
func BenchmarkAddFunc(b *testing.B) {
	c := New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.AddFunc("* * * * *", func() {})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAddFuncWhileRunning benchmarks adding entries while the scheduler
// is running (exercises the channel path).
func BenchmarkAddFuncWhileRunning(b *testing.B) {
	c := New()
	c.Start()
	b.Cleanup(func() { c.Stop() })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := c.AddFunc("@every 1h", func() {})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEntries benchmarks snapshotting entries from a stopped Cron.
func BenchmarkEntries(b *testing.B) {
	c := New()
	for i := 0; i < 100; i++ {
		c.AddFunc("* * * * *", func() {}) //nolint:errcheck
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Entries()
	}
}

// BenchmarkEntriesWhileRunning benchmarks the snapshot path through the
// run-loop channel.
func BenchmarkEntriesWhileRunning(b *testing.B) {
	c := New()
	for i := 0; i < 100; i++ {
		c.AddFunc("@every 1h", func() {}) //nolint:errcheck
	}
	c.Start()
	b.Cleanup(func() { c.Stop() })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Entries()
	}
}
