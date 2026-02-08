package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"log"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

var (
	bundle        *i18n.Bundle
	defaultLocale = "en"
)

type ctxKey struct{}

// Init loads all locale files and sets the default locale.
func Init(defLocale string) {
	if defLocale != "" {
		defaultLocale = defLocale
	}

	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		log.Fatalf("i18n: read locales dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := localeFS.ReadFile("locales/" + e.Name())
		if err != nil {
			log.Fatalf("i18n: read %s: %v", e.Name(), err)
		}
		bundle.MustParseMessageFileBytes(data, e.Name())
	}
	log.Printf("i18n: loaded %d locale files, default=%s", len(entries), defaultLocale)
}

// WithLocale returns a new context carrying the given locale string (e.g. "vi", "en").
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxKey{}, locale)
}

// LocaleFromContext extracts the locale from the context.
// Returns the configured default locale if not set.
func LocaleFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok && v != "" {
		return v
	}
	return defaultLocale
}

// T translates a message ID using the locale from the context.
// Optional templateData provides values for template placeholders.
func T(ctx context.Context, messageID string, templateData ...map[string]any) string {
	lang := LocaleFromContext(ctx)
	l := i18n.NewLocalizer(bundle, lang)

	cfg := &i18n.LocalizeConfig{MessageID: messageID}
	if len(templateData) > 0 && templateData[0] != nil {
		cfg.TemplateData = templateData[0]
	}

	msg, err := l.Localize(cfg)
	if err != nil {
		return messageID
	}
	return msg
}
