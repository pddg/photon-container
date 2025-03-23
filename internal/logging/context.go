package logging

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	loggerKey ctxKey = "logger"
)

func FromContext(ctx context.Context) *slog.Logger {
	logger := ctx.Value(loggerKey)
	if logger == nil {
		return slog.Default()
	}
	return logger.(*slog.Logger)
}

func NewContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}
