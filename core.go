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
	client *sentry.Client
	cfg    *Configuration
	zapcore.LevelEnabler
	flushTimeout time.Duration

	sentryScope       *sentry.Scope
	exceptionProvider ExceptionProvider

	fields            map[string]interface{}
	registeredTagKeys map[string]byte
}

func NewCore(cfg Configuration, factory SentryClientFactory) (zapcore.Core, error) {
	client, err := factory()
	if err != nil {
		return zapcore.NewNopCore(), err
	}

	if cfg.EnableBreadcrumbs && cfg.BreadcrumbLevel > cfg.Level {
		return zapcore.NewNopCore(), errors.New("breadcrumb level must be lower than error level")
	}

	core := core{
		client: client,
		cfg:    &cfg,
		LevelEnabler: &LevelEnabler{
			Level:             cfg.Level,
			breadcrumbsLevel:  cfg.BreadcrumbLevel,
			enableBreadcrumbs: cfg.EnableBreadcrumbs,
		},
		flushTimeout:      5 * time.Second,
		fields:            make(map[string]interface{}),
		exceptionProvider: nopExceptionProvider,
	}

	if !cfg.DisableStacktrace {
		var ff StacktraceFrameFilter = &DefaultStacktraceFrameFilter{}
		if cfg.StacktraceFrameFilter != nil {
			ff = cfg.StacktraceFrameFilter
		}
		core.exceptionProvider = NewExceptionProvider(ff)
	}

	if cfg.FlushTimeout > 0 {
		core.flushTimeout = cfg.FlushTimeout
	}

	return &core, nil
}

func (c *core) With(fs []zapcore.Field) zapcore.Core {
	return c.with(fs)
}

func (c *core) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.cfg.EnableBreadcrumbs && c.cfg.BreadcrumbLevel.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	if c.cfg.Level.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

func (c *core) Write(ent zapcore.Entry, fs []zapcore.Field) error {
	clone := c.with(fs)

	/*
		only when we have local sentryScope to avoid collecting all breadcrumbs ever in a global scope
	*/
	if c.cfg.EnableBreadcrumbs && c.cfg.BreadcrumbLevel.Enabled(ent.Level) && c.sentryScope != nil {
		breadcrumb := NewBreadcrumb(ent, clone.fields)
		c.sentryScope.AddBreadcrumb(breadcrumb, maxLimit)
	}
	tags := c.tagsFromFields(fs)
	for k, v := range c.cfg.Tags {
		tags[k] = v
	}

	if ent.Level.Enabled(c.cfg.Level) {
		event := sentry.NewEvent()
		event.Message = ent.Message
		event.Timestamp = ent.Time
		event.Level = zapToSentryLevel(ent.Level)
		event.Platform = "Golang"
		event.Extra = clone.fields
		event.Exception = c.exceptionProvider.Exception(ent)
		if c.cfg.Environment != "" {
			event.Environment = c.cfg.Environment
		}

		_ = c.client.CaptureEvent(event, nil, c.scope())
	}

	// We may be crashing the program, so should flush any buffered events.
	if ent.Level > zapcore.ErrorLevel {
		c.client.Flush(c.flushTimeout)
	}
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
	if c.cfg.Hub != nil {
		return c.cfg.Hub
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

func (c *core) Sync() error {
	c.client.Flush(c.flushTimeout)
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
		client:       c.client,
		cfg:          c.cfg,
		flushTimeout: c.flushTimeout,
		fields:       m,
		LevelEnabler: c.LevelEnabler,
		sentryScope:  c.findScope(fs),
	}
}
