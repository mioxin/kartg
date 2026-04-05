package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/database"
	"github.com/mioxin/kartg/internal/models"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}, &models.CartridgeModel{}, &models.User{}); err != nil {
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

	repo := NewGORMRepository(testDB)
	svc := NewCartridgeServiceServer(repo)
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

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
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

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
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

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
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

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
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

func TestAnalyticsService_GetRefillsStats(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
	operationSvc := NewOperationServiceServer(testDB)
	analyticsSvc := NewAnalyticsServiceServer(testDB)
	ctx := context.Background()

	// Регистрация и цикл заправки
	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{Id: "CART-A", Model: "HP 12A"})
	if err != nil {
		t.Fatalf("Не удалось зарегистрировать картридж: %v", err)
	}

	_, err = operationSvc.SendToRefill(ctx, &proto.SendToRefillRequest{CartridgeId: "CART-A"})
	if err != nil {
		t.Fatalf("Не удалось отправить на заправку: %v", err)
	}

	_, err = operationSvc.ReceiveFromRefill(ctx, &proto.ReceiveFromRefillRequest{CartridgeId: "CART-A"})
	if err != nil {
		t.Fatalf("Не удалось принять с заправки: %v", err)
	}

	now := time.Now()
	resp, err := analyticsSvc.GetRefillsStats(ctx, &proto.RefillsStatsRequest{
		PeriodStart: timestamppb.New(now.Add(-time.Hour)),
		PeriodEnd:   timestamppb.New(now.Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("Не удалось получить статистику заправок: %v", err)
	}

	if resp.TotalRefills != 1 {
		t.Errorf("Ожидалось 1 заправку, получено %d", resp.TotalRefills)
	}
	if resp.UniqueCartridges != 1 {
		t.Errorf("Ожидалось 1 уникальный картридж, получено %d", resp.UniqueCartridges)
	}
}

func TestOperationService_GenerateAct(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
	operationSvc := NewOperationServiceServer(testDB)
	ctx := context.Background()

	ids := []string{"ACT-001", "ACT-002"}
	for _, id := range ids {
		_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{Id: id, Model: "HP 12A"})
		if err != nil {
			t.Fatalf("Не удалось зарегистрировать картридж %s: %v", id, err)
		}
		_, err = operationSvc.SendToRefill(ctx, &proto.SendToRefillRequest{CartridgeId: id, Comment: "test"})
		if err != nil {
			t.Fatalf("Не удалось отправить картридж %s на заправку: %v", id, err)
		}
	}

	resp, err := operationSvc.GenerateAct(ctx, &proto.GenerateActRequest{})
	if err != nil {
		t.Fatalf("Не удалось сгенерировать акт: %v", err)
	}

	content := string(resp.Value)
	if !contains(content, "Акт выдачи картриджей") {
		t.Errorf("Ожидался HTML акт, получено: %s", content)
	}
	for _, id := range ids {
		if !contains(content, id) {
			t.Errorf("Ожидался акт содержащий ID %s", id)
		}
	}

	var latestSend models.Transaction
	if err := testDB.Where("cartridge_id IN ? AND type = ?", ids, models.OperationTypeSendToRefill).
		Order("timestamp DESC").First(&latestSend).Error; err != nil {
		t.Fatalf("Не удалось получить дату отправки из БД: %v", err)
	}

	if !contains(content, latestSend.Timestamp.Format("02.01.2006")) {
		t.Errorf("Ожидался акт с датой последней отправки из БД, получено: %s", content)
	}
}

func TestAnalyticsService_ExportRefillsStats(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
	operationSvc := NewOperationServiceServer(testDB)
	analyticsSvc := NewAnalyticsServiceServer(testDB)
	ctx := context.Background()

	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{Id: "STAT-001", Model: "HP 12A"})
	if err != nil {
		t.Fatalf("Не удалось зарегистрировать картридж: %v", err)
	}

	_, err = operationSvc.SendToRefill(ctx, &proto.SendToRefillRequest{CartridgeId: "STAT-001"})
	if err != nil {
		t.Fatalf("Не удалось отправить на заправку: %v", err)
	}

	_, err = operationSvc.ReceiveFromRefill(ctx, &proto.ReceiveFromRefillRequest{CartridgeId: "STAT-001"})
	if err != nil {
		t.Fatalf("Не удалось принять с заправки: %v", err)
	}

	now := time.Now()

	for _, format := range []string{"csv", "txt"} {
		resp, err := analyticsSvc.ExportRefillsStats(ctx, &proto.ExportRefillsStatsRequest{
			PeriodStart: timestamppb.New(now.Add(-time.Hour)),
			PeriodEnd:   timestamppb.New(now.Add(time.Hour)),
			Format:      format,
		})
		if err != nil {
			t.Fatalf("Не удалось экспортировать статистику в %s: %v", format, err)
		}
		content := string(resp.Value)
		if !contains(content, "ID картриджа") {
			t.Fatalf("Ожидался отчет в формате %s, получено: %s", format, content)
		}
	}
}

func TestOperationService_ExportCartridgeHistory(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	cartridgeRepo := NewGORMRepository(testDB)
	cartridgeSvc := NewCartridgeServiceServer(cartridgeRepo)
	operationSvc := NewOperationServiceServer(testDB)
	ctx := context.Background()

	_, err := cartridgeSvc.RegisterCartridge(ctx, &proto.RegisterCartridgeRequest{Id: "HIST-001", Model: "HP 12A"})
	if err != nil {
		t.Fatalf("Не удалось зарегистрировать картридж: %v", err)
	}

	_, err = operationSvc.SendToRefill(ctx, &proto.SendToRefillRequest{CartridgeId: "HIST-001"})
	if err != nil {
		t.Fatalf("Не удалось отправить на заправку: %v", err)
	}

	_, err = operationSvc.ReceiveFromRefill(ctx, &proto.ReceiveFromRefillRequest{CartridgeId: "HIST-001"})
	if err != nil {
		t.Fatalf("Не удалось принять с заправки: %v", err)
	}

	for _, format := range []string{"csv", "txt"} {
		resp, err := operationSvc.ExportCartridgeHistory(ctx, &proto.ExportCartridgeHistoryRequest{CartridgeId: "HIST-001", Format: format})
		if err != nil {
			t.Fatalf("Не удалось экспортировать историю в %s: %v", format, err)
		}
		content := string(resp.Value)
		if !contains(content, "ID транзакции") {
			t.Fatalf("Ожидался отчет об истории в формате %s, получено: %s", format, content)
		}
	}
}

func TestModelService_ListModelsAndUpsertModel(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	modelSvc := NewModelServiceServer(testDB)
	ctx := context.Background()

	resp, err := modelSvc.UpsertModel(ctx, &proto.UpsertModelRequest{Name: "HP 12A"})
	if err != nil {
		t.Fatalf("Не удалось создать модель: %v", err)
	}
	if resp.Name != "HP 12A" {
		t.Errorf("Ожидалось имя модели HP 12A, получено %s", resp.Name)
	}

	listResp, err := modelSvc.ListModels(ctx, &proto.ListModelsRequest{Search: "HP", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Не удалось получить список моделей: %v", err)
	}
	if len(listResp.Models) == 0 {
		t.Fatalf("Ожидалось хотя бы одну модель, получено %d", len(listResp.Models))
	}
	if listResp.Models[0].Name != "HP 12A" {
		t.Errorf("Ожидалось модель HP 12A, получено %s", listResp.Models[0].Name)
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
