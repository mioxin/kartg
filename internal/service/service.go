package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/models"
	"gorm.io/gorm"
)

// CartridgeServiceServer реализует сервис управления картриджами
type CartridgeServiceServer struct {
	proto.UnimplementedCartridgeServiceServer
	db *gorm.DB
}

// NewCartridgeServiceServer создает новый сервис картриджей
func NewCartridgeServiceServer(db *gorm.DB) *CartridgeServiceServer {
	return &CartridgeServiceServer{db: db}
}

// RegisterCartridge регистрирует новый картридж
func (s *CartridgeServiceServer) RegisterCartridge(ctx context.Context, req *proto.RegisterCartridgeRequest) (*proto.Cartridge, error) {
	slog.Info("Регистрация картриджа", "id", req.Id, "model", req.Model)

	// Транзакция БД
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var cartridge models.Cartridge
		result := tx.FirstOrCreate(&cartridge, models.Cartridge{ID: req.Id}, map[string]interface{}{
			"model":         req.Model,
			"status":        models.CartridgeStatusInUse,
			"total_refills": 0,
			"created_at":    time.Now(),
		})

		if result.Error != nil {
			return fmt.Errorf("ошибка при регистрации: %w", result.Error)
		}

		// Если картридж уже существовал и модель отличается - обновляем
		if result.RowsAffected == 0 && cartridge.Model != req.Model && req.Model != "" {
			slog.Info("Обновление модели картриджа", "id", req.Id, "old_model", cartridge.Model, "new_model", req.Model)
			tx.Model(&cartridge).Update("model", req.Model)
			cartridge.Model = req.Model
		}

		// Сохраняем модель в справочник
		if req.Model != "" {
			var model models.CartridgeModel
			now := time.Now()

			// Пытаемся найти существующую модель
			modelResult := tx.Where("name = ?", req.Model).First(&model)

			if modelResult.Error == gorm.ErrRecordNotFound {
				// Создаем новую модель
				model = models.CartridgeModel{
					Name:       req.Model,
					UsageCount: 1,
					LastUsedAt: now,
					CreatedAt:  now,
				}
				if err := tx.Create(&model).Error; err != nil {
					return fmt.Errorf("ошибка при создании модели: %w", err)
				}
				slog.Info("Создана новая модель в справочнике", "name", req.Model)
			} else if modelResult.Error == nil {
				// Обновляем счетчик использования
				tx.Model(&model).Updates(map[string]interface{}{
					"usage_count":  gorm.Expr("usage_count + 1"),
					"last_used_at": now,
				})
			}
		}

		// Создаем транзакцию регистрации
		transaction := models.Transaction{
			ID:          uuid.New().String(),
			CartridgeID: cartridge.ID,
			Type:        models.OperationTypeRegistration,
			Timestamp:   time.Now(),
			Comment:     "Регистрация картриджа",
		}
		return tx.Create(&transaction).Error
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при регистрации: %v", err)
	}

	// Перечитываем картридж
	var cartridge models.Cartridge
	s.db.First(&cartridge, "id = ?", req.Id)
	return toProtoCartridge(&cartridge), nil
}

// GetCartridge получает информацию о картридже
func (s *CartridgeServiceServer) GetCartridge(ctx context.Context, req *proto.GetCartridgeRequest) (*proto.Cartridge, error) {
	var cartridge models.Cartridge
	result := s.db.First(&cartridge, "id = ?", req.Id)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "картридж не найден: %s", req.Id)
		}
		return nil, status.Errorf(codes.Internal, "ошибка при получении: %v", result.Error)
	}

	return toProtoCartridge(&cartridge), nil
}

