package zapsentry

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	maxLimit = 1000

	zapSentryScopeKey = "_zapsentry_scope_"
	zapSentryHubKey   = "_zapsentry_hub_"
)

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
	sentry.CurrentHub().BindClient(client)

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
		clone.hub().Scope().AddBreadcrumb(breadcrumb, maxLimit)
	}

	if c.level.Enabled(ent.Level) {
		event := c.events.new(ent, fs, clone.fields)
		_ = clone.hub().CaptureEvent(event)
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

func (c *core) findScope(fs []zapcore.Field) *sentry.Scope {
	for _, f := range fs {
		if s := getScope(f); s != nil {
			return s
		}
	}
	return c.scope()
}

func (c *core) findHub(fs []zapcore.Field) (*sentry.Hub, bool) {
	for _, f := range fs {
		if h := getHub(f); h != nil {
			return h, true
		}
	}
	return c.hub(), false
}

func getScope(field zapcore.Field) *sentry.Scope {
	if field.Type == zapcore.SkipType && field.Key == zapSentryScopeKey {
		if scope, ok := field.Interface.(*sentry.Scope); ok {
			return scope
		}
	}
	return nil
}

func getHub(field zapcore.Field) *sentry.Hub {
	if field.Type == zapcore.SkipType && field.Key == zapSentryHubKey {
		if hub, ok := field.Interface.(*sentry.Hub); ok {
			log.Println("in hub casting", fmt.Sprintf("type %T", hub))
			return hub
		}
	}
	return nil
}

func (c *core) hub() *sentry.Hub {
	if c.sentryHub != nil {
		return c.sentryHub
	}
	return sentry.CurrentHub().Clone()
}

func (c *core) scope() *sentry.Scope {
	if c.sentryScope != nil {
		return c.sentryScope
	}
	return c.hub().Scope()
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

	scope := c.findScope(fs)
	hub, found := c.findHub(fs)
	if !found {
		if c.sentryHub == nil {
			hub = sentry.NewHub(c.client, scope)
		}
		hub = c.sentryHub
	}

	return &core{
		LevelEnabler: c.LevelEnabler,
		breadcrumbs:  c.breadcrumbs,
		events:       c.events,
		client:       c.client,
		sentryScope:  scope,
		sentryHub:    hub,
		level:        c.level,
		flushTimeout: c.flushTimeout,
		fields:       m,
	}
}

func NewScope() zapcore.Field {
	f := zap.Skip()
	f.Interface = sentry.NewScope()
	f.Key = zapSentryScopeKey
	return f
}

func WrapHub(hub *sentry.Hub) zapcore.Field {
	f := zap.Skip()
	f.Interface = hub
	f.Key = zapSentryHubKey
	return f
}

func WrapScope(scope *sentry.Scope) zapcore.Field {
	f := zap.Skip()
	f.Interface = scope
	f.Key = zapSentryScopeKey
	return f
}
