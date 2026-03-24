package i18n

import (
	"context"
)

// T возвращает локализованную строку для ключа
func T(ctx context.Context, key string, templateData interface{}) string {
	lang := GetLanguageFromContext(ctx)
	return GlobalLocalizer.Localize(lang, key, templateData)
}

// TR возвращает локализованную строку для указанного языка
func TR(lang Language, key string, templateData interface{}) string {
	return GlobalLocalizer.Localize(lang, key, templateData)
}

// MustLocalize создает локализатор или паникует при ошибке
func MustLocalize() *Localizer {
	l, err := NewLocalizer()
	if err != nil {
		panic(err)
	}
	return l
}

// GlobalLocalizer глобальный экземпляр локализатора
var GlobalLocalizer *Localizer

// InitLocalizer инициализирует глобальный локализатор
func InitLocalizer() error {
	var err error
	GlobalLocalizer, err = NewLocalizer()
	return err
}
