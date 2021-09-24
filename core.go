package zapsentry

import (
	"errors"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	maxLimit = 1000

	zapSentryScopeKey = "_zapsentry_scope_"
)

func NewScope() zapcore.Field {
	f := zap.Skip()
	f.Interface = sentry.NewScope()
	f.Key = zapSentryScopeKey

	return f
}

var _ zapcore.Core = (*core)(nil)

type core struct {
	zapcore.LevelEnabler
	breadcrumbs *breadcrumbs
	cfg         *config

	client      *sentry.Client
	sentryHub   *sentry.Hub
	sentryScope *sentry.Scope

	stacktraceFrameFilter StacktraceFrameFilter
	exceptionProvider     ExceptionProvider

	fields            map[string]interface{}
	registeredTagKeys map[string]byte
}

func NewCore(factory SentryClientFactory, opts ...Option) (zapcore.Core, error) {
	client, err := factory()
	if err != nil {
		return zapcore.NewNopCore(), err
	}

	core := &core{
		client:                client,
		fields:                make(map[string]interface{}),
		cfg:                   defaults,
		breadcrumbs:           newBreadcrumbs(),
		registeredTagKeys:     make(map[string]byte),
		stacktraceFrameFilter: &DefaultStacktraceFrameFilter{},
		exceptionProvider:     nopExceptionProvider,
	}
	for _, o := range opts {
		err := o(core)
		if err != nil {
			return zapcore.NewNopCore(), err
		}
	}

	if core.breadcrumbs.enabled && core.breadcrumbs.level > core.cfg.level {
		return zapcore.NewNopCore(), errors.New("breadcrumb level must be lower than error level")
	}
	core.LevelEnabler = &LevelEnabler{
		level:       core.cfg.level,
		breadcrumbs: core.breadcrumbs,
	}

	if !core.cfg.disableStacktrace {
		core.exceptionProvider = NewExceptionProvider(core.stacktraceFrameFilter)
	}

	return core, nil
}

func (c *core) With(fs []zapcore.Field) zapcore.Core {
	return c.with(fs)
}

func (c *core) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.LevelEnabler.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *core) Write(ent zapcore.Entry, fs []zapcore.Field) error {
	clone := c.with(fs)

	/*
		only when we have local sentryScope to avoid collecting all breadcrumbs ever in a global scope
	*/
	if c.breadcrumbs.Enabled(ent.Level) && c.sentryScope != nil {
		breadcrumb := c.breadcrumbs.new(ent, clone.fields)
		c.sentryScope.AddBreadcrumb(breadcrumb, maxLimit)
	}
	tags := c.tagsFromFields(fs)
	for k, v := range c.cfg.tags {
		tags[k] = v
	}

	if ent.Level.Enabled(c.cfg.level) {
		event := sentry.NewEvent()
		event.Message = ent.Message
		event.Timestamp = ent.Time
		event.Level = zapToSentryLevel(ent.Level)
		event.Platform = "Golang"
		event.Extra = clone.fields
		event.Exception = c.exceptionProvider.Exception(ent)
		if c.cfg.environment != "" {
			event.Environment = c.cfg.environment
		}

		_ = c.client.CaptureEvent(event, nil, c.scope())
	}

	// We may be crashing the program, so should flush any buffered events.
	if ent.Level > zapcore.ErrorLevel {
		// revive:disable-next-line:unhandled-error we always return nil here so we don't heed to handle it
		c.Sync()
	}
	return nil
}

func (c *core) Sync() error {
	c.client.Flush(c.cfg.flushTimeout)
	return nil
}

func (c *core) tagsFromFields(fs []zapcore.Field) map[string]string {
	tags := make(map[string]string)
	for _, f := range fs {
		if f.Type != zapcore.StringType {
			continue
		}
		if _, ok := c.registeredTagKeys[f.Key]; !ok {
			continue
		}
		tags[f.Key] = f.String
	}
	return tags
}

func (c *core) hub() *sentry.Hub {
	if c.sentryHub != nil {
		return c.sentryHub
	}
	return sentry.CurrentHub()
}

func (c *core) scope() *sentry.Scope {
	if c.sentryScope != nil {
		return c.sentryScope
	}
	return c.hub().Scope()
}

func (c *core) findScope(fs []zapcore.Field) *sentry.Scope {
	for _, f := range fs {
		if s := getScope(f); s != nil {
			return s
		}
	}
	return c.sentryScope
}

func getScope(field zapcore.Field) *sentry.Scope {
	if field.Type == zapcore.SkipType {
		if scope, ok := field.Interface.(*sentry.Scope); ok && field.Key == zapSentryScopeKey {
			return scope
		}
	}

	return nil
}

func (c *core) with(fs []zapcore.Field) *core {
	// Copy our map.
	m := make(map[string]interface{}, len(c.fields))
	for k, v := range c.fields {
		m[k] = v
	}

	// Add fields to an in-memory encoder.
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fs {
		f.AddTo(enc)
	}

	// Merge the two maps.
	for k, v := range enc.Fields {
		m[k] = v
	}

	return &core{
		client:                c.client,
		cfg:                   c.cfg,
		fields:                m,
		breadcrumbs:           c.breadcrumbs,
		LevelEnabler:          c.LevelEnabler,
		sentryScope:           c.findScope(fs),
		stacktraceFrameFilter: c.stacktraceFrameFilter,
		exceptionProvider:     c.exceptionProvider,
		registeredTagKeys:     c.registeredTagKeys,
	}
}
