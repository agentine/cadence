package cadence

import (
	"fmt"
	"log"
	"strings"
)

// PrintfFunc is the subset of *log.Logger used by PrintfLogger.
type PrintfFunc interface {
	Printf(string, ...interface{})
}

// PrintfLogger wraps a Printf-style logger to satisfy the Logger interface.
// Only Info messages are logged; Error messages include the error string.
func PrintfLogger(l PrintfFunc) Logger {
	return &printfLogger{l: l, verbose: false}
}

// VerbosePrintfLogger is like PrintfLogger but also logs Info-level messages
// with key-value pairs.
func VerbosePrintfLogger(l PrintfFunc) Logger {
	return &printfLogger{l: l, verbose: true}
}

type printfLogger struct {
	l       PrintfFunc
	verbose bool
}

func (p *printfLogger) Info(msg string, keysAndValues ...interface{}) {
	if !p.verbose {
		return
	}
	if len(keysAndValues) > 0 {
		p.l.Printf("%s %s", msg, formatKV(keysAndValues))
	} else {
		p.l.Printf("%s", msg)
	}
}

func (p *printfLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	kvStr := formatKV(keysAndValues)
	if kvStr != "" {
		p.l.Printf("error: %s %s error=%v", msg, kvStr, err)
	} else {
		p.l.Printf("error: %s error=%v", msg, err)
	}
}

func formatKV(keysAndValues []interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	var parts []string
	for i := 0; i+1 < len(keysAndValues); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", keysAndValues[i], keysAndValues[i+1]))
	}
	// Handle odd trailing value.
	if len(keysAndValues)%2 != 0 {
		parts = append(parts, fmt.Sprintf("EXTRA=%v", keysAndValues[len(keysAndValues)-1]))
	}
	return strings.Join(parts, " ")
}

// DefaultLogger logs to the standard log package.
var DefaultLogger Logger = PrintfLogger(log.Default())

// DiscardLogger silently discards all log output.
var DiscardLogger Logger = &discardLogger{}

type discardLogger struct{}

func (discardLogger) Info(string, ...interface{})        {}
func (discardLogger) Error(error, string, ...interface{}) {}
