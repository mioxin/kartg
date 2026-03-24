package service

import (
	"context"
	"os"
	"testing"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/database"
	"github.com/mioxin/kartg/internal/models"
	"gorm.io/gorm"
)

var testDB *gorm.DB

// setupTestDB создает тестовую базу данных в памяти
func setupTestDB(t *testing.T) {
	t.Helper()

	// Создаем временную БД в памяти
	db, err := database.New(database.Config{
		DBPath:   ":memory:",
		LogLevel: "error",
	})
	if err != nil {
		t.Fatalf("Не удалось создать тестовую БД: %v", err)
	}

	// Миграция
	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}, &models.User{}); err != nil {
		t.Fatalf("Ошибка миграции: %v", err)
	}

	testDB = db
}

// teardownTestDB очищает БД после теста
func teardownTestDB(t *testing.T) {
	t.Helper()

	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("Ошибка получения SQL подключения: %v", err)
	}
	sqlDB.Close()
	testDB = nil
}

// TestCartridgeService_RegisterCartridge тестирует регистрацию картриджа
func TestCartridgeService_RegisterCartridge(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	svc := NewCartridgeServiceServer(testDB)
	ctx := context.Background()

	tests := []struct {
		name        string
		id          string
		model       string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Успешная регистрация",
			id:      "CART-001",
			model:   "HP 12A",
			wantErr: false,
		},
		{
			name:    "Регистрация с пустой моделью",
			id:      "CART-002",
			model:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.RegisterCartridgeRequest{
				Id:    tt.id,
				Model: tt.model,
			}

			resp, err := svc.RegisterCartridge(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Id != tt.id {
				t.Errorf("Ожидался ID %s, получен %s", tt.id, resp.Id)
			}
		})
	}
}

// TestOperationService_SendToRefill тестирует отправку на заправку
func TestOperationService_SendToRefill(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeSvc := NewCartridgeServiceServer(testDB)
	operationSvc := NewOperationServiceServer(testDB)
	ctx := context.Background()

	// Создаем тестовый картридж
	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{
		Id:    "CART-001",
		Model: "HP 12A",
	})
	if err != nil {
		t.Fatalf("Не удалось создать картридж: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Успешная отправка на заправку",
			id:      "CART-001",
			wantErr: false,
		},
		{
			name:        "Повторная отправка (уже на заправке)",
			id:          "CART-001",
			wantErr:     true,
			errContains: "уже находится на заправке",
		},
		{
			name:    "Отправка несуществующего картриджа",
			id:      "CART-999",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.SendToRefillRequest{
				CartridgeId: tt.id,
				Comment:     "Тестовая заправка",
			}

			resp, err := operationSvc.SendToRefill(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("Ожидалась ошибка содержащая '%s', получено: %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Status != proto.CartridgeStatus_CARTRIDGE_STATUS_REFILLING {
				t.Errorf("Ожидался статус REFILLING, получен %v", resp.Status)
			}
		})
	}
}

// TestOperationService_ReceiveFromRefill тестирует прием с заправки
func TestOperationService_ReceiveFromRefill(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeSvc := NewCartridgeServiceServer(testDB)
	operationSvc := NewOperationServiceServer(testDB)
	ctx := context.Background()

	// Создаем и отправляем картридж на заправку
	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{
		Id:    "CART-002",
		Model: "Canon 725",
	})
	if err != nil {
		t.Fatalf("Не удалось создать картридж: %v", err)
	}

	_, err = operationSvc.SendToRefill(ctx, &proto.SendToRefillRequest{
		CartridgeId: "CART-002",
	})
	if err != nil {
		t.Fatalf("Не удалось отправить на заправку: %v", err)
	}

	tests := []struct {
		name         string
		id           string
		wantErr      bool
		checkRefills bool
	}{
		{
			name:         "Успешный прием с заправки",
			id:           "CART-002",
			wantErr:      false,
			checkRefills: true,
		},
		{
			name:    "Прием картриджа не с заправки",
			id:      "CART-002",
			wantErr: true,
		},
	}

	var initialRefills int32 = 0

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Получаем текущее количество заправок
			cart, _ := cartridgeSvc.GetCartridge(ctx, &proto.GetCartridgeRequest{Id: tt.id})
			if cart != nil {
				initialRefills = cart.TotalRefills
			}

			req := &proto.ReceiveFromRefillRequest{
				CartridgeId: tt.id,
				Comment:     "Принят после заправки",
			}

			resp, err := operationSvc.ReceiveFromRefill(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Status != proto.CartridgeStatus_CARTRIDGE_STATUS_IN_USE {
				t.Errorf("Ожидался статус IN_USE, получен %v", resp.Status)
			}

			if tt.checkRefills && resp.TotalRefills != initialRefills+1 {
				t.Errorf("Ожидалось увеличение счетчика заправок с %d до %d, получено %d",
					initialRefills, initialRefills+1, resp.TotalRefills)
			}
		})
	}
}