// ListCartridges возвращает список картриджей с пагинацией
func (s *CartridgeServiceServer) ListCartridges(ctx context.Context, req *proto.ListCartridgesRequest) (*proto.ListCartridgesResponse, error) {
	var cartridges []models.Cartridge
	var total int64

	query := s.db.Model(&models.Cartridge{})

	// Фильтр по поиску
	if req.Search != "" {
		query = query.Where("id LIKE ? OR model LIKE ?", "%"+req.Search+"%", "%"+req.Search+"%")
	}

	// Фильтр по статусу
	if req.Status != proto.CartridgeStatus_CARTRIDGE_STATUS_UNSPECIFIED {
		query = query.Where("status = ?", fromProtoStatus(req.Status))
	}

	// Получаем общее количество
	query.Count(&total)

	// Пагинация
	offset := (req.Page - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	result := query.Offset(int(offset)).Limit(int(pageSize)).Find(&cartridges)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении списка: %v", result.Error)
	}

	protoCartridges := make([]*proto.Cartridge, len(cartridges))
	for i, c := range cartridges {
		protoCartridges[i] = toProtoCartridge(&c)
	}

	return &proto.ListCartridgesResponse{
		Cartridges: protoCartridges,
		TotalCount: int32(total),
	}, nil
}

// toProtoCartridge конвертирует модель в proto сообщение
func toProtoCartridge(c *models.Cartridge) *proto.Cartridge {
	pc := &proto.Cartridge{
		Id:           c.ID,
		Model:        c.Model,
		Status:       toProtoStatus(c.Status),
		TotalRefills: int32(c.TotalRefills),
		CreatedAt:    timestamppb.New(c.CreatedAt),
	}
	if c.RetiredAt != nil {
		pc.RetiredAt = timestamppb.New(*c.RetiredAt)
	}
	return pc
}

// toProtoStatus конвертирует статус модели в proto статус
func toProtoStatus(s models.CartridgeStatus) proto.CartridgeStatus {
	switch s {
	case models.CartridgeStatusInUse:
		return proto.CartridgeStatus_CARTRIDGE_STATUS_IN_USE
	case models.CartridgeStatusRefilling:
		return proto.CartridgeStatus_CARTRIDGE_STATUS_REFILLING
	case models.CartridgeStatusRetired:
		return proto.CartridgeStatus_CARTRIDGE_STATUS_RETIRED
	default:
		return proto.CartridgeStatus_CARTRIDGE_STATUS_UNSPECIFIED
	}
}

// fromProtoStatus конвертирует proto статус в статус модели
func fromProtoStatus(s proto.CartridgeStatus) models.CartridgeStatus {
	switch s {
	case proto.CartridgeStatus_CARTRIDGE_STATUS_IN_USE:
		return models.CartridgeStatusInUse
	case proto.CartridgeStatus_CARTRIDGE_STATUS_REFILLING:
		return models.CartridgeStatusRefilling
	case proto.CartridgeStatus_CARTRIDGE_STATUS_RETIRED:
		return models.CartridgeStatusRetired
	default:
		return models.CartridgeStatusInUse
	}
}

// OperationServiceServer реализует сервис операций
type OperationServiceServer struct {
	proto.UnimplementedOperationServiceServer
	db *gorm.DB
}

// NewOperationServiceServer создает новый сервис операций
func NewOperationServiceServer(db *gorm.DB) *OperationServiceServer {
	return &OperationServiceServer{db: db}
}

// SendToRefill отправляет картридж на заправку
func (s *OperationServiceServer) SendToRefill(ctx context.Context, req *proto.SendToRefillRequest) (*proto.Cartridge, error) {
	slog.Info("Отправка на заправку", "cartridge_id", req.CartridgeId, "comment", req.Comment)

	var cartridge models.Cartridge
	if err := s.db.First(&cartridge, "id = ?", req.CartridgeId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "картридж не найден: %s", req.CartridgeId)
		}
		return nil, status.Errorf(codes.Internal, "ошибка: %v", err)
	}

	// Проверяем валидность перехода
	if err := cartridge.CanSendToRefill(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	// Транзакция БД
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Обновляем статус
		if err := tx.Model(&cartridge).Update("status", models.CartridgeStatusRefilling).Error; err != nil {
			return err
		}

		// Создаем транзакцию операции
		transaction := models.Transaction{
			ID:          uuid.New().String(),
			CartridgeID: cartridge.ID,
			Type:        models.OperationTypeSendToRefill,
			Timestamp:   time.Now(),
			Comment:     req.Comment,
		}
		return tx.Create(&transaction).Error
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при отправке: %v", err)
	}

	cartridge.Status = models.CartridgeStatusRefilling
	return toProtoCartridge(&cartridge), nil
}

