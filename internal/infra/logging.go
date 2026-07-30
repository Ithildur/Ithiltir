package infra

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	kitlog "github.com/Ithildur/EiluneKit/logging"
)

type loggerState struct {
	base   *slog.Logger
	log    *kitlog.Helper
	level  kitlog.Level
	format kitlog.Format
}

var logState atomic.Value
var logMu sync.Mutex

func newLoggerState(level kitlog.Level, format kitlog.Format) loggerState {
	base := kitlog.New(kitlog.Options{
		Level:  level,
		Format: format,
		Writer: os.Stdout,
	})
	return loggerState{
		base:   base,
		log:    kitlog.NewHelper(base),
		level:  level,
		format: format,
	}
}

func storeLoggerState(state loggerState) {
	setDefaultSlog(state.base)
	logState.Store(state)
}

func currentLoggerState() loggerState {
	if state, ok := logState.Load().(loggerState); ok {
		return state
	}

	logMu.Lock()
	defer logMu.Unlock()

	if state, ok := logState.Load().(loggerState); ok {
		return state
	}

	state := newLoggerState(kitlog.LevelInfo, kitlog.FormatText)
	storeLoggerState(state)
	return state
}

// InitLogger sets the global logger based on level/format strings.
func InitLogger(levelRaw, formatRaw string) (*kitlog.Helper, error) {
	level, err := kitlog.ParseLevel(levelRaw)
	if err != nil {
		return nil, err
	}
	format, err := kitlog.ParseFormat(formatRaw)
	if err != nil {
		return nil, err
	}
	state := newLoggerState(level, format)

	logMu.Lock()
	storeLoggerState(state)
	logMu.Unlock()

	return state.log, nil
}

// Log returns the global logger.
func Log() *kitlog.Helper {
	return currentLoggerState().log
}

// Slog returns the global base slog logger.
func Slog() *slog.Logger {
	return currentLoggerState().base
}

// LogLevel returns the configured log level.
func LogLevel() kitlog.Level {
	return currentLoggerState().level
}

// DebugEnabled reports whether debug logging is active.
func DebugEnabled() bool {
	return Slog().Enabled(context.Background(), slog.LevelDebug)
}

// WithModule returns a logger pre-tagged with module name.
func WithModule(name string) *kitlog.Helper {
	name = strings.TrimSpace(name)
	if name == "" {
		return Log()
	}
	return kitlog.NewHelper(SlogWithModule(name))
}

// SlogWithModule returns a base slog logger pre-tagged with module name.
func SlogWithModule(name string) *slog.Logger {
	name = strings.TrimSpace(name)
	if name == "" {
		return Slog()
	}
	return Slog().With(slog.String("module", name))
}

func setDefaultSlog(logger *slog.Logger) {
	if logger == nil {
		return
	}
	slog.SetDefault(logger)
}

// Fatal logs an error message and exits with code 1.
// Use for startup failures that should terminate the program.
func Fatal(msg string, err error, attrs ...slog.Attr) {
	Log().Error(msg, err, attrs...)
	os.Exit(1)
}
