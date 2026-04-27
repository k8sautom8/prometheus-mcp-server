package prommcpserver

import (
	"log/slog"
	"os"
)

// NewLogger returns a JSON logger writing to stderr, similar to the previous structlog setup.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
