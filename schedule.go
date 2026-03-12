package cadence

import "time"

// SpecSchedule represents a parsed cron expression.
// Each field is a bitset where bit N indicates that value N is active.
type SpecSchedule struct {
	Second, Minute, Hour uint64
	Dom                  uint64 // days 1-31
	Month                uint64 // months 1-12
	Dow                  uint64 // 0=Sunday .. 6=Saturday
	Location             *time.Location
}

// Next returns the next time this schedule is activated, after the given time.
func (s *SpecSchedule) Next(t time.Time) time.Time {
	if s.Location != nil {
		t = t.In(s.Location)
	}

	// Start from the next second.
	t = t.Add(1*time.Second - time.Duration(t.Nanosecond())*time.Nanosecond)

	// Bound the search to 5 years to avoid infinite loops.
	yearLimit := t.Year() + 5

WRAP:
	if t.Year() > yearLimit {
		return time.Time{}
	}

	// Month
	for 1<<uint(t.Month())&s.Month == 0 {
		t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
		if t.Year() > yearLimit {
			return time.Time{}
		}
	}

	// Day of month / day of week (OR semantics if both restricted).
	for {
		domMatch := 1<<uint(t.Day())&s.Dom != 0
		dowMatch := 1<<uint(t.Weekday())&s.Dow != 0

		domRestricted := s.Dom != allDom
		dowRestricted := s.Dow != allDow

		var dayOK bool
		if domRestricted && dowRestricted {
			dayOK = domMatch || dowMatch // OR semantics
		} else {
			dayOK = domMatch && dowMatch
		}

		if dayOK {
			break
		}
		t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, t.Location())
		// Wrapped to next month?
		if t.Day() == 1 {
			goto WRAP
		}
	}

	// Hour
	for 1<<uint(t.Hour())&s.Hour == 0 {
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
		if t.Hour() == 0 {
			goto WRAP
		}
	}

	// Minute
	for 1<<uint(t.Minute())&s.Minute == 0 {
		t = t.Add(1*time.Minute - time.Duration(t.Second())*time.Second)
		if t.Minute() == 0 {
			goto WRAP
		}
	}

	// Second
	for 1<<uint(t.Second())&s.Second == 0 {
		t = t.Add(1 * time.Second)
		if t.Second() == 0 {
			goto WRAP
		}
	}

	return t
}

// allDom is the bitset with bits 1-31 set.
var allDom uint64 = (1<<32 - 1) &^ 1 // bits 1..31

// allDow is the bitset with bits 0-6 set.
var allDow uint64 = 1<<7 - 1

// ConstantDelaySchedule represents a fixed-interval schedule.
type ConstantDelaySchedule struct {
	Delay time.Duration
}

// Next returns t + Delay.
func (s ConstantDelaySchedule) Next(t time.Time) time.Time {
	return t.Add(s.Delay)
}

// Every returns a ConstantDelaySchedule with the given duration.
// Durations less than 1 second are rounded up.
func Every(duration time.Duration) ConstantDelaySchedule {
	if duration < time.Second {
		duration = time.Second
	}
	return ConstantDelaySchedule{Delay: duration}
}
