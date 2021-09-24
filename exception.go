package zapsentry

import (
	"strings"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap/zapcore"
)

// ExceptionProvider provides sentry exceptions from zapcore's entries.
type ExceptionProvider interface {
	// Exception accepts a zapcore.Entry and provides a sentry exception.
	// Sentry defines it's exception as an Exception array.
	// This array should contain one exception if an exception exists
	// It will return an empty array if no exception is created.
	Exception(ent zapcore.Entry) []sentry.Exception
}

var _ ExceptionProvider = (*NopExceptionProvider)(nil)

// NopExceptionProvider is a ExceptionProvider implementation which always provides an empty
// exception array.
// Used when exceptions are disabled.
type NopExceptionProvider struct{}

// nopExceptionProvider is a global nopExceptionProvider
var nopExceptionProvider = &NopExceptionProvider{}

// Exception accepts a zapcore.Entry and provides a sentry exception.
// Sentry defines it's exception as an Exception array.
// This array should contain one exception if an exception exists
// It will return an empty array if no exception is created.
func (nep *NopExceptionProvider) Exception(_ zapcore.Entry) []sentry.Exception { return nil }

var _ ExceptionProvider = (*DefaultExceptionProvider)(nil)

type DefaultExceptionProvider struct {
	frameFilter StacktraceFrameFilter
}

// NewExceptionProvider returns anew DefaultExceptionProvider with the passed StacktraceFrameFilter
// as it's frame filter.
func NewExceptionProvider(ff StacktraceFrameFilter) *DefaultExceptionProvider {
	return &DefaultExceptionProvider{frameFilter: ff}
}

// Exception accepts a zapcore.Entry and provides a sentry exception.
// Sentry defines it's exception as an Exception array.
// This array should contain one exception if an exception exists
// It will return an empty array if no exception is created.
func (dep *DefaultExceptionProvider) Exception(ent zapcore.Entry) []sentry.Exception {
	trace := sentry.NewStacktrace()
	if trace == nil {
		return nopExceptionProvider.Exception(ent)
	}

	trace.Frames = dep.frameFilter.FilterFrames(trace.Frames)
	return []sentry.Exception{{
		Type:       ent.Message,
		Value:      ent.Caller.TrimmedPath(),
		Stacktrace: trace,
	}}
}

// StacktraceFrameFilter filters stacktrace frames.
// Used to skip unnecesarry stack trace frames.
type StacktraceFrameFilter interface {
	// FilterFrames will filter out unwanted stacktrace frames
	FilterFrames(frames []sentry.Frame) []sentry.Frame
}

var _ StacktraceFrameFilter = (*DefaultStacktraceFrameFilter)(nil)

// DefaultStacktraceFrameFilter is a default StacktraceFrameFilter implementation
// It uses the same login as the original TheZeroSlave/zapsentry implementation of stack trace
// filtering uses.
// It has sane defaults so it's still the default stack trace filter.
type DefaultStacktraceFrameFilter struct{}

// FilterFrames will filter out unwanted stacktrace frames
func (sff *DefaultStacktraceFrameFilter) FilterFrames(frames []sentry.Frame) []sentry.Frame {
	if len(frames) == 0 {
		return nil
	}
	filteredFrames := make([]sentry.Frame, 0, len(frames))

	for i := range frames {
		// Skip zapsentry and zap internal frames, except for frames in _test packages (for
		// testing).
		if (strings.HasPrefix(frames[i].Module, "github.com/TheZeroSlave/zapsentry") ||
			strings.HasPrefix(frames[i].Function, "go.uber.org/zap")) &&
			!strings.HasSuffix(frames[i].Module, "_test") {
			break
		}
		filteredFrames = append(filteredFrames, frames[i])
	}
	return filteredFrames
}
