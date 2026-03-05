package service

import "context"

type noopLogger struct{}

func (n noopLogger) Info(_ context.Context, _ string, _ ...any) {}

func (n noopLogger) Error(_ context.Context, _ string, _ ...any) {}
