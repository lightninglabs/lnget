package build

import (
	"os"

	"github.com/btcsuite/btclog/v2"
)

// LogType indicates the type of logging specified by the build flag.
type LogType byte

const (
	// LogTypeNone indicates no logging.
	LogTypeNone LogType = iota

	// LogTypeStdOut indicates all logging is written directly to stdout.
	LogTypeStdOut

	// LogTypeDefault logs to both stdout and a given io.PipeWriter.
	LogTypeDefault
)

// String returns a human readable identifier for the logging type.
func (t LogType) String() string {
	switch t {
	case LogTypeNone:
		return "none"

	case LogTypeStdOut:
		return "stdout"

	case LogTypeDefault:
		return "default"

	default:
		return "unknown"
	}
}

// NewSubLogger constructs a new subsystem logger. For lnget, we use a simple
// stdout-based logger.
func NewSubLogger(subsystem string, gen func(string) btclog.Logger) btclog.Logger {
	if gen != nil {
		return gen(subsystem)
	}

	// Default to stdout logging.
	backend := btclog.NewDefaultHandler(os.Stdout)
	logger := btclog.NewSLogger(backend.SubSystem(subsystem))

	return logger
}

// NewDefaultLogger creates a default logger that writes to stdout.
func NewDefaultLogger(subsystem string) btclog.Logger {
	backend := btclog.NewDefaultHandler(os.Stdout)
	return btclog.NewSLogger(backend.SubSystem(subsystem))
}

// SetLogLevel sets the log level for a logger.
func SetLogLevel(logger btclog.Logger, level string) {
	lvl, ok := btclog.LevelFromString(level)
	if !ok {
		return
	}

	logger.SetLevel(lvl)
}