// ReceiveFromRefill принимает картридж с заправки
func (s *OperationServiceServer) ReceiveFromRefill(ctx context.Context, req *proto.ReceiveFromRefillRequest) (*proto.Cartridge, error) {
	slog.Info("Прием с заправки", "cartridge_id", req.CartridgeId, "comment", req.Comment)

	var cartridge models.Cartridge
	if err := s.db.First(&cartridge, "id = ?", req.CartridgeId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "картридж не найден: %s", req.CartridgeId)
		}
		return nil, status.Errorf(codes.Internal, "ошибка: %v", err)
	}

	// Проверяем валидность перехода
	if err := cartridge.CanReceiveFromRefill(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	// Транзакция БД
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Обновляем статус и инкрементируем счетчик заправок
		updates := map[string]interface{}{
			"status":        models.CartridgeStatusInUse,
			"total_refills": gorm.Expr("total_refills + 1"),
		}
		if err := tx.Model(&cartridge).Updates(updates).Error; err != nil {
			return err
		}

		// Создаем транзакцию операции
		transaction := models.Transaction{
			ID:          uuid.New().String(),
			CartridgeID: cartridge.ID,
			Type:        models.OperationTypeReceiveFromRefill,
			Timestamp:   time.Now(),
			Comment:     req.Comment,
		}
		return tx.Create(&transaction).Error
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при приеме: %v", err)
	}

	// Перечитываем картридж для получения актуального счетчика
	s.db.First(&cartridge, "id = ?", req.CartridgeId)
	return toProtoCartridge(&cartridge), nil
}

// RetireCartridge утилизирует картридж
func (s *OperationServiceServer) RetireCartridge(ctx context.Context, req *proto.RetireCartridgeRequest) (*proto.Cartridge, error) {
	slog.Info("Утилизация картриджа", "cartridge_id", req.CartridgeId, "comment", req.Comment)

	var cartridge models.Cartridge
	if err := s.db.First(&cartridge, "id = ?", req.CartridgeId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "картридж не найден: %s", req.CartridgeId)
		}
		return nil, status.Errorf(codes.Internal, "ошибка: %v", err)
	}

	// Проверяем валидность перехода
	if err := cartridge.CanRetire(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	now := time.Now()

	// Транзакция БД
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Обновляем статус и дату утилизации
		updates := map[string]interface{}{
			"status":     models.CartridgeStatusRetired,
			"retired_at": now,
		}
		if err := tx.Model(&cartridge).Updates(updates).Error; err != nil {
			return err
		}

		// Создаем транзакцию операции
		transaction := models.Transaction{
			ID:          uuid.New().String(),
			CartridgeID: cartridge.ID,
			Type:        models.OperationTypeRetirement,
			Timestamp:   now,
			Comment:     req.Comment,
		}
		return tx.Create(&transaction).Error
	})

	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при утилизации: %v", err)
	}

	cartridge.Status = models.CartridgeStatusRetired
	cartridge.RetiredAt = &now
	return toProtoCartridge(&cartridge), nil
}

// GetCartridgeHistory возвращает историю операций картриджа
func (s *OperationServiceServer) GetCartridgeHistory(ctx context.Context, req *proto.GetCartridgeHistoryRequest) (*proto.GetCartridgeHistoryResponse, error) {
	var transactions []models.Transaction
	result := s.db.Where("cartridge_id = ?", req.CartridgeId).Order("timestamp DESC").Find(&transactions)

	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении истории: %v", result.Error)
	}

	protoTransactions := make([]*proto.Transaction, len(transactions))
	for i, t := range transactions {
		protoTransactions[i] = toProtoTransaction(&t)
	}

	return &proto.GetCartridgeHistoryResponse{
		Transactions: protoTransactions,
	}, nil
}

