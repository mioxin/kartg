package i18n

import (
	"context"
	"testing"
)

func TestLocalizer(t *testing.T) {
	// Инициализация
	err := InitLocalizer()
	if err != nil {
		t.Fatalf("Failed to initialize localizer: %v", err)
	}

	tests := []struct {
		name         string
		lang         Language
		key          string
		templateData interface{}
		wantContains string
	}{
		// Русские переводы
		{
			name:         "Russian cartridge registration",
			lang:         RU,
			key:          "cartridge.registration.success",
			templateData: map[string]string{"ID": "TEST-001"},
			wantContains: "TEST-001",
		},
		{
			name:         "Russian status in use",
			lang:         RU,
			key:          "status.in_use",
			templateData: nil,
			wantContains: "В использовании",
		},
		// Английские переводы
		{
			name:         "English cartridge registration",
			lang:         EN,
			key:          "cartridge.registration.success",
			templateData: map[string]string{"ID": "TEST-001"},
			wantContains: "TEST-001",
		},
		{
			name:         "English status in use",
			lang:         EN,
			key:          "status.in_use",
			templateData: nil,
			wantContains: "In Use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GlobalLocalizer.Localize(tt.lang, tt.key, tt.templateData)
			if tt.wantContains != "" && !contains(got, tt.wantContains) {
				t.Errorf("Localize() = %v, want to contain %v", got, tt.wantContains)
			}
		})
	}
}

func TestT(t *testing.T) {
	err := InitLocalizer()
	if err != nil {
		t.Fatalf("Failed to initialize localizer: %v", err)
	}

	ctx := context.Background()
	
	// Тест без языка в контексте (должен использовать default)
	got := T(ctx, "status.in_use", nil)
	if got == "" {
		t.Error("Expected non-empty translation")
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	err := InitLocalizer()
	if err != nil {
		t.Fatalf("Failed to initialize localizer: %v", err)
	}

	langs := GlobalLocalizer.GetSupportedLanguages()
	if len(langs) != 2 {
		t.Errorf("Expected 2 languages, got %d", len(langs))
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name string
		header string
		want Language
	}{
		{"English", "en-US,en;q=0.9", EN},
		{"Russian", "ru-RU,ru;q=0.9", RU},
		{"Default", "", RU},
		{"Unknown", "fr-FR,fr;q=0.9", RU}, // Default fallback
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLanguage(tt.header)
			if got != tt.want {
				t.Errorf("parseLanguage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
