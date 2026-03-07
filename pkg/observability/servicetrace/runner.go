package servicetrace

import (
	"context"
	"encoding/json"
	"strings"

	obstrace "personal_assistant/pkg/observability/trace"
)

type Options struct {
	Backend     obstrace.TraceBackend
	ServiceName string
	Stage       string
	Module      string
	Method      string
}

func Start(ctx context.Context, opt Options) (context.Context, *obstrace.SpanEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opt.Backend == nil {
		return ctx, nil
	}

	stage := strings.TrimSpace(opt.Stage)
	if stage == "" {
		stage = "service"
	}
	serviceName := strings.TrimSpace(opt.ServiceName)
	if serviceName == "" {
		serviceName = "unknown_service"
	}
	module := strings.TrimSpace(opt.Module)
	method := strings.TrimSpace(opt.Method)

	return obstrace.StartSpan(ctx, obstrace.StartOptions{
		Service: serviceName,
		Stage:   stage,
		Name:    stage + "." + module + "." + method,
		Kind:    "internal",
		Tags: map[string]string{
			"module": module,
			"method": method,
		},
	})
}

func Finish(ctx context.Context, opt Options, spanEvent *obstrace.SpanEvent, err error) {
	if spanEvent == nil || opt.Backend == nil {
		return
	}

	status := obstrace.SpanStatusOK
	code := ""
	message := ""
	if err != nil {
		status = obstrace.SpanStatusError
		code = "service_error"
		message = err.Error()
		spanEvent.WithErrorDetail(buildErrorDetail(opt.Module, opt.Method, err))
	}

	span := spanEvent.End(status, code, message, nil)
	if span != nil {
		_ = opt.Backend.RecordSpan(ctx, span)
	}
}

func Run[T any](ctx context.Context, opt Options, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if fn == nil {
		return zero, nil
	}
	spanCtx, spanEvent := Start(ctx, opt)
	out, err := fn(spanCtx)
	Finish(spanCtx, opt, spanEvent, err)
	return out, err
}

func RunErr(ctx context.Context, opt Options, fn func(context.Context) error) error {
	_, err := Run(ctx, opt, func(inner context.Context) (struct{}, error) {
		if fn == nil {
			return struct{}{}, nil
		}
		return struct{}{}, fn(inner)
	})
	return err
}

func buildErrorDetail(module, method string, err error) string {
	payload := map[string]string{
		"module": strings.TrimSpace(module),
		"method": strings.TrimSpace(method),
	}
	if err != nil {
		payload["error"] = strings.TrimSpace(err.Error())
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return ""
	}
	return string(data)
}
