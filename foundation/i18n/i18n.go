package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Language represents a supported language.
type Language struct {
	Tag        language.Tag
	Name       string
	NativeName string // Name in the language itself (e.g., "العربية" for Arabic)
	Dir        string // "ltr" or "rtl"
}

// IsRTL returns true if the language is right-to-left.
func (l Language) IsRTL() bool {
	return l.Dir == "rtl"
}

var (
	// SupportedLanguages lists all available languages.
	SupportedLanguages = []Language{
		{Tag: language.English, Name: "English", NativeName: "English", Dir: "ltr"},
		{Tag: language.Arabic, Name: "Arabic", NativeName: "العربية", Dir: "rtl"},
	}

	// Matcher is used to find the best match for a list of preferred tags.
	Matcher = language.NewMatcher([]language.Tag{
		language.English, // The first language is the fallback
		language.Arabic,
	})

	// messagesCache stores raw translations for frontend exposure
	messagesCache = make(map[string]map[string]string)
)

// GetLanguageByTag returns the Language for a given tag, or nil if not found.
func GetLanguageByTag(tag language.Tag) *Language {
	base, _ := tag.Base()
	for i := range SupportedLanguages {
		langBase, _ := SupportedLanguages[i].Tag.Base()
		if langBase == base {
			return &SupportedLanguages[i]
		}
	}
	return nil
}

// LoadCatalogs loads translation files from the specified directory.
// It expects files named like "en.json", "ar.json".
func LoadCatalogs(localesDir string) error {
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return fmt.Errorf("failed to read locales dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		tagStr := entry.Name()[:len(entry.Name())-5] // remove .json
		tag, err := language.Parse(tagStr)
		if err != nil {
			continue // skip invalid language tags
		}

		path := filepath.Join(localesDir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open locale file %s: %w", path, err)
		}
		// No defer close in loop, but we close it manually
		// defer file.Close()

		var messages map[string]string
		if err := json.NewDecoder(file).Decode(&messages); err != nil {
			file.Close()
			return fmt.Errorf("failed to decode locale file %s: %w", path, err)
		}
		file.Close()

		// Store in cache for frontend
		messagesCache[tag.String()] = messages

		for key, msg := range messages {
			if err := message.SetString(tag, key, msg); err != nil {
				return fmt.Errorf("failed to set message for %s key %s: %w", tag, key, err)
			}
		}
		fmt.Printf("Loaded %d messages for language %s from %s\n", len(messages), tag, entry.Name())
	}
	return nil
}

// GetMessages returns the raw translation map for a given tag.
func GetMessages(tag language.Tag) map[string]string {
	// Try exact match first
	if m, ok := messagesCache[tag.String()]; ok {
		return m
	}

	// Try base language match
	base, _ := tag.Base()
	if m, ok := messagesCache[base.String()]; ok {
		return m
	}

	// Fallback to English
	return messagesCache["en"]
}

// GetDirection returns "rtl" for Arabic and "ltr" for others.
func GetDirection(tag language.Tag) string {
	base, _ := tag.Base()
	if base.String() == "ar" {
		return "rtl"
	}
	return "ltr"
}

// contextKey is a private type for context keys.
type contextKey string

const (
	langKey contextKey = "lang"
)

// FromContext returns the language tag from the context.
func FromContext(ctx context.Context) language.Tag {
	if tag, ok := ctx.Value(langKey).(language.Tag); ok {
		return tag
	}
	return language.English
}

// WithLanguage returns a new context with the assigned language tag.
func WithLanguage(ctx context.Context, tag language.Tag) context.Context {
	return context.WithValue(ctx, langKey, tag)
}

// T translates the given key using the language in the context.
func T(ctx context.Context, key string, args ...any) string {
	tag := FromContext(ctx)
	p := message.NewPrinter(tag)

	// Localize dynamic values in args if they are numbers and we are in Arabic mode
	base, _ := tag.Base()
	isArabic := base.String() == "ar"

	if isArabic {
		for i, arg := range args {
			switch v := arg.(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
				args[i] = toEasternArabicDigits(fmt.Sprintf("%v", v))
			}
		}
	}

	translated := p.Sprintf(key, args...)

	// Global conversion for Arabic strings to use Eastern numerals
	if isArabic {
		return toEasternArabicDigits(translated)
	}

	return translated
}

func toEasternArabicDigits(s string) string {
	mapping := map[rune]rune{
		'0': '٠', '1': '١', '2': '٢', '3': '٣', '4': '٤',
		'5': '٥', '6': '٦', '7': '٧', '8': '٨', '9': '٩',
	}

	runes := []rune(s)
	for i, r := range runes {
		if mapped, ok := mapping[r]; ok {
			runes[i] = mapped
		}
	}
	return string(runes)
}
