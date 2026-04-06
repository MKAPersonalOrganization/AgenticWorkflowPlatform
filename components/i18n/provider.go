package i18n

import (
	"context"
	"aicompass/foundation/i18n"

	"golang.org/x/text/language"
)

// T translates a key using the printer found in the context.
func T(ctx context.Context, key string, args ...interface{}) string {
	return i18n.T(ctx, key, args...)
}

// Dir returns the direction ("ltr" or "rtl") from the context.
func Dir(ctx context.Context) string {
	tag := i18n.FromContext(ctx)
	return i18n.GetDirection(tag)
}

// Lang returns the language tag from the context.
func Lang(ctx context.Context) string {
	return i18n.FromContext(ctx).String()
}

// Messages returns the translation map for the current language.
func Messages(ctx context.Context) map[string]string {
	tag, _ := language.Parse(Lang(ctx))
	return i18n.GetMessages(tag)
}
