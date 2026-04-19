// Package logger provides support for initializing the log system.
package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"time"
)

// TraceIDFn represents a function that can return the trace id from
// the specified context.
type TraceIDFn func(ctx context.Context) string

// Logger represents a logger for logging information.
type Logger struct {
	slog.Handler
	discard   bool
	traceIDFn TraceIDFn
}

type logMode int

const (
	modeJSON logMode = iota
	modeText
	modeMinimal
)

// New constructs a new log for application use using JSON.
func New(w io.Writer, minLevel Level, serviceName string, traceIDFn TraceIDFn) *Logger {
	return new(w, minLevel, serviceName, traceIDFn, Events{}, modeJSON)
}

// NewText constructs a new log for application use using Text.
func NewText(w io.Writer, minLevel Level, serviceName string, traceIDFn TraceIDFn) *Logger {
	return new(w, minLevel, serviceName, traceIDFn, Events{}, modeText)
}

// NewTextWithEvents constructs a new log for application use with events using Text.
func NewTextWithEvents(w io.Writer, minLevel Level, serviceName string, traceIDFn TraceIDFn, events Events) *Logger {
	return new(w, minLevel, serviceName, traceIDFn, events, modeText)
}

// Debug logs at LevelDebug with the given context.
func (log *Logger) Debug(ctx context.Context, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelDebug, 3, msg, args...)
}

// Debugc logs the information at the specified call stack position.
func (log *Logger) Debugc(ctx context.Context, caller int, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelDebug, caller, msg, args...)
}

// Info logs at LevelInfo with the given context.
func (log *Logger) Info(ctx context.Context, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelInfo, 3, msg, args...)
}

// Infoc logs the information at the specified call stack position.
func (log *Logger) Infoc(ctx context.Context, caller int, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelInfo, caller, msg, args...)
}

// Warn logs at LevelWarn with the given context.
func (log *Logger) Warn(ctx context.Context, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelWarn, 3, msg, args...)
}

// Warnc logs the information at the specified call stack position.
func (log *Logger) Warnc(ctx context.Context, caller int, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelWarn, caller, msg, args...)
}

// Error logs at LevelError with the given context.
func (log *Logger) Error(ctx context.Context, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelError, 3, msg, args...)
}

// Errorc logs the information at the specified call stack position.
func (log *Logger) Errorc(ctx context.Context, caller int, msg string, args ...any) {
	if log.discard {
		return
	}

	log.write(ctx, LevelError, caller, msg, args...)
}

func (log *Logger) write(ctx context.Context, level Level, caller int, msg string, args ...any) {
	slogLevel := slog.Level(level)

	if !log.Enabled(ctx, slogLevel) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(caller, pcs[:])

	r := slog.NewRecord(time.Now(), slogLevel, msg, pcs[0])

	if log.traceIDFn != nil {
		args = append(args, "trace_id", log.traceIDFn(ctx))
	}
	r.Add(args...)

	_ = log.Handle(ctx, r)
}

func new(w io.Writer, minLevel Level, serviceName string, traceIDFn TraceIDFn, events Events, mode logMode) *Logger {
	// Convert the file name to just the name.ext when this key/value will
	// be logged.
	f := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey || a.Key == "service" {
			return slog.Attr{}
		}

		if a.Key == slog.SourceKey {
			if source, ok := a.Value.Any().(*slog.Source); ok {
				v := fmt.Sprintf("%s:%d", filepath.Base(source.File), source.Line)
				return slog.Attr{Key: "file", Value: slog.StringValue(v)}
			}
		}

		return a
	}

	// Construct the slog handler for use.
	var handler slog.Handler
	options := &slog.HandlerOptions{AddSource: true, Level: slog.Level(minLevel), ReplaceAttr: f}
	switch mode {
	case modeJSON:
		handler = slog.NewJSONHandler(w, options)
	case modeText:
		handler = slog.NewTextHandler(w, options)
	case modeMinimal:
		handler = &minimalHandler{w: w, options: options}
	}

	// If events are to be processed, wrap the JSON handler around the custom
	// log handler.
	if events.Debug != nil || events.Info != nil || events.Warn != nil || events.Error != nil {
		handler = newLogHandler(handler, events)
	}

	// Attributes to add to every log.
	attrs := []slog.Attr{
		{Key: "service", Value: slog.StringValue(serviceName)},
	}

	// Add those attributes and capture the final handler.
	handler = handler.WithAttrs(attrs)

	return &Logger{
		Handler:   handler,
		discard:   w == io.Discard,
		traceIDFn: traceIDFn,
	}
}

type minimalHandler struct {
	w       io.Writer
	options *slog.HandlerOptions
	attrs   []slog.Attr
}

func (h *minimalHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.options.Level.Level()
}

func (h *minimalHandler) Handle(ctx context.Context, r slog.Record) error {
	var out string

	// Handle level
	level := r.Level.String()
	if h.options.ReplaceAttr != nil {
		attr := h.options.ReplaceAttr(nil, slog.Any(slog.LevelKey, r.Level))
		if attr.Key != "" {
			level = attr.Value.String()
		} else if attr.Equal(slog.Attr{}) {
			level = ""
		}
	}
	if level != "" {
		out += level + " "
	}

	// Handle message
	out += r.Message

	// Handle attributes from WithAttrs
	for _, attr := range h.attrs {
		if h.options.ReplaceAttr != nil {
			attr = h.options.ReplaceAttr(nil, attr)
		}
		if attr.Equal(slog.Attr{}) {
			continue
		}
		out += " " + attr.Value.String()
	}

	// Handle record attributes
	r.Attrs(func(attr slog.Attr) bool {
		if h.options.ReplaceAttr != nil {
			attr = h.options.ReplaceAttr(nil, attr)
		}
		if attr.Equal(slog.Attr{}) {
			return true
		}
		out += " " + attr.Value.String()
		return true
	})

	_, err := fmt.Fprintln(h.w, out)
	return err
}

func (h *minimalHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr(nil), h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &minimalHandler{w: h.w, options: h.options, attrs: newAttrs}
}

func (h *minimalHandler) WithGroup(name string) slog.Handler {
	return h // Groups not supported in minimal mode
}