// GenerateAct генерирует акт выдачи картриджей на заправку
func (s *OperationServiceServer) GenerateAct(ctx context.Context, req *proto.GenerateActRequest) (*wrapperspb.BytesValue, error) {
	if len(req.CartridgeIds) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "список картриджей не может быть пустым")
	}

	slog.Info("Генерация акта выдачи", "cartridge_count", len(req.CartridgeIds))

	// Получаем картриджи из БД
	var cartridges []models.Cartridge
	result := s.db.Where("id IN ?", req.CartridgeIds).Find(&cartridges)

	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении картриджей: %v", result.Error)
	}

	if len(cartridges) != len(req.CartridgeIds) {
		return nil, status.Errorf(codes.NotFound, "не все картриджи найдены")
	}

	// Генерируем Markdown контент акта
	content := s.generateActMarkdown(cartridges)

	return &wrapperspb.BytesValue{Value: []byte(content)}, nil
}

// generateActMarkdown генерирует Markdown контент акта
func (s *OperationServiceServer) generateActMarkdown(cartridges []models.Cartridge) string {
	var buf strings.Builder

	buf.WriteString("# <center>Акт выдачи картриджей на заправку</center>\n\n")
	buf.WriteString("Филиал АО \"Kaspi Bank\" в г. Петропавловск выдал на заправку\n")
	buf.WriteString("в ТОО \"Петроком Центр\" следующие картриджи\n\n")

	// Таблица с картриджами
	buf.WriteString("| № | ID     | Тип картриджа |\n")
	buf.WriteString("|---|--------|---------------|\n")

	for i, c := range cartridges {
		buf.WriteString(fmt.Sprintf("| %d | %-6s | %-13s |\n", i+1, c.ID, c.Model))
	}

	buf.WriteString(fmt.Sprintf("\nИтого выдано %d картриджей\n\n", len(cartridges)))

	// Подписи
	buf.WriteString("<table>\n")
	buf.WriteString("<tr>\n")
	buf.WriteString("<td width=\"50%\"><b>ФИО/Подпись заказчика</b><br>_____________________</td>\n")
	buf.WriteString("<td width=\"50%\"><b>ФИО/Подпись подрядчика</b><br>_____________________</td>\n")
	buf.WriteString("</tr>\n")
	buf.WriteString("</table>\n\n")

	// Дата
	buf.WriteString("<br>\n\n")
	buf.WriteString(fmt.Sprintf("**Дата:** %s\n", time.Now().Format("02.01.2006")))

	return buf.String()
}

// toProtoTransaction конвертирует модель транзакции в proto сообщение
func toProtoTransaction(t *models.Transaction) *proto.Transaction {
	return &proto.Transaction{
		Id:          t.ID,
		CartridgeId: t.CartridgeID,
		Type:        toProtoOperationType(t.Type),
		Timestamp:   timestamppb.New(t.Timestamp),
		Comment:     t.Comment,
	}
}

// toProtoOperationType конвертирует тип операции модели в proto тип
func toProtoOperationType(t models.OperationType) proto.OperationType {
	switch t {
	case models.OperationTypeRegistration:
		return proto.OperationType_OPERATION_TYPE_REGISTRATION
	case models.OperationTypeSendToRefill:
		return proto.OperationType_OPERATION_TYPE_SEND_TO_REFILL
	case models.OperationTypeReceiveFromRefill:
		return proto.OperationType_OPERATION_TYPE_RECEIVE_FROM_REFILL
	case models.OperationTypeRetirement:
		return proto.OperationType_OPERATION_TYPE_RETIREMENT
	default:
		return proto.OperationType_OPERATION_TYPE_UNSPECIFIED
	}
}

// AnalyticsServiceServer реализует сервис аналитики
type AnalyticsServiceServer struct {
	proto.UnimplementedAnalyticsServiceServer
	db *gorm.DB
}

