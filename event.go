package zapsentry

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

const defaultPlatform = "Golang"

// Tagger allows adding custom tags on datatypes
type Tagger interface {
	// Tags returns custom defined tags for the type
	Tags() map[string]string
}

type events struct {
	environment       string
	platform          string
	registeredTagKeys map[string]byte

	disabledStacktrace    bool
	stackTraceFrameFilter StacktraceFrameFilter
	exceptionProvider     ExceptionProvider

	tags map[string]string
}

func newEvents() *events {
	return &events{
		platform:              defaults.platform,
		environment:           defaults.environment,
		disabledStacktrace:    defaults.disableStacktrace,
		stackTraceFrameFilter: defaults.stackTraceFrameFilter,
		exceptionProvider:     defaults.exceptionProvider,
		registeredTagKeys:     make(map[string]byte),
		tags:                  make(map[string]string),
	}
}

func (e *events) new(
	ent zapcore.Entry,
	fs []zapcore.Field,
	extra map[string]interface{},
) *sentry.Event {
	event := sentry.NewEvent()
	event.Message = ent.Message
	event.Timestamp = ent.Time
	event.Level = zapToSentryLevel(ent.Level)
	event.Extra = extra
	event.Platform = e.platform
	event.Exception = e.exceptionProvider.Exception(ent)
	if e.environment != "" {
		event.Environment = e.environment
	}

	tags := e.tagsFromFields(fs)
	for k, v := range e.tags {
		tags[k] = v
	}
	event.Tags = tags
	return event
}

func (e *events) tagsFromFields(fs []zapcore.Field) map[string]string {
	tags := make(map[string]string)
	for _, f := range fs {
		if _, ok := e.registeredTagKeys[f.Key]; !ok {
			continue
		}
		if f.Type == zapcore.ObjectMarshalerType {
			tags = e.tryAddObjectTag(f, tags)
			continue
		}
		if f.Type == zapcore.StringType {
			tags[f.Key] = f.String
			continue
		}
		if f.Type == zapcore.ReflectType {
			tags = e.tryAddObjectTag(f, tags)
			continue
		}
	}
	return tags
}

func (e *events) tryAddObjectTag(field zapcore.Field, tags map[string]string) map[string]string {
	if field.Interface == nil {
		return tags
	}
	cast, ok := field.Interface.(Tagger)
	if !ok {
		return tags
	}
	objectTags := cast.Tags()
	if objectTags == nil {
		return tags
	}
	for k, v := range objectTags {
		tags[k] = v
	}
	return tags
}
