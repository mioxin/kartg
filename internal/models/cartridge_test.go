package models

import (
	"testing"
)

// TestCartridge_CanSendToRefill тестирует возможность отправки на заправку
func TestCartridge_CanSendToRefill(t *testing.T) {
	tests := []struct {
		name    string
		status  CartridgeStatus
		wantErr bool
	}{
		{"Можно отправить из InUse", CartridgeStatusInUse, false},
		{"Нельзя отправить из Refilling", CartridgeStatusRefilling, true},
		{"Нельзя отправить из Retired", CartridgeStatusRetired, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cartridge{
				ID:     "TEST-001",
				Model:  "HP 12A",
				Status: tt.status,
			}

			err := c.CanSendToRefill()

			if tt.wantErr && err == nil {
				t.Errorf("Ожидалась ошибка, но получено nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
			}
		})
	}
}

// TestCartridge_CanReceiveFromRefill тестирует возможность приема с заправки
func TestCartridge_CanReceiveFromRefill(t *testing.T) {
	tests := []struct {
		name    string
		status  CartridgeStatus
		wantErr bool
	}{
		{"Нельзя принять из InUse", CartridgeStatusInUse, true},
		{"Можно принять из Refilling", CartridgeStatusRefilling, false},
		{"Нельзя принять из Retired", CartridgeStatusRetired, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cartridge{
				ID:     "TEST-001",
				Model:  "HP 12A",
				Status: tt.status,
			}

			err := c.CanReceiveFromRefill()

			if tt.wantErr && err == nil {
				t.Errorf("Ожидалась ошибка, но получено nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
			}
		})
	}
}

// TestCartridge_CanRetire тестирует возможность утилизации
func TestCartridge_CanRetire(t *testing.T) {
	tests := []struct {
		name    string
		status  CartridgeStatus
		wantErr bool
	}{
		{"Можно утилизировать из InUse", CartridgeStatusInUse, false},
		{"Можно утилизировать из Refilling", CartridgeStatusRefilling, false},
		{"Нельзя утилизировать из Retired", CartridgeStatusRetired, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Cartridge{
				ID:     "TEST-001",
				Model:  "HP 12A",
				Status: tt.status,
			}

			err := c.CanRetire()

			if tt.wantErr && err == nil {
				t.Errorf("Ожидалась ошибка, но получено nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
			}
		})
	}
}

// TestIsValidTransition тестирует валидность переходов статусов
func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    CartridgeStatus
		to      CartridgeStatus
		wantErr bool
	}{
		{"InUse -> Refilling", CartridgeStatusInUse, CartridgeStatusRefilling, false},
		{"InUse -> Retired", CartridgeStatusInUse, CartridgeStatusRetired, false},
		{"Refilling -> InUse", CartridgeStatusRefilling, CartridgeStatusInUse, false},
		{"Refilling -> Retired", CartridgeStatusRefilling, CartridgeStatusRetired, false},
		{"Retired -> InUse", CartridgeStatusRetired, CartridgeStatusInUse, true},
		{"Retired -> Refilling", CartridgeStatusRetired, CartridgeStatusRefilling, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsValidTransition(tt.from, tt.to)

			if tt.wantErr && err == nil {
				t.Errorf("Ожидалась ошибка для перехода %s -> %s, но получено nil", tt.from, tt.to)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Не ожидалась ошибка для перехода %s -> %s, но получено: %v", tt.from, tt.to, err)
			}
		})
	}
}
