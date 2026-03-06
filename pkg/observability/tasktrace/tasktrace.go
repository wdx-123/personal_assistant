package tasktrace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"personal_assistant/pkg/observability/contextid"
	obstrace "personal_assistant/pkg/observability/trace"
	"personal_assistant/pkg/redislock"
)

// Options 定义定时任务 tracing 的可配置项。
type Options struct {
	Backend     obstrace.TraceBackend
	ServiceName string
	Kind        string
	Trigger     string
	Tags        map[string]string
	LockEnabled bool
	LockKey     string
	LockTTL     time.Duration
}

type taskLocker interface {
	TryLock() error
	Unlock() error
}

var newTaskLocker = func(ctx context.Context, key string, ttl time.Duration) taskLocker {
	return redislock.NewRedisLock(ctx, key, ttl)
}

// Wrap 为定时任务提供统一的 root span 包装。
func Wrap(name string, opt Options, fn func(context.Context) error) func() {
	return func() {
		ctx := context.Background()
		ctx, _ = contextid.EnsureIDs(ctx)

		spanCtx := ctx
		var spanEvent *obstrace.SpanEvent
		if opt.Backend != nil {
			spanCtx, spanEvent = obstrace.StartSpan(ctx, obstrace.StartOptions{
				Service: strings.TrimSpace(opt.ServiceName),
				Stage:   "task",
				Name:    strings.TrimSpace(name),
				Kind:    resolveKind(opt.Kind),
				Tags:    buildTags(name, opt),
			})
		}

		finalTags := map[string]string{}
		status := obstrace.SpanStatusOK
		code := ""
		message := ""
		var finalErr error
		finishNow := func() {
			finish(spanCtx, strings.TrimSpace(name), opt, spanEvent, status, code, message, finalTags, finalErr)
		}

		var lock taskLocker
		if opt.LockEnabled {
			finalTags["lock_enabled"] = "true"
			lockKey := strings.TrimSpace(opt.LockKey)
			if lockKey == "" {
				finalTags["lock_acquired"] = "false"
				finalTags["execution_result"] = "error"
				status = obstrace.SpanStatusError
				code = "task_lock_error"
				message = "task lock key is empty"
				finalErr = errors.New(message)
				finishNow()
				return
			}

			lock = newTaskLocker(spanCtx, lockKey, opt.LockTTL)
			if err := lock.TryLock(); err != nil {
				finalTags["lock_acquired"] = "false"
				if errors.Is(err, redislock.ErrLockFailed) {
					finalTags["execution_result"] = "skipped"
					message = "lock not acquired"
					finishNow()
					return
				}

				finalTags["execution_result"] = "error"
				status = obstrace.SpanStatusError
				code = "task_lock_error"
				message = err.Error()
				finalErr = err
				finishNow()
				return
			}
			finalTags["lock_acquired"] = "true"
		} else {
			finalTags["lock_enabled"] = "false"
		}

		defer func() {
			if lock != nil {
				if err := lock.Unlock(); err != nil {
					status = obstrace.SpanStatusError
					code = "task_lock_release_error"
					message = err.Error()
					finalErr = err
					finalTags["execution_result"] = "error"
				}
			}
			finishNow()
		}()

		if fn != nil {
			finalErr = fn(spanCtx)
		}
		if finalErr != nil {
			status = obstrace.SpanStatusError
			code = "task_error"
			message = finalErr.Error()
			finalTags["execution_result"] = "error"
			return
		}
		finalTags["execution_result"] = "success"
	}
}

func resolveKind(kind string) string {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return "cron"
	}
	return kind
}

// buildTags 构建定时任务的 tags。
func buildTags(name string, opt Options) map[string]string {
	tags := map[string]string{
		"task": strings.TrimSpace(name),
	}
	trigger := strings.TrimSpace(opt.Trigger)
	if trigger == "" {
		trigger = "cron"
	}
	tags["trigger"] = trigger
	for k, v := range opt.Tags {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		tags[k] = strings.TrimSpace(v)
	}
	return tags
}

// finish 结束定时任务的 root span。
func finish(
	ctx context.Context,
	name string,
	opt Options,
	spanEvent *obstrace.SpanEvent,
	status string,
	code string,
	message string,
	tags map[string]string,
	err error,
) {
	if spanEvent == nil || opt.Backend == nil {
		return
	}
	if err != nil {
		spanEvent.WithErrorDetail(buildErrorDetail(name, err))
	}
	span := spanEvent.End(status, code, message, tags)
	_ = opt.Backend.RecordSpan(ctx, span)
}

// buildErrorDetail 构建错误详情。
func buildErrorDetail(name string, err error) string {
	payload := map[string]string{
		"task":  strings.TrimSpace(name),
		"stage": "task",
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
