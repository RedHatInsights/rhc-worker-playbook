package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var LevelFatal slog.Level = 99
var LevelTrace slog.Level = -8
var LevelUnknown slog.Level = -99

// ParseLevel parses the log level string to an slog.Level
func ParseLevel(str string) (slog.Level, error) {
	switch strings.ToUpper(str) {
	case "ERROR":
		return slog.LevelError, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "DEBUG":
		return slog.LevelDebug, nil
	case "TRACE":
		// slog has no trace level by default
		return LevelTrace, nil
	}

	return LevelUnknown, fmt.Errorf("cannot parse log level: %v", str)
}

// SetLevel sets the log level from a provided slog.Level
func SetLevel(level slog.Level) {
	slog.SetLogLoggerLevel(level)
}

// Debug wraps the slog.Debug method
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// Debugf wraps the slog.Debug method
// and passes the arguments to fmt.Sprintf
func Debugf(msg string, args ...any) {
	slog.Debug(fmt.Sprintf(msg, args...))
}

// Error wraps the slog.Error method
func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}

// Errorf wraps the slog.Error method
// and passes the arguments to fmt.Sprintf
func Errorf(msg string, args ...any) {
	slog.Error(fmt.Sprintf(msg, args...))
}

// Fatal wraps the slog.Log method with a custom log level,
// and mimics go's log.Fatal which calls os.Exit
func Fatal(msg string, args ...any) {
	slog.Log(context.TODO(), LevelFatal, msg, args...)
	os.Exit(1)
}

// Fatal wraps the slog.Log method with a custom log level,
// passes the arguments to fmt.Sprintf,
// and mimics go's log.Fatal which calls os.Exit
func Fatalf(msg string, args ...any) {
	slog.Log(context.TODO(), LevelFatal, fmt.Sprintf(msg, args...))
	os.Exit(1)
}

// Info wraps the slog.Info method
func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

// Infof wraps the slog.Info method
// and passes the arguments to fmt.Sprintf
func Infof(msg string, args ...any) {
	slog.Info(fmt.Sprintf(msg, args...))
}

// Trace wraps the slog.Log method with a custom log level,
func Trace(msg string, args ...any) {
	slog.Log(context.TODO(), LevelTrace, msg, args...)
}

// Tracef wraps the slog.Log method with a custom log level
// and passes the arguments to fmt.Sprintf
func Tracef(msg string, args ...any) {
	slog.Log(context.TODO(), LevelTrace, fmt.Sprintf(msg, args...))
}

// Warn wraps the slog.Warn method
func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// Warnf wraps the slog.Warn method
func Warnf(msg string, args ...any) {
	slog.Warn(fmt.Sprintf(msg, args...))
}
