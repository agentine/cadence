package cadence

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Parser parses cron specs according to the configured options.
type Parser struct {
	options ParseOption
}

// NewParser creates a Parser with the given options.
//
// Common presets:
//
//	NewParser(Minute | Hour | Dom | Month | Dow | Descriptor)        // standard 5-field
//	NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor)  // 6-field with seconds
func NewParser(options ParseOption) Parser {
	return Parser{options: options}
}

// Parse parses a cron spec string.
func (p Parser) Parse(spec string) (Schedule, error) {
	if spec == "" {
		return nil, fmt.Errorf("empty spec string")
	}

	// Handle descriptors.
	if spec[0] == '@' {
		if p.options&Descriptor == 0 {
			return nil, fmt.Errorf("descriptors not enabled: %s", spec)
		}
		return parseDescriptor(spec)
	}

	// Handle TZ/CRON_TZ prefix.
	var loc *time.Location
	if strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=") {
		var err error
		loc, spec, err = parseTZPrefix(spec)
		if err != nil {
			return nil, err
		}
	}

	// Build the list of expected fields.
	fields := strings.Fields(spec)
	expected := p.expectedFieldCount()
	optionalSecond := p.options&SecondOptional != 0

	if optionalSecond {
		if len(fields) != expected && len(fields) != expected-1 {
			return nil, fmt.Errorf("expected %d or %d fields, got %d: %s",
				expected-1, expected, len(fields), spec)
		}
		if len(fields) == expected-1 {
			// No second field: prepend "0".
			fields = append([]string{"0"}, fields...)
		}
	} else if len(fields) != expected {
		return nil, fmt.Errorf("expected %d fields, got %d: %s",
			expected, len(fields), spec)
	}

	// Parse each field.
	fieldIdx := 0
	schedule := &SpecSchedule{Location: loc}

	type fieldDef struct {
		name string
		min  int
		max  int
		dest *uint64
	}

	// Build ordered field definitions based on options.
	var defs []fieldDef
	if p.options&Second != 0 || p.options&SecondOptional != 0 {
		defs = append(defs, fieldDef{"second", 0, 59, &schedule.Second})
	}
	if p.options&Minute != 0 {
		defs = append(defs, fieldDef{"minute", 0, 59, &schedule.Minute})
	}
	if p.options&Hour != 0 {
		defs = append(defs, fieldDef{"hour", 0, 23, &schedule.Hour})
	}
	if p.options&Dom != 0 {
		defs = append(defs, fieldDef{"day-of-month", 1, 31, &schedule.Dom})
	}
	if p.options&Month != 0 {
		defs = append(defs, fieldDef{"month", 1, 12, &schedule.Month})
	}
	if p.options&Dow != 0 || p.options&DowOptional != 0 {
		defs = append(defs, fieldDef{"day-of-week", 0, 6, &schedule.Dow})
	}

	for _, def := range defs {
		if fieldIdx >= len(fields) {
			return nil, fmt.Errorf("not enough fields")
		}
		bits, err := parseField(fields[fieldIdx], def.min, def.max, def.name)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s field %q: %w",
				def.name, fields[fieldIdx], err)
		}
		*def.dest = bits
		fieldIdx++
	}

	// Fill defaults for unset fields.
	if schedule.Second == 0 && p.options&Second == 0 && p.options&SecondOptional == 0 {
		schedule.Second = 1 << 0 // second 0
	}
	if schedule.Minute == 0 && p.options&Minute == 0 {
		schedule.Minute = 1 << 0
	}
	if schedule.Hour == 0 && p.options&Hour == 0 {
		schedule.Hour = 1 << 0
	}

	return schedule, nil
}

func (p Parser) expectedFieldCount() int {
	n := 0
	if p.options&Second != 0 || p.options&SecondOptional != 0 {
		n++
	}
	if p.options&Minute != 0 {
		n++
	}
	if p.options&Hour != 0 {
		n++
	}
	if p.options&Dom != 0 {
		n++
	}
	if p.options&Month != 0 {
		n++
	}
	if p.options&Dow != 0 || p.options&DowOptional != 0 {
		n++
	}
	return n
}

// ---------------------------------------------------------------------------
// Descriptor parsing
// ---------------------------------------------------------------------------

func parseDescriptor(spec string) (Schedule, error) {
	lower := strings.ToLower(spec)
	switch {
	case lower == "@yearly" || lower == "@annually":
		return &SpecSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    1 << 1,
			Month:  1 << 1,
			Dow:    allDow,
		}, nil
	case lower == "@monthly":
		return &SpecSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    1 << 1,
			Month:  allMonths,
			Dow:    allDow,
		}, nil
	case lower == "@weekly":
		return &SpecSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    allDom,
			Month:  allMonths,
			Dow:    1 << 0,
		}, nil
	case lower == "@daily" || lower == "@midnight":
		return &SpecSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   1 << 0,
			Dom:    allDom,
			Month:  allMonths,
			Dow:    allDow,
		}, nil
	case lower == "@hourly":
		return &SpecSchedule{
			Second: 1 << 0,
			Minute: 1 << 0,
			Hour:   allHours,
			Dom:    allDom,
			Month:  allMonths,
			Dow:    allDow,
		}, nil
	case strings.HasPrefix(lower, "@every "):
		durationStr := strings.TrimSpace(spec[7:])
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse @every duration: %w", err)
		}
		return Every(d), nil
	default:
		return nil, fmt.Errorf("unrecognised descriptor: %s", spec)
	}
}