// NewAnalyticsServiceServer создает новый сервис аналитики
func NewAnalyticsServiceServer(db *gorm.DB) *AnalyticsServiceServer {
	return &AnalyticsServiceServer{db: db}
}

// GetRefillsStats возвращает статистику заправок за период
func (s *AnalyticsServiceServer) GetRefillsStats(ctx context.Context, req *proto.RefillsStatsRequest) (*proto.RefillsStatsResponse, error) {
	var count int64
	var uniqueCartridges int64

	startTime := req.PeriodStart.AsTime()
	endTime := req.PeriodEnd.AsTime()

	// Считаем количество операций приема с заправки
	query := s.db.Model(&models.Transaction{}).
		Where("type = ? AND timestamp BETWEEN ? AND ?",
			models.OperationTypeReceiveFromRefill, startTime, endTime)

	query.Count(&count)

	// Считаем уникальные картриджи
	query.Distinct("cartridge_id").Count(&uniqueCartridges)

	slog.Info("Получение статистики заправок",
		"total_refills", count,
		"unique_cartridges", uniqueCartridges,
		"period_start", startTime,
		"period_end", endTime)

	return &proto.RefillsStatsResponse{
		TotalRefills:     int32(count),
		UniqueCartridges: int32(uniqueCartridges),
	}, nil
}

// GetGlobalStats возвращает общую статистику
func (s *AnalyticsServiceServer) GetGlobalStats(ctx context.Context, req *proto.GlobalStatsRequest) (*proto.GlobalStatsResponse, error) {
	var totalCartridges, inUse, refilling, retired int64
	var totalRefillsAllTime int32

	s.db.Model(&models.Cartridge{}).Count(&totalCartridges)
	s.db.Model(&models.Cartridge{}).Where("status = ?", models.CartridgeStatusInUse).Count(&inUse)
	s.db.Model(&models.Cartridge{}).Where("status = ?", models.CartridgeStatusRefilling).Count(&refilling)
	s.db.Model(&models.Cartridge{}).Where("status = ?", models.CartridgeStatusRetired).Count(&retired)

	// Суммируем все заправки
	s.db.Model(&models.Cartridge{}).Select("COALESCE(SUM(total_refills), 0)").Scan(&totalRefillsAllTime)

	return &proto.GlobalStatsResponse{
		TotalCartridges:     int32(totalCartridges),
		InUse:               int32(inUse),
		Refilling:           int32(refilling),
		Retired:             int32(retired),
		TotalRefillsAllTime: totalRefillsAllTime,
	}, nil
}

// ExportRefillsStats экспортирует статистику заправок в CSV или TXT формате
func (s *AnalyticsServiceServer) ExportRefillsStats(ctx context.Context, req *proto.ExportRefillsStatsRequest) (*wrapperspb.BytesValue, error) {
	startTime := req.PeriodStart.AsTime()
	endTime := req.PeriodEnd.AsTime()
	format := strings.ToLower(req.Format)
	if format == "" {
		format = "csv"
	}

	slog.Info("Экспорт статистики заправок", "format", format, "period_start", startTime, "period_end", endTime)

	// Получаем данные
	var transactions []models.Transaction
	query := s.db.Where("type = ? AND timestamp BETWEEN ? AND ?",
		models.OperationTypeReceiveFromRefill, startTime, endTime).
		Order("timestamp ASC").
		Find(&transactions)

	if query.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении данных: %v", query.Error)
	}

	// Генерируем контент в зависимости от формата
	var content []byte
	if format == "csv" {
		content = s.exportRefillsCSV(transactions)
	} else {
		content = s.exportRefillsTXT(transactions)
	}

	return &wrapperspb.BytesValue{Value: content}, nil
}

