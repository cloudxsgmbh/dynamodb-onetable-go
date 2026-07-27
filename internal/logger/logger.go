package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	// DEBUG enables verbose diagnostic logs.
	DEBUG LogLevel = "DEBUG"
	// INFO is used for informational logs.
	INFO LogLevel = "INFO"
	// WARNING is used for warning logs.
	WARNING LogLevel = "WARNING"
	// ERROR is used for error logs.
	ERROR LogLevel = "ERROR"
	// FATAL is used for fatal logs that exit the process.
	FATAL LogLevel = "FATAL"
)

// LogEntry represents one structured log message.
type LogEntry struct {
	Timestamp string   `json:"timestamp"`
	Level     LogLevel `json:"level"`
	Message   string   `json:"message"`
	Fields    any      `json:"fields,omitempty"`
}

const debugEnvVar = "DEBUG_LOGS_ENABLED"

func init() {
	log.SetFlags(0)
}

func debugEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(debugEnvVar)))
	sstDev := os.Getenv("SST_DEV")

	if sstDev == "true" {
		return true
	}

	return value == "true" || value == "1" || value == "yes" || value == "y"
}

func logJSON(level LogLevel, msg string, fields any) {
	if level == DEBUG && !debugEnabled() {
		return
	}
	entry := LogEntry{
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level,
		Message:   msg,
		Fields:    normalizeFields(fields),
	}
	entryBytes, err := json.Marshal(entry)
	if err != nil {
		log.Printf(`{"timestamp":"%s","level":"ERROR","message":"Failed to marshal log entry: %v"}`,
			time.Now().Format(time.RFC3339Nano), err)
		fallbackEntry := LogEntry{
			Timestamp: time.Now().Format(time.RFC3339Nano),
			Level:     level,
			Message:   msg,
			Fields:    fmt.Sprintf("%#v", fields),
		}
		fallbackBytes, fallbackErr := json.Marshal(fallbackEntry)
		if fallbackErr != nil {
			log.Printf(`{"timestamp":"%s","level":"ERROR","message":"Failed to marshal fallback log entry: %v"}`,
				time.Now().Format(time.RFC3339Nano), fallbackErr)
			return
		}
		log.Printf("%s", string(fallbackBytes))
		return
	}
	log.Printf("%s", string(entryBytes))
	if level == FATAL {
		os.Exit(1)
	}
}

func normalizeFields(fields any) any {
	switch value := fields.(type) {
	case nil:
		return nil
	case error:
		return value.Error()
	case fmt.Stringer:
		return value.String()
	case []any:
		normalized := make([]any, len(value))
		for i, entry := range value {
			normalized[i] = normalizeFields(entry)
		}
		return normalized
	case map[string]any:
		normalized := make(map[string]any, len(value))
		for key, entry := range value {
			normalized[key] = normalizeFields(entry)
		}
		return normalized
	default:
		return value
	}
}

// Debug logs a debug message.
func Debug(msg string, fields ...any) {
	logJSON(DEBUG, msg, fields)
}

// Info logs an informational message.
func Info(msg string, fields ...any) {
	logJSON(INFO, msg, fields)
}

// Warn logs a warning message.
func Warn(msg string, fields ...any) {
	logJSON(WARNING, msg, fields)
}

// Error logs an error message.
func Error(msg string, fields ...any) {
	logJSON(ERROR, msg, fields)
}

// Fatal logs a fatal message and exits.
func Fatal(msg string, fields ...any) {
	logJSON(FATAL, msg, fields)
}
