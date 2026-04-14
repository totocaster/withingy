package debuglog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const EnvVar = "WITHINGY_DEBUG_AUTH_LOG"

var (
	defaultLogger *Logger
	defaultOnce   sync.Once
)

// Logger writes structured debug events when enabled by environment variable.
type Logger struct {
	mu      sync.Mutex
	enabled bool
	writer  io.Writer
}

// Default returns the process-wide debug logger.
func Default() *Logger {
	defaultOnce.Do(func() {
		defaultLogger = newLoggerFromEnv()
	})
	return defaultLogger
}

// Enabled reports whether debug logging is active.
func (l *Logger) Enabled() bool {
	return l != nil && l.enabled && l.writer != nil
}

// Log appends a structured event. Nil and empty values are omitted.
func (l *Logger) Log(event string, fields map[string]any) {
	if !l.Enabled() {
		return
	}

	record := map[string]any{
		"event": event,
		"at":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	for key, value := range fields {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		}
		record[key] = value
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_ = json.NewEncoder(l.writer).Encode(record)
}

// Fingerprint returns a short SHA-256 fingerprint suitable for correlating tokens in logs.
func Fingerprint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:12]
}

// ParseResponseDate extracts the server date header when present.
func ParseResponseDate(resp *http.Response) (time.Time, bool) {
	if resp == nil {
		return time.Time{}, false
	}
	dateHeader := strings.TrimSpace(resp.Header.Get("Date"))
	if dateHeader == "" {
		return time.Time{}, false
	}
	parsed, err := http.ParseTime(dateHeader)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func newLoggerFromEnv() *Logger {
	path := strings.TrimSpace(os.Getenv(EnvVar))
	if path == "" {
		return &Logger{}
	}
	if path == "stderr" {
		return &Logger{enabled: true, writer: os.Stderr}
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		_, _ = io.WriteString(os.Stderr, "withingy: failed to open debug log; falling back to stderr: "+err.Error()+"\n")
		return &Logger{enabled: true, writer: os.Stderr}
	}
	return &Logger{enabled: true, writer: file}
}