// TestOperationService_RetireCartridge тестирует утилизацию картриджа
func TestOperationService_RetireCartridge(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeSvc := NewCartridgeServiceServer(testDB)
	operationSvc := NewOperationServiceServer(testDB)
	ctx := context.Background()

	// Создаем тестовый картридж
	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{
		Id:    "CART-003",
		Model: "HP 13A",
	})
	if err != nil {
		t.Fatalf("Не удалось создать картридж: %v", err)
	}

	tests := []struct {
		name        string
		id          string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Успешная утилизация",
			id:      "CART-003",
			wantErr: false,
		},
		{
			name:        "Повторная утилизация",
			id:          "CART-003",
			wantErr:     true,
			errContains: "уже утилизирован",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.RetireCartridgeRequest{
				CartridgeId: tt.id,
				Comment:     "Списан по причине износа",
			}

			resp, err := operationSvc.RetireCartridge(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				if tt.errContains != "" && err != nil && !contains(err.Error(), tt.errContains) {
					t.Errorf("Ожидалась ошибка содержащая '%s', получено: %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Status != proto.CartridgeStatus_CARTRIDGE_STATUS_RETIRED {
				t.Errorf("Ожидался статус RETIRED, получен %v", resp.Status)
			}

			if resp.RetiredAt == nil {
				t.Errorf("Ожидалась дата утилизации, получено nil")
			}
		})
	}
}

// TestAnalyticsService_GetGlobalStats тестирует получение общей статистики
func TestAnalyticsService_GetGlobalStats(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeSvc := NewCartridgeServiceServer(testDB)
	analyticsSvc := NewAnalyticsServiceServer(testDB)
	ctx := context.Background()

	// Создаем несколько картриджей
	testData := []struct {
		id     string
		model  string
		status models.CartridgeStatus
	}{
		{"CART-001", "HP 12A", models.CartridgeStatusInUse},
		{"CART-002", "Canon 725", models.CartridgeStatusRefilling},
		{"CART-003", "HP 13A", models.CartridgeStatusRetired},
	}

	for _, td := range testData {
		_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{
			Id:    td.id,
			Model: td.model,
		})
		if err != nil {
			t.Fatalf("Не удалось создать картридж: %v", err)
		}

		// Обновляем статус если нужно
		if td.status != models.CartridgeStatusInUse {
			testDB.Model(&models.Cartridge{}).Where("id = ?", td.id).Update("status", td.status)
		}
	}

	// Получаем статистику
	resp, err := analyticsSvc.GetGlobalStats(ctx, &proto.GlobalStatsRequest{})
	if err != nil {
		t.Fatalf("Не удалось получить статистику: %v", err)
	}

	if resp.TotalCartridges != 3 {
		t.Errorf("Ожидалось 3 картриджа, получено %d", resp.TotalCartridges)
	}

	if resp.InUse != 1 {
		t.Errorf("Ожидался 1 картридж в использовании, получено %d", resp.InUse)
	}

	if resp.Refilling != 1 {
		t.Errorf("Ожидался 1 картридж на заправке, получено %d", resp.Refilling)
	}

	if resp.Retired != 1 {
		t.Errorf("Ожидался 1 утилизированный картридж, получено %d", resp.Retired)
	}
}

// contains проверяет, содержит ли строка подстроку
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

// TestMain запускается перед всеми тестами
func TestMain(m *testing.M) {
	// Настраиваем окружение для тестов
	os.Setenv("LOG_LEVEL", "error")

	// Запускаем тесты
	code := m.Run()

	// Выходим с кодом результата
	os.Exit(code)
}
