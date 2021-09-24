package zapsentry

import (
	"errors"
	"time"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// defaults are the sane defaults for zapsentry configuration
var defaults = &config{
	platform:     "Golang",
	level:        zapcore.ErrorLevel,
	flushTimeout: 5 * time.Second,
	tags:         make(map[string]string),
}

// Config is a minimal set of parameters for Sentry integration.
type config struct {
	level             zapcore.Level
	flushTimeout      time.Duration
	tags              map[string]string
	disableStacktrace bool
	environment       string
	platform          string
}

type Option func(c *core) error

func Level(lvl zapcore.Level) Option {
	return func(c *core) error {
		c.cfg.level = lvl
		return nil
	}
}

func WithTags(tags map[string]string) Option {
	return func(c *core) error {
		c.cfg.tags = tags
		return nil
	}
}

func WithEnvironment(env string) Option {
	return func(c *core) error {
		c.cfg.environment = env
		return nil
	}
}

func UseStacktraceFrameFilter(ff StacktraceFrameFilter) Option {
	return func(c *core) error {
		if c.cfg.disableStacktrace {
			return errors.New("stacktrace disabled, don't pass stacktrace frame filter opt")
		}
		c.stacktraceFrameFilter = ff
		return nil
	}
}

func DisableStacktrace() Option {
	return func(c *core) error {
		c.cfg.disableStacktrace = true
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
		c.cfg.flushTimeout = after
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
		c.registeredTagKeys = make(map[string]byte, len(keys))
		for _, k := range keys {
			c.registeredTagKeys[k] = 1
		}
		return nil
	}
}
