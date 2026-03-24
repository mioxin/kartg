package i18n

import (
	"context"
	"net/http"
	"strings"
)

// contextKey тип для ключей контекста
type contextKey string

const (
	// LanguageKey ключ для хранения языка в контексте
	LanguageKey contextKey = "language"
)

// LanguageMiddleware определяет язык из заголовка Accept-Language
func LanguageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := detectLanguage(r)
		ctx := context.WithValue(r.Context(), LanguageKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// detectLanguage определяет язык из запроса
func detectLanguage(r *http.Request) Language {
	// Проверяем query параметр
	if lang := r.URL.Query().Get("lang"); lang != "" {
		return parseLanguage(lang)
	}

	// Проверяем заголовок Accept-Language
	acceptLang := r.Header.Get("Accept-Language")
	if acceptLang != "" {
		return parseLanguage(acceptLang)
	}

	// Язык по умолчанию
	return DefaultLanguage()
}

// parseLanguage парсит строку языка
func parseLanguage(lang string) Language {
	// Берем первую часть до запятой или точки с запятой
	parts := strings.Split(lang, ",")
	if len(parts) > 0 {
		lang = strings.TrimSpace(parts[0])
	}

	// Нормализуем
	lang = strings.ToLower(lang)
	lang = strings.Split(lang, "-")[0]

	switch lang {
	case "en":
		return EN
	case "ru":
		return RU
	default:
		return DefaultLanguage()
	}
}

// GetLanguageFromContext получает язык из контекста
func GetLanguageFromContext(ctx context.Context) Language {
	if lang, ok := ctx.Value(LanguageKey).(Language); ok {
		return lang
	}
	return DefaultLanguage()
}
