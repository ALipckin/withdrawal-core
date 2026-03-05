package logger

import (
	"context"
	"log/slog"
	"os"
)

type StructuredLogger interface {
	Info(ctx context.Context, msg string, attrs ...any)
	Error(ctx context.Context, msg string, attrs ...any)
}

type SlogLogger struct {
	logger *slog.Logger
}

func New() *SlogLogger {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	return &SlogLogger{logger: slog.New(handler)}
}

func (l *SlogLogger) Info(ctx context.Context, msg string, attrs ...any) {
	l.logger.Log(ctx, slog.LevelInfo, msg, attrs...)
}

func (l *SlogLogger) Error(ctx context.Context, msg string, attrs ...any) {
	l.logger.Log(ctx, slog.LevelError, msg, attrs...)
}
