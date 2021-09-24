package zapsentry

import (
	"errors"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// defaults are the sane defaults for zapsentry configuration
var defaults = &config{
	level:                 zapcore.ErrorLevel,
	flushTimeout:          5 * time.Second,
	platform:              "Golang",
	disableStacktrace:     false,
	stackTraceFrameFilter: &DefaultStacktraceFrameFilter{},
	exceptionProvider:     nopExceptionProvider,
}

// Config is a minimal set of parameters for Sentry integration.
type config struct {
	level        zapcore.Level
	flushTimeout time.Duration

	platform    string
	environment string

	disableStacktrace     bool
	stackTraceFrameFilter StacktraceFrameFilter
	exceptionProvider     ExceptionProvider
}

type Option func(c *core) error

func Level(lvl zapcore.Level) Option {
	return func(c *core) error {
		c.level = lvl
		return nil
	}
}

func WithTags(tags map[string]string) Option {
	return func(c *core) error {
		c.events.tags = tags
		return nil
	}
}

func WithEnvironment(env string) Option {
	return func(c *core) error {
		c.events.environment = env
		return nil
	}
}

func WithPlaform(platform string) Option {
	return func(c *core) error {
		c.events.platform = platform
		return nil
	}
}

func UseStacktraceFrameFilter(ff StacktraceFrameFilter) Option {
	return func(c *core) error {
		if c.events.disabledStacktrace {
			return errors.New("stacktrace disabled, don't pass stacktrace frame filter opt")
		}
		c.events.stackTraceFrameFilter = ff
		return nil
	}
}

func DisableStacktrace() Option {
	return func(c *core) error {
		c.events.disabledStacktrace = true
		c.events.exceptionProvider = nopExceptionProvider
		return nil
	}
}

func WithBreadcrumbs(level zapcore.Level) Option {
	return func(c *core) error {
		c.breadcrumbs.enabled = true
		c.breadcrumbs.level = level
		return nil
	}
}

func WithGlobalBreadcrumbs() Option {
	return func(c *core) error {
		c.breadcrumbs.localOnly = false
		return nil
	}
}

func WithFlushTimeout(after time.Duration) Option {
	return func(c *core) error {
		if after == 0 {
			return errors.New("flush timeout can't be 0")
		}
		c.flushTimeout = after
		return nil
	}
}

func UseHub(hub *sentry.Hub) Option {
	return func(c *core) error {
		c.sentryHub = hub
		return nil
	}
}

func ConvertFieldsToTags(keys ...string) Option {
	return func(c *core) error {
		c.events.registeredTagKeys = make(map[string]byte, len(keys))
		for _, k := range keys {
			c.events.registeredTagKeys[k] = 1
		}
		return nil
	}
}
