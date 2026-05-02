package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// New returns a zerolog.Logger configured from a string level (debug/info/warn/error).
func New(level string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil || lvl == zerolog.NoLevel {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Str("service", "auditrail").
		Logger()
}
