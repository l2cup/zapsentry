package zapsentry

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// These constants define breadcrumb types.
//
// https://develop.sentry.dev/sdk/event-payloads/breadcrumbs/#breadcrumb-types
const (
	// breadcrumbTypeDefault is the sentry's default breadcrumb type
	breadcrumbTypeDefault = "default"
	// breadcrumbTypeDefault is the sentry's info breadcrumb type
	breadcrumbTypeInfo = "info"
	// breadcrumbTypeDefault is the sentry's debug breadcrumb type
	breadcrumbTypeDebug = "debug"
)

// breadcrumbs allows creating breadcrumbs
type breadcrumbs struct {
	// enabled is true if breadcrumbs are enabled
	enabled bool

	// level is the level after which breadcrumbs will be added
	level zapcore.Level

	// localOnly
	localOnly bool
}

// newBreadcrumbs returns new breadcrumbs with default settings.
func newBreadcrumbs() *breadcrumbs {
	return &breadcrumbs{localOnly: true}
}

// Enabled returns true if the given level is at or above the breadcrumbs level.
// It also checks if breadcrumbs are enabled.
func (bc *breadcrumbs) Enabled(lvl zapcore.Level) bool {
	return bc.enabled && bc.level.Enabled(lvl)
}

// new returns a new sentry Breadcrumb from the passed zapcore.Entry and data.
func (bc *breadcrumbs) new(ent zapcore.Entry, data map[string]interface{}) *sentry.Breadcrumb {
	return &sentry.Breadcrumb{
		Data:      data,
		Message:   ent.Message,
		Level:     zapToSentryLevel(ent.Level),
		Type:      zapLevelToBreadcrumbType(ent.Level),
		Timestamp: ent.Time,
	}
}

// zapLevelToBreadcrumbType maps zap's Level to Sentry's breadcrumb type.
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
