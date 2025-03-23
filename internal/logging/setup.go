package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

func Configure(
	logLevel string,
	logFormat string,
	out io.Writer,
) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		AddSource: true,
	}
	switch logLevel {
	case "debug":
		opts.Level = slog.LevelDebug
	case "info":
		opts.Level = slog.LevelInfo
	case "warn", "warning":
		opts.Level = slog.LevelWarn
	case "error", "critical":
		opts.Level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: %s", logLevel)
	}
	var handler slog.Handler
	switch logFormat {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		return nil, fmt.Errorf("invalid log format: %s", logFormat)
	}
	return slog.New(handler), nil
}
