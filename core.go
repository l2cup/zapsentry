package zapsentry

import (
	"errors"
	"time"

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

	level        zapcore.Level
	flushTimeout time.Duration

	events      *events
	breadcrumbs *breadcrumbs

	client      *sentry.Client
	sentryHub   *sentry.Hub
	sentryScope *sentry.Scope

	fields map[string]interface{}
}

func NewCore(factory SentryClientFactory, opts ...Option) (zapcore.Core, error) {
	client, err := factory()
	if err != nil {
		return zapcore.NewNopCore(), err
	}

	core := &core{
		client:       client,
		flushTimeout: defaults.flushTimeout,
		level:        defaults.level,
		fields:       make(map[string]interface{}),
		breadcrumbs:  newBreadcrumbs(),
		events:       newEvents(),
	}
	for _, o := range opts {
		err := o(core)
		if err != nil {
			return zapcore.NewNopCore(), err
		}
	}

	if core.breadcrumbs.enabled && core.breadcrumbs.level > core.level {
		return zapcore.NewNopCore(), errors.New("breadcrumb level must be lower than error level")
	}
	core.LevelEnabler = &LevelEnabler{
		level:       core.level,
		breadcrumbs: core.breadcrumbs,
	}

	if !core.events.disabledStacktrace {
		core.events.exceptionProvider = NewExceptionProvider(core.events.stackTraceFrameFilter)
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

	// only when we have local sentryScope to avoid collecting all breadcrumbs ever in a global scope
	if c.breadcrumbs.Enabled(ent.Level) && c.sentryScope != nil {
		breadcrumb := c.breadcrumbs.new(ent, clone.fields)
		c.sentryScope.AddBreadcrumb(breadcrumb, maxLimit)
	}

	if c.level.Enabled(ent.Level) {
		event := c.events.new(ent, fs, clone.fields)
		_ = c.client.CaptureEvent(event, nil, c.scope())
	}

	// We may be crashing the program, so should flush any buffered events.
	if ent.Level > zapcore.ErrorLevel {
		// revive:disable-next-line:unhandled-error *
		// We always return nil here so we don't heed to handle it
		c.Sync()
	}
	return nil
}

func (c *core) Sync() error {
	c.client.Flush(c.flushTimeout)
	return nil
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
		LevelEnabler: c.LevelEnabler,
		breadcrumbs:  c.breadcrumbs,
		events:       c.events,
		client:       c.client,
		sentryScope:  c.findScope(fs),
		level:        c.level,
		flushTimeout: c.flushTimeout,
		fields:       m,
	}
}
