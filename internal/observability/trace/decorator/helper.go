package decorator

import (
	"context"
	"strings"

	"personal_assistant/global"
	"personal_assistant/pkg/observability/servicetrace"
)

func newTraceOptions(module, method string) servicetrace.Options {
	return servicetrace.Options{
		Backend:     global.ObservabilityTraces,
		ServiceName: resolveServiceName(),
		Stage:       "service",
		Module:      strings.TrimSpace(module),
		Method:      strings.TrimSpace(method),
	}
}

func runTraced[T any](ctx context.Context, module, method string, fn func(context.Context) (T, error)) (T, error) {
	return servicetrace.Run(ctx, newTraceOptions(module, method), fn)
}

func runTracedErr(ctx context.Context, module, method string, fn func(context.Context) error) error {
	return servicetrace.RunErr(ctx, newTraceOptions(module, method), fn)
}

func resolveServiceName() string {
	if global.Config == nil {
		return "personal_assistant"
	}
	if v := strings.TrimSpace(global.Config.Observability.ServiceName); v != "" {
		return v
	}
	return "personal_assistant"
}