var allMonths uint64 = (1<<13 - 1) &^ 1 // bits 1..12
var allHours uint64 = 1<<24 - 1          // bits 0..23

// ---------------------------------------------------------------------------
// TZ prefix
// ---------------------------------------------------------------------------

func parseTZPrefix(spec string) (*time.Location, string, error) {
	var tzName, rest string
	if strings.HasPrefix(spec, "CRON_TZ=") {
		parts := strings.SplitN(spec[8:], " ", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid CRON_TZ prefix: %s", spec)
		}
		tzName = parts[0]
		rest = parts[1]
	} else if strings.HasPrefix(spec, "TZ=") {
		parts := strings.SplitN(spec[3:], " ", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid TZ prefix: %s", spec)
		}
		tzName = parts[0]
		rest = parts[1]
	}

	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, "", fmt.Errorf("unrecognised timezone %q: %w", tzName, err)
	}
	return loc, rest, nil
}

// ---------------------------------------------------------------------------
// Field parsing
// ---------------------------------------------------------------------------

// parseField parses a single cron field (e.g., "1-5", "*/2", "1,3,5")
// and returns a bitset.
func parseField(field string, min, max int, name string) (uint64, error) {
	if field == "*" || field == "?" {
		return allBits(min, max), nil
	}

	var bits uint64
	// Split by comma.
	parts := strings.Split(field, ",")
	for _, part := range parts {
		b, err := parseRange(part, min, max, name)
		if err != nil {
			return 0, err
		}
		bits |= b
	}
	return bits, nil
}

func parseRange(field string, min, max int, name string) (uint64, error) {
	// Check for step: "a-b/c" or "*/c".
	var step int
	if idx := strings.IndexByte(field, '/'); idx >= 0 {
		var err error
		step, err = strconv.Atoi(field[idx+1:])
		if err != nil || step <= 0 {
			return 0, fmt.Errorf("invalid step in %s field: %q", name, field)
		}
		field = field[:idx]
	}

	// All values.
	if field == "*" || field == "?" {
		return steppedBits(min, max, step), nil
	}

	// Range: "a-b".
	if idx := strings.IndexByte(field, '-'); idx >= 0 {
		lo, err := parseValue(field[:idx], min, max, name)
		if err != nil {
			return 0, err
		}
		hi, err := parseValue(field[idx+1:], min, max, name)
		if err != nil {
			return 0, err
		}
		if lo > hi {
			return 0, fmt.Errorf("invalid range %d-%d in %s field", lo, hi, name)
		}
		if step > 0 {
			return steppedBits(lo, hi, step), nil
		}
		return rangeBits(lo, hi), nil
	}

	// Single value.
	val, err := parseValue(field, min, max, name)
	if err != nil {
		return 0, err
	}
	if step > 0 {
		return steppedBits(val, max, step), nil
	}
	return 1 << uint(val), nil
}

func parseValue(s string, min, max int, name string) (int, error) {
	// Check for named values (months and weekdays).
	if name == "month" {
		if v, ok := monthNames[strings.ToLower(s)]; ok {
			return v, nil
		}
	}
	if name == "day-of-week" {
		if v, ok := dowNames[strings.ToLower(s)]; ok {
			return v, nil
		}
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value %q in %s field", s, name)
	}

	// Sunday=7 → Sunday=0.
	if name == "day-of-week" && val == 7 {
		val = 0
	}

	if val < min || val > max {
		return 0, fmt.Errorf("value %d out of range [%d, %d] in %s field",
			val, min, max, name)
	}
	return val, nil
}

// ---------------------------------------------------------------------------
// Bitset helpers
// ---------------------------------------------------------------------------

func allBits(min, max int) uint64 {
	return rangeBits(min, max)
}

func rangeBits(lo, hi int) uint64 {
	var bits uint64
	for i := lo; i <= hi; i++ {
		bits |= 1 << uint(i)
	}
	return bits
}

func steppedBits(min, max, step int) uint64 {
	if step <= 0 {
		return rangeBits(min, max)
	}
	var bits uint64
	for i := min; i <= max; i += step {
		bits |= 1 << uint(i)
	}
	return bits
}

// ---------------------------------------------------------------------------
// Named values
// ---------------------------------------------------------------------------

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4,
	"may": 5, "jun": 6, "jul": 7, "aug": 8,
	"sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var dowNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3,
	"thu": 4, "fri": 5, "sat": 6,
}

// ---------------------------------------------------------------------------
// StandardParser is the default 5-field parser with descriptors.
// ---------------------------------------------------------------------------

// StandardParser is a Parser that handles the standard 5-field cron format
// plus descriptors.
var StandardParser = NewParser(Minute | Hour | Dom | Month | Dow | Descriptor)

// Ensure SpecSchedule and ConstantDelaySchedule satisfy Schedule.
var _ Schedule = &SpecSchedule{}
var _ Schedule = ConstantDelaySchedule{}

// Ensure Parser satisfies ScheduleParser.
var _ ScheduleParser = Parser{}

