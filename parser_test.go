package cadence

import (
	"testing"
	"time"
)

func TestStandardParser_Basic(t *testing.T) {
	tests := []struct {
		spec string
		desc string
	}{
		{"* * * * *", "every minute"},
		{"0 0 * * *", "midnight"},
		{"30 4 1 * *", "4:30 AM on 1st"},
		{"0 */2 * * *", "every 2 hours"},
		{"0 0 * * 0", "Sunday midnight"},
		{"15 14 1 * *", "2:15 PM on 1st"},
		{"0 0 1 1 *", "midnight Jan 1st"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := StandardParser.Parse(tt.spec)
			if err != nil {
				t.Errorf("Parse(%q): %v", tt.spec, err)
			}
		})
	}
}

func TestStandardParser_Next(t *testing.T) {
	// Parse "30 4 * * *" (4:30 AM every day).
	sched, err := StandardParser.Parse("30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	next := sched.Next(base)

	// Should be 4:30 AM next day.
	expected := time.Date(2026, 3, 13, 4, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestStandardParser_EveryMinute(t *testing.T) {
	sched, err := StandardParser.Parse("* * * * *")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 3, 12, 10, 30, 0, 0, time.UTC)
	next := sched.Next(base)

	// With default second=0 (5-field), next is 10:31:00.
	expected := time.Date(2026, 3, 12, 10, 31, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestParser_WithSeconds(t *testing.T) {
	p := NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)
	sched, err := p.Parse("0 30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Hour() != 4 || next.Minute() != 30 || next.Second() != 0 {
		t.Errorf("got %v", next)
	}
}

func TestParser_SecondOptional(t *testing.T) {
	p := NewParser(SecondOptional | Minute | Hour | Dom | Month | Dow | Descriptor)

	// Without seconds (5-field).
	sched1, err := p.Parse("30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	next1 := sched1.Next(base)
	if next1.Hour() != 4 || next1.Minute() != 30 {
		t.Errorf("5-field: got %v", next1)
	}

	// With seconds (6-field).
	sched2, err := p.Parse("15 30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}
	next2 := sched2.Next(base)
	if next2.Hour() != 4 || next2.Minute() != 30 || next2.Second() != 15 {
		t.Errorf("6-field: got %v", next2)
	}
}

func TestParser_Ranges(t *testing.T) {
	sched, err := StandardParser.Parse("0 9-17 * * *")
	if err != nil {
		t.Fatal(err)
	}

	// At 8 AM, next should be 9:00.
	base := time.Date(2026, 3, 12, 8, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Hour() != 9 || next.Minute() != 0 {
		t.Errorf("got %v", next)
	}

	// At 5 PM (17:00), next should be 9 AM next day.
	base = time.Date(2026, 3, 12, 17, 0, 1, 0, time.UTC)
	next = sched.Next(base)
	if next.Day() != 13 || next.Hour() != 9 {
		t.Errorf("after 5 PM: got %v", next)
	}
}

func TestParser_Steps(t *testing.T) {
	sched, err := StandardParser.Parse("*/15 * * * *")
	if err != nil {
		t.Fatal(err)
	}
	// */15 = minutes 0,15,30,45. Second defaults to 0.
	// At 10:01, next should be 10:15:00.
	base := time.Date(2026, 3, 12, 10, 1, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Minute() != 15 {
		t.Errorf("got minute %d, want 15", next.Minute())
	}

	// At 10:15, next should be 10:30:00.
	base = time.Date(2026, 3, 12, 10, 15, 0, 0, time.UTC)
	next = sched.Next(base)
	if next.Minute() != 30 {
		t.Errorf("got minute %d, want 30", next.Minute())
	}
}

func TestParser_Lists(t *testing.T) {
	sched, err := StandardParser.Parse("0 1,6,12,18 * * *")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 12, 2, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Hour() != 6 {
		t.Errorf("got hour %d, want 6", next.Hour())
	}
}

func TestParser_MonthNames(t *testing.T) {
	sched, err := StandardParser.Parse("0 0 1 jan *")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Month() != time.January {
		t.Errorf("got month %v", next.Month())
	}
}

func TestParser_DowNames(t *testing.T) {
	sched, err := StandardParser.Parse("0 0 * * mon")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC) // Thursday
	next := sched.Next(base)
	if next.Weekday() != time.Monday {
		t.Errorf("got weekday %v", next.Weekday())
	}
}

func TestParser_Sunday7(t *testing.T) {
	// Sunday as 7 (not just 0).
	sched, err := StandardParser.Parse("0 0 * * 7")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC) // Thursday
	next := sched.Next(base)
	if next.Weekday() != time.Sunday {
		t.Errorf("got weekday %v", next.Weekday())
	}
}

func TestParser_Descriptors(t *testing.T) {
	tests := []struct {
		spec string
		desc string
	}{
		{"@yearly", "yearly"},
		{"@annually", "annually"},
		{"@monthly", "monthly"},
		{"@weekly", "weekly"},
		{"@daily", "daily"},
		{"@midnight", "midnight"},
		{"@hourly", "hourly"},
		{"@every 1h30m", "every 1h30m"},
		{"@every 5s", "every 5s"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			sched, err := StandardParser.Parse(tt.spec)
			if err != nil {
				t.Errorf("Parse(%q): %v", tt.spec, err)
				return
			}
			// Ensure Next() produces a valid time.
			next := sched.Next(time.Now())
			if next.IsZero() {
				t.Errorf("Next() returned zero time for %q", tt.spec)
			}
		})
	}
}

func TestParser_TZPrefix(t *testing.T) {
	sched, err := StandardParser.Parse("TZ=America/New_York 30 4 * * *")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	loc, _ := time.LoadLocation("America/New_York")
	nextNY := next.In(loc)
	if nextNY.Hour() != 4 || nextNY.Minute() != 30 {
		t.Errorf("got %v in NY, want 4:30", nextNY)
	}
}

func TestParser_CRONTZ(t *testing.T) {
	_, err := StandardParser.Parse("CRON_TZ=Europe/London 0 9 * * *")
	if err != nil {
		t.Fatalf("Parse CRON_TZ: %v", err)
	}
}

func TestParser_InvalidSpecs(t *testing.T) {
	invalid := []string{
		"",
		"invalid",
		"* * *",
		"60 * * * *",
		"* 25 * * *",
		"* * 32 * *",
		"* * * 13 *",
		"* * * * 8",
		"@unknown",
	}
	for _, spec := range invalid {
		t.Run(spec, func(t *testing.T) {
			_, err := StandardParser.Parse(spec)
			if err == nil {
				t.Errorf("expected error for spec %q", spec)
			}
		})
	}
}

func TestEvery(t *testing.T) {
	sched := Every(5 * time.Minute)
	base := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	expected := base.Add(5 * time.Minute)
	if !next.Equal(expected) {
		t.Errorf("got %v, want %v", next, expected)
	}
}

func TestEvery_SubSecondRoundup(t *testing.T) {
	sched := Every(500 * time.Millisecond)
	if sched.Delay != time.Second {
		t.Errorf("got %v, want 1s", sched.Delay)
	}
}

func TestSpecSchedule_DomDowOR(t *testing.T) {
	// "0 0 13 * 5" = midnight on 13th OR Friday.
	sched, err := StandardParser.Parse("0 0 13 * 5")
	if err != nil {
		t.Fatal(err)
	}

	// March 13, 2026 is a Friday — should match.
	base := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	// Should be March 13 (which is both the 13th AND a Friday).
	if next.Day() != 13 {
		t.Errorf("got day %d", next.Day())
	}
}

func TestSpecSchedule_YearlyNext(t *testing.T) {
	sched, err := StandardParser.Parse("@yearly")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	next := sched.Next(base)
	if next.Year() != 2027 || next.Month() != 1 || next.Day() != 1 {
		t.Errorf("got %v, want Jan 1 2027", next)
	}
}
