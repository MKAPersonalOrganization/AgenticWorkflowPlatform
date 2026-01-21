package i18n

import (
	"context"
	"school-with-ai/foundation/middleware"
)

// T translates a key using the printer found in the context.
func T(ctx context.Context, key string, args ...interface{}) string {
	printer := middleware.GetPrinter(ctx)
	return printer.Sprintf(key, args...)
}

// Dir returns the direction ("ltr" or "rtl") from the context.
func Dir(ctx context.Context) string {
	return middleware.GetDirection(ctx)
}

// Lang returns the language tag from the context.
func Lang(ctx context.Context) string {
	return middleware.GetLanguage(ctx)
}