// exportRefillsCSV экспортирует данные в CSV формате
func (s *AnalyticsServiceServer) exportRefillsCSV(transactions []models.Transaction) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = ';' // Используем точку с запятой для совместимости с Excel

	// Заголовок
	if err := writer.Write([]string{"ID транзакции", "ID картриджа", "Дата", "Комментарий"}); err != nil {
		return []byte{}
	}

	for _, t := range transactions {
		if err := writer.Write([]string{
			t.ID,
			t.CartridgeID,
			t.Timestamp.Format("2006-01-02 15:04:05"),
			t.Comment,
		}); err != nil {
			return []byte{}
		}
	}

	writer.Flush()
	return buf.Bytes()
}

// exportRefillsTXT экспортирует данные в текстовом формате
func (s *AnalyticsServiceServer) exportRefillsTXT(transactions []models.Transaction) []byte {
	var buf bytes.Buffer

	buf.WriteString("Отчет по заправкам картриджей\n")
	buf.WriteString("==============================\n\n")
	buf.WriteString(fmt.Sprintf("Период: %s - %s\n\n",
		transactions[0].Timestamp.Format("2006-01-02"),
		transactions[len(transactions)-1].Timestamp.Format("2006-01-02")))

	buf.WriteString(fmt.Sprintf("%-40s %-20s %-25s %s\n", "ID транзакции", "ID картриджа", "Дата", "Комментарий"))
	buf.WriteString(strings.Repeat("-", 100) + "\n")

	for _, t := range transactions {
		comment := t.Comment
		if len(comment) > 30 {
			comment = comment[:30] + "..."
		}
		buf.WriteString(fmt.Sprintf("%-40s %-20s %-25s %s\n", t.ID, t.CartridgeID, t.Timestamp.Format("2006-01-02 15:04:05"), comment))
	}

	buf.WriteString("\n==============================\n")
	buf.WriteString(fmt.Sprintf("Всего заправок: %d\n", len(transactions)))

	return buf.Bytes()
}

// ExportCartridgeHistory экспортирует историю картриджа в CSV или TXT формате
func (s *AnalyticsServiceServer) ExportCartridgeHistory(ctx context.Context, req *proto.ExportCartridgeHistoryRequest) (*wrapperspb.BytesValue, error) {
	format := strings.ToLower(req.Format)
	if format == "" {
		format = "csv"
	}

	slog.Info("Экспорт истории картриджа", "cartridge_id", req.CartridgeId, "format", format)

	// Получаем историю
	var transactions []models.Transaction
	result := s.db.Where("cartridge_id = ?", req.CartridgeId).Order("timestamp ASC").Find(&transactions)

	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении истории: %v", result.Error)
	}

	// Генерируем контент
	var content []byte
	if format == "csv" {
		content = s.exportHistoryCSV(transactions)
	} else {
		content = s.exportHistoryTXT(transactions, req.CartridgeId)
	}

	return &wrapperspb.BytesValue{Value: content}, nil
}

// exportHistoryCSV экспортирует историю в CSV формате
func (s *AnalyticsServiceServer) exportHistoryCSV(transactions []models.Transaction) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'

	if err := writer.Write([]string{"ID транзакции", "Тип операции", "Дата", "Комментарий"}); err != nil {
		return []byte{}
	}

	for _, t := range transactions {
		if err := writer.Write([]string{
			t.ID,
			string(t.Type),
			t.Timestamp.Format("2006-01-02 15:04:05"),
			t.Comment,
		}); err != nil {
			return []byte{}
		}
	}

	writer.Flush()
	return buf.Bytes()
}

// exportHistoryTXT экспортирует историю в текстовом формате
func (s *AnalyticsServiceServer) exportHistoryTXT(transactions []models.Transaction, cartridgeID string) []byte {
	var buf bytes.Buffer

	buf.WriteString("История операций картриджа\n")
	buf.WriteString("===========================\n\n")
	buf.WriteString(fmt.Sprintf("ID картриджа: %s\n\n", cartridgeID))

	buf.WriteString(fmt.Sprintf("%-40s %-25s %-25s %s\n", "ID транзакции", "Тип операции", "Дата", "Комментарий"))
	buf.WriteString(strings.Repeat("-", 110) + "\n")

	for _, t := range transactions {
		comment := t.Comment
		if len(comment) > 25 {
			comment = comment[:25] + "..."
		}
		buf.WriteString(fmt.Sprintf("%-40s %-25s %-25s %s\n", t.ID, string(t.Type), t.Timestamp.Format("2006-01-02 15:04:05"), comment))
	}

	buf.WriteString("\n===========================\n")
	buf.WriteString(fmt.Sprintf("Всего операций: %d\n", len(transactions)))

	return buf.Bytes()
}

