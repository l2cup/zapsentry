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
	// breadcrumbTypeInfo is the sentry's info breadcrumb type
	breadcrumbTypeInfo = "info"
	// breadcrumbTypeDebug is the sentry's debug breadcrumb type
	breadcrumbTypeDebug = "debug"
	// breadcrumbTypeError is the sentry's error breadcrumb type
	breadcrumbTypeError = "error"
	// breadcrumbTypeWarn is the sentry's warn breadcrumb type
	breadcrumbTypeWarn = "info"
	// breadcrumbTypeFatal is the sentry's fatal breadcrumb type
	breadcrumbTypeFatal = "error"
)

// These constants define breadcrumb categories.
const (
	// breadcrumbCategoryInfo is the sentry's info breadcrumb category
	breadcrumbCategoryInfo = "info"
	// breadcrumbCategoryDebug is the sentry's warn breadcrumb category
	breadcrumbCategoryDebug = "debug"
	// breadcrumbCategoryWarn is the sentry's warn breadcrumb category
	breadcrumbCategoryWarn = "warning"
	// breadcrumbCategoryError is the sentry's error breadcrumb category
	breadcrumbCategoryError = "error"
	// breadcrumbCategoryFatal is the sentry's fatal breadcrumb category
	breadcrumbCategoryFatal = "fatal"
)

var levelsToBreadcrumbTypes = map[zapcore.Level]string{
	zapcore.DebugLevel:  breadcrumbTypeDebug,
	zapcore.InfoLevel:   breadcrumbTypeInfo,
	zapcore.WarnLevel:   breadcrumbTypeWarn,
	zapcore.ErrorLevel:  breadcrumbTypeError,
	zapcore.FatalLevel:  breadcrumbTypeFatal,
	zapcore.DPanicLevel: breadcrumbTypeFatal,
}

var levelsToBreadcrumbCategories = map[zapcore.Level]string{
	zapcore.DebugLevel: breadcrumbCategoryDebug,
	zapcore.InfoLevel:  breadcrumbCategoryInfo,
	zapcore.WarnLevel:  breadcrumbCategoryWarn,
	zapcore.ErrorLevel: breadcrumbCategoryError,
	zapcore.FatalLevel: breadcrumbCategoryFatal,
}

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
func newBreadcrumbs() *breadcrumbs { return &breadcrumbs{localOnly: true} }

// Enabled returns true if the given level is at or above the breadcrumbs level.
// It also checks if breadcrumbs are enabled.
func (bc *breadcrumbs) Enabled(lvl zapcore.Level) bool {
	return bc.enabled && bc.level.Enabled(lvl) && bc.level != zapcore.ErrorLevel
}

// new returns a new sentry Breadcrumb from the passed zapcore.Entry and data.
func (bc *breadcrumbs) new(ent zapcore.Entry, data map[string]interface{}) *sentry.Breadcrumb {
	return &sentry.Breadcrumb{
		Data:      data,
		Message:   ent.Message,
		Level:     zapToSentryLevel(ent.Level),
		Type:      zapLevelToBreadcrumbType(ent.Level),
		Category:  zapLevelToBreadcrumbCategory(ent.Level),
		Timestamp: ent.Time,
	}
}

// zapLevelToBreadcrumbType maps zap's Level to Sentry's breadcrumb type.
func zapLevelToBreadcrumbType(lvl zapcore.Level) string {
	t, ok := levelsToBreadcrumbTypes[lvl]
	if !ok {
		return breadcrumbTypeDefault
	}
	return t
}

// zapLevelToBreadcrumbCategory maps zap's Level to Sentry's breadcrumb category.
func zapLevelToBreadcrumbCategory(lvl zapcore.Level) string {
	t, ok := levelsToBreadcrumbCategories[lvl]
	if !ok {
		return ""
	}
	return t
}
