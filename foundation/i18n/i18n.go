package i18n

import (
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
		defer file.Close()

		var messages map[string]string
		if err := json.NewDecoder(file).Decode(&messages); err != nil {
			return fmt.Errorf("failed to decode locale file %s: %w", path, err)
		}

		for key, msg := range messages {
			if err := message.SetString(tag, key, msg); err != nil {
				return fmt.Errorf("failed to set message for %s key %s: %w", tag, key, err)
			}
		}
		fmt.Printf("Loaded %d messages for language %s from %s\n", len(messages), tag, entry.Name())
	}
	return nil
}

// GetDirection returns "rtl" for Arabic and "ltr" for others.
func GetDirection(tag language.Tag) string {
	base, _ := tag.Base()
	if base.String() == "ar" {
		return "rtl"
	}
	return "ltr"
}
