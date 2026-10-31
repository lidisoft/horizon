// Package logging builds zerolog loggers with a "system" field, as in tocsin.
package logging

import (
	"io"
	"os"

	"github.com/rs/zerolog"
)

const fieldSystem = "system"

// New returns a logger writing JSON (or human-friendly console output when
// format is "console") at level, tagged with system.
func New(level zerolog.Level, format, system string) zerolog.Logger {
	return NewTo(os.Stdout, level, format, system)
}

// NewTo is New with an explicit writer, for tests.
func NewTo(w io.Writer, level zerolog.Level, format, system string) zerolog.Logger {
	if format == "console" {
		w = zerolog.ConsoleWriter{Out: w}
	}
	return zerolog.New(w).Level(level).With().Timestamp().Str(fieldSystem, system).Logger()
}

// WithSystem returns a child logger tagged with another system.
func WithSystem(log zerolog.Logger, system string) zerolog.Logger {
	return log.With().Str(fieldSystem, system).Logger()
}
