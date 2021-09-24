package zapsentry

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

const (
	breadcrumbTypeDefault = "default"
	breadcrumbTypeInfo    = "info"
	breadcrumbTypeDebug   = "debug"
)

func NewBreadcrumb(ent zapcore.Entry, data map[string]interface{}) *sentry.Breadcrumb {
	return &sentry.Breadcrumb{
		Data:      data,
		Message:   ent.Message,
		Level:     zapToSentryLevel(ent.Level),
		Type:      zapLevelToBreadcrumbType(ent.Level),
		Timestamp: ent.Time,
	}
}

func zapLevelToBreadcrumbType(lvl zapcore.Level) string {
	switch lvl {
	case zapcore.InfoLevel:
		return breadcrumbTypeInfo
	case zapcore.DebugLevel:
		return breadcrumbTypeDebug
	default:
		return breadcrumbTypeDefault
	}
}
