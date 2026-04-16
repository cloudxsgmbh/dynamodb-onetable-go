/*
Package onetable – logging interface.

Mirrors the JS SenseLogs-compatible Log shim inside Table.js.
*/
package onetable

import (
	"encoding/json"
	"log"
)

// Logger is the interface callers may supply to Table.
// Each method receives a structured context map (may be nil).
type Logger interface {
	Trace(message string, ctx map[string]any)
	Info(message string, ctx map[string]any)
	Error(message string, ctx map[string]any)
	Data(message string, ctx map[string]any)
}

// defaultLogger writes info/error to the standard library logger and silently
// drops trace/data (matching JS default behavior).
type defaultLogger struct{}

func (defaultLogger) Trace(string, map[string]any) {}
func (defaultLogger) Data(string, map[string]any)  {}

func (defaultLogger) Info(msg string, ctx map[string]any) {
	logLine("INFO", msg, ctx)
}

func (defaultLogger) Error(msg string, ctx map[string]any) {
	logLine("ERROR", msg, ctx)
}

func logLine(level, msg string, ctx map[string]any) {
	if ctx == nil {
		log.Printf("[%s] %s", level, msg)
		return
	}
	b, err := json.Marshal(ctx)
	if err != nil {
		log.Printf("[%s] %s %v", level, msg, ctx)
		return
	}
	log.Printf("[%s] %s %s", level, msg, b)
}

// verboseLogger additionally prints trace / data lines.
type verboseLogger struct{}

func (verboseLogger) Trace(msg string, ctx map[string]any) { logLine("TRACE", msg, ctx) }
func (verboseLogger) Data(msg string, ctx map[string]any)  { logLine("DATA", msg, ctx) }
func (verboseLogger) Info(msg string, ctx map[string]any)  { logLine("INFO", msg, ctx) }
func (verboseLogger) Error(msg string, ctx map[string]any) { logLine("ERROR", msg, ctx) }

// FuncLogger wraps a plain function: func(level, message string, ctx map[string]any).
type FuncLogger struct {
	Fn func(level, message string, ctx map[string]any)
}

// Trace logs a trace-level message via the wrapped function.
func (f FuncLogger) Trace(msg string, ctx map[string]any) { f.Fn("trace", msg, ctx) }

// Data logs a data-level message via the wrapped function.
func (f FuncLogger) Data(msg string, ctx map[string]any) { f.Fn("data", msg, ctx) }

// Info logs an info-level message via the wrapped function.
func (f FuncLogger) Info(msg string, ctx map[string]any) { f.Fn("info", msg, ctx) }

// Error logs an error-level message via the wrapped function.
func (f FuncLogger) Error(msg string, ctx map[string]any) { f.Fn("error", msg, ctx) }

// logTrace is a convenience helper on Table; same idea as JS this.log.trace(…)
func logTrace(l Logger, msg string, ctx map[string]any) { l.Trace(msg, ctx) }
func logInfo(l Logger, msg string, ctx map[string]any)  { l.Info(msg, ctx) }
func logError(l Logger, msg string, ctx map[string]any) { l.Error(msg, ctx) }
