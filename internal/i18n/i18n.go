package i18n

import (
	"embed"
	"fmt"
	"sync"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var LocaleFS embed.FS

// Language представляет поддерживаемый язык
type Language string

const (
	RU Language = "ru"
	EN Language = "en"
)

// Localizer содержит локализатор для всех языков
type Localizer struct {
	bundles map[Language]*i18n.Localizer
	mu      sync.RWMutex
}

// NewLocalizer создает новый локализатор
func NewLocalizer() (*Localizer, error) {
	l := &Localizer{
		bundles: make(map[Language]*i18n.Localizer),
	}

	// Загружаем русские переводы
	ruBundle, err := l.loadBundle("locales/ru.json", language.Russian)
	if err != nil {
		return nil, fmt.Errorf("failed to load Russian bundle: %w", err)
	}
	l.bundles[RU] = i18n.NewLocalizer(ruBundle, "ru")

	// Загружаем английские переводы
	enBundle, err := l.loadBundle("locales/en.json", language.English)
	if err != nil {
		return nil, fmt.Errorf("failed to load English bundle: %w", err)
	}
	l.bundles[EN] = i18n.NewLocalizer(enBundle, "en")

	return l, nil
}

// loadBundle загружает файл переводов
func (l *Localizer) loadBundle(path string, tag language.Tag) (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(tag)
	_, err := bundle.LoadMessageFileFS(LocaleFS, path)
	if err != nil {
		return nil, err
	}
	return bundle, nil
}

// Localize возвращает локализованную строку
func (l *Localizer) Localize(lang Language, key string, templateData interface{}) string {
	l.mu.RLock()
	localizer, ok := l.bundles[lang]
	if !ok {
		localizer = l.bundles[EN] // Fallback на английский
	}
	l.mu.RUnlock()

	if localizer == nil {
		return key // Если ничего не загружено, возвращаем ключ
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    key,
		TemplateData: templateData,
	})
	if err != nil {
		return key // Если ошибка, возвращаем ключ
	}

	return msg
}

// GetSupportedLanguages возвращает список поддерживаемых языков
func (l *Localizer) GetSupportedLanguages() []Language {
	return []Language{RU, EN}
}

// DefaultLanguage возвращает язык по умолчанию
func DefaultLanguage() Language {
	return RU
}