// HealthServiceServer реализует сервис health check
type HealthServiceServer struct {
	proto.UnimplementedHealthServiceServer
}

// NewHealthServiceServer создает новый сервис health check
func NewHealthServiceServer() *HealthServiceServer {
	return &HealthServiceServer{}
}

// Check выполняет проверку здоровья сервиса
func (s *HealthServiceServer) Check(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	slog.Debug("Health check passed")
	return &emptypb.Empty{}, nil
}

// ModelServiceServer реализует сервис управления моделями картриджей
type ModelServiceServer struct {
	proto.UnimplementedModelServiceServer
	db *gorm.DB
}

// NewModelServiceServer создает новый сервис управления моделями
func NewModelServiceServer(db *gorm.DB) *ModelServiceServer {
	return &ModelServiceServer{db: db}
}

// ListModels возвращает список моделей картриджей
func (s *ModelServiceServer) ListModels(ctx context.Context, req *proto.ListModelsRequest) (*proto.ListModelsResponse, error) {
	var cartridgeModels []models.CartridgeModel
	var total int64

	query := s.db.Model(&models.CartridgeModel{})

	// Поиск по названию
	if req.Search != "" {
		query = query.Where("name LIKE ?", "%"+req.Search+"%")
	}

	// Общее количество
	query.Count(&total)

	// Пагинация
	offset := (req.Page - 1) * req.PageSize
	if offset < 0 {
		offset = 0
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	// Сортировка по популярности
	result := query.Order("usage_count DESC, name ASC").Offset(int(offset)).Limit(int(pageSize)).Find(&cartridgeModels)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении списка моделей: %v", result.Error)
	}

	protoModels := make([]*proto.ModelItem, len(cartridgeModels))
	for i, m := range cartridgeModels {
		protoModels[i] = toProtoModelItem(&m)
	}

	return &proto.ListModelsResponse{
		Models:     protoModels,
		TotalCount: int32(total),
	}, nil
}

// UpsertModel создает или обновляет модель картриджа
func (s *ModelServiceServer) UpsertModel(ctx context.Context, req *proto.UpsertModelRequest) (*proto.ModelItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "название модели обязательно")
	}

	slog.Info("Upsert модели картриджа", "name", name)

	var model models.CartridgeModel

	// Пытаемся найти существующую модель
	result := s.db.Where("name = ?", name).First(&model)

	if result.Error == gorm.ErrRecordNotFound {
		// Создаем новую модель
		now := time.Now()
		model = models.CartridgeModel{
			Name:       name,
			UsageCount: 0,
			LastUsedAt: now,
			CreatedAt:  now,
		}

		if err := s.db.Create(&model).Error; err != nil {
			return nil, status.Errorf(codes.Internal, "ошибка при создании модели: %v", err)
		}

		slog.Info("Создана новая модель картриджа", "id", model.ID, "name", name)
	} else if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при поиске модели: %v", result.Error)
	}
	// Если модель найдена - возвращаем существующую

	return toProtoModelItem(&model), nil
}

// toProtoModelItem конвертирует модель БД в proto сообщение
func toProtoModelItem(m *models.CartridgeModel) *proto.ModelItem {
	return &proto.ModelItem{
		Id:         uint32(m.ID),
		Name:       m.Name,
		UsageCount: int32(m.UsageCount),
		LastUsedAt: timestamppb.New(m.LastUsedAt),
		CreatedAt:  timestamppb.New(m.CreatedAt),
	}
}
