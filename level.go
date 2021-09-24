package zapsentry

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

type LevelEnabler struct {
	zapcore.Level
	enableBreadcrumbs bool
	breadcrumbsLevel  zapcore.Level
}

func (l *LevelEnabler) Enabled(lvl zapcore.Level) bool {
	return l.Level.Enabled(lvl) || (l.enableBreadcrumbs && l.breadcrumbsLevel.Enabled(lvl))
}

// zapToSentryLevels maps all zap's debug levels to it's corresponding sentry level.
var zapToSentryLevels = map[zapcore.Level]sentry.Level{
	zapcore.DebugLevel:  sentry.LevelDebug,
	zapcore.InfoLevel:   sentry.LevelInfo,
	zapcore.WarnLevel:   sentry.LevelWarning,
	zapcore.ErrorLevel:  sentry.LevelError,
	zapcore.DPanicLevel: sentry.LevelFatal,
	zapcore.PanicLevel:  sentry.LevelFatal,
	zapcore.FatalLevel:  sentry.LevelFatal,
}

// zapToSentryLevel returns the appropriate sentry.Level for the passed zap level.
// If a level is not mapped, by default it will return sentry.LevelFatal
func zapToSentryLevel(lvl zapcore.Level) sentry.Level {
	sentryLvl, ok := zapToSentryLevels[lvl]
	if !ok {
		return sentry.LevelFatal
	}
	return sentryLvl
}
