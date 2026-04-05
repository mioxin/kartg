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

// CartridgeStats содержит статистику заправок по картриджу
type CartridgeStats struct {
	CartridgeID     string
	Model           string
	RefillsInPeriod int
	TotalRefills    int
}

// CartridgeRepository определяет интерфейс для работы с картриджами
// Это позволяет соблюдать Dependency Inversion Principle и упрощает тестирование
type CartridgeRepository interface {
	// FindByID находит картридж по ID
	FindByID(id string) (*models.Cartridge, error)
	// Register регистрирует новый картридж (создание или обновление)
	Register(id, model string) (*models.Cartridge, error)
	// Update обновляет существующий картридж
	Update(cartridge *models.Cartridge) error
	// UpdateStatus обновляет статус картриджа
	UpdateStatus(id string, status models.CartridgeStatus) error
	// IncrementRefills увеличивает счётчик заправок
	IncrementRefills(id string) error
	// List возвращает список картриджей с пагинацией
	List(search string, page, pageSize int32) ([]models.Cartridge, int64, error)
	// ListWithStatus возвращает список картриджей с пагинацией и фильтром по статусу
	ListWithStatus(search string, status models.CartridgeStatus, page, pageSize int32) ([]models.Cartridge, int64, error)
}

// GORMRepository реализует CartridgeRepository с использованием GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository создаёт новый репозиторий
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// WithTransaction возвращает репозиторий в контексте транзакции
func (r *GORMRepository) WithTransaction(tx *gorm.DB) CartridgeRepository {
	return &GORMRepository{db: tx}
}

// FindByID находит картридж по ID
func (r *GORMRepository) FindByID(id string) (*models.Cartridge, error) {
	return getCartridgeByID(r.db, id)
}

// Register регистрирует новый картридж (создание или обновление)
func (r *GORMRepository) Register(id, model string) (*models.Cartridge, error) {
	var cartridge models.Cartridge

	// Транзакция для атомарности
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Создаём или находим картридж
		result := tx.FirstOrCreate(&cartridge, models.Cartridge{ID: id}, map[string]interface{}{
			"model":         model,
			"status":        models.CartridgeStatusInUse,
			"total_refills": 0,
			"created_at":    time.Now(),
		})

		if result.Error != nil {
			return fmt.Errorf("ошибка при регистрации: %w", result.Error)
		}

		// Если картридж уже существовал и модель отличается - обновляем
		if result.RowsAffected == 0 && cartridge.Model != model && model != "" {
			slog.Info("Обновление модели картриджа", "id", id, "old_model", cartridge.Model, "new_model", model)
			tx.Model(&cartridge).Update("model", model)
			cartridge.Model = model
		}

		// Сохраняем модель в справочник
		if model != "" {
			var cartridgeModel models.CartridgeModel
			now := time.Now()

			modelResult := tx.Where("name = ?", model).First(&cartridgeModel)

			if modelResult.Error == gorm.ErrRecordNotFound {
				// Создаем новую модель
				cartridgeModel = models.CartridgeModel{
					Name:       model,
					UsageCount: 1,
					LastUsedAt: now,
					CreatedAt:  now,
				}
				if err := tx.Create(&cartridgeModel).Error; err != nil {
					return fmt.Errorf("ошибка при создании модели: %w", err)
				}
				slog.Info("Создана новая модель в справочнике", "name", model)
			} else if modelResult.Error == nil {
				// Обновляем счетчик использования
				tx.Model(&cartridgeModel).Updates(map[string]interface{}{
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

	return &cartridge, nil
}

// Update обновляет существующий картридж
func (r *GORMRepository) Update(cartridge *models.Cartridge) error {
	return r.db.Save(cartridge).Error
}

// UpdateStatus обновляет статус картриджа
func (r *GORMRepository) UpdateStatus(id string, status models.CartridgeStatus) error {
	return r.db.Model(&models.Cartridge{}).Where("id = ?", id).Update("status", status).Error
}

// IncrementRefills увеличивает счётчик заправок
func (r *GORMRepository) IncrementRefills(id string) error {
	return r.db.Model(&models.Cartridge{}).Where("id = ?", id).Update("total_refills", gorm.Expr("total_refills + 1")).Error
}

// List возвращает список картриджей с пагинацией
func (r *GORMRepository) List(search string, page, pageSize int32) ([]models.Cartridge, int64, error) {
	var cartridges []models.Cartridge
	var total int64

	query := r.db.Model(&models.Cartridge{})

	if search != "" {
		query = query.Where("id LIKE ? OR model LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Count(&total)

	offset, limit := calculatePagination(page, pageSize)

	result := query.Offset(offset).Limit(limit).Find(&cartridges)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return cartridges, total, nil
}

// ListWithStatus возвращает список картриджей с пагинацией и фильтром по статусу
func (r *GORMRepository) ListWithStatus(search string, status models.CartridgeStatus, page, pageSize int32) ([]models.Cartridge, int64, error) {
	var cartridges []models.Cartridge
	var total int64

	query := r.db.Model(&models.Cartridge{})

	if search != "" {
		query = query.Where("id LIKE ? OR model LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset, limit := calculatePagination(page, pageSize)

	result := query.Offset(offset).Limit(limit).Find(&cartridges)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return cartridges, total, nil
}

// getCartridgeByID извлекает картридж по ID с обработкой ошибок
func getCartridgeByID(tx *gorm.DB, id string) (*models.Cartridge, error) {
	var cartridge models.Cartridge
	if err := tx.First(&cartridge, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "картридж не найден: %s", id)
		}
		return nil, status.Errorf(codes.Internal, "ошибка: %v", err)
	}
	return &cartridge, nil
}

// writeCSVHeader записывает заголовок CSV
func writeCSVHeader(writer *csv.Writer, columns []string) error {
	return writer.Write(columns)
}

// writeCSVRow записывает строку CSV
func writeCSVRow(writer *csv.Writer, row []string) error {
	return writer.Write(row)
}

// writeTXTHeader записывает заголовок текстового отчета
func writeTXTHeader(buf *bytes.Buffer, title string, underline string) {
	buf.WriteString(title + "\n")
	buf.WriteString(underline + "\n\n")
}

// writeTXTFooter записывает подвал текстового отчета
func writeTXTFooter(buf *bytes.Buffer, separator string, summary string) {
	buf.WriteString("\n" + separator + "\n")
	buf.WriteString(summary + "\n")
}

// writeTXTTableHeader записывает заголовок таблицы текстового отчета
func writeTXTTableHeader(buf *bytes.Buffer, format string, args ...interface{}) {
	buf.WriteString(fmt.Sprintf(format, args...))
}

// writeTXTTableRow записывает строку таблицы текстового отчета
func writeTXTTableRow(buf *bytes.Buffer, format string, args ...interface{}) {
	buf.WriteString(fmt.Sprintf(format, args...))
}

// calculatePagination вычисляет offset и limit для пагинации
func calculatePagination(page, pageSize int32) (offset, limit int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	offset = int((page - 1) * pageSize)
	limit = int(pageSize)
	return
}

// CartridgeServiceServer реализует сервис управления картриджами
type CartridgeServiceServer struct {
	proto.UnimplementedCartridgeServiceServer
	repo CartridgeRepository
}

// NewCartridgeServiceServer создает новый сервис картриджей
func NewCartridgeServiceServer(repo CartridgeRepository) *CartridgeServiceServer {
	return &CartridgeServiceServer{repo: repo}
}

// RegisterCartridge регистрирует новый картридж
func (s *CartridgeServiceServer) RegisterCartridge(ctx context.Context, req *proto.RegisterCartridgeRequest) (*proto.Cartridge, error) {
	slog.Info("Регистрация картриджа", "id", req.Id, "model", req.Model)

	cartridge, err := s.repo.Register(req.Id, req.Model)
	if err != nil {
		return nil, err
	}

	return toProtoCartridge(cartridge), nil
}

// GetCartridge получает информацию о картридже
func (s *CartridgeServiceServer) GetCartridge(ctx context.Context, req *proto.GetCartridgeRequest) (*proto.Cartridge, error) {
	cartridge, err := s.repo.FindByID(req.Id)
	if err != nil {
		return nil, err
	}

	return toProtoCartridge(cartridge), nil
}

// ListCartridges возвращает список картриджей с пагинацией
func (s *CartridgeServiceServer) ListCartridges(ctx context.Context, req *proto.ListCartridgesRequest) (*proto.ListCartridgesResponse, error) {
	// Преобразуем статус из proto в model
	var cartridgeStatus models.CartridgeStatus
	if req.Status != proto.CartridgeStatus_CARTRIDGE_STATUS_UNSPECIFIED {
		cartridgeStatus = fromProtoStatus(req.Status)
	}

	cartridges, total, err := s.repo.ListWithStatus(req.Search, cartridgeStatus, req.Page, req.PageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении списка: %v", err)
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

	cartridge, err := getCartridgeByID(s.db, req.CartridgeId)
	if err != nil {
		return nil, err
	}

	// Проверяем валидность перехода
	if err := cartridge.CanSendToRefill(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	// Транзакция БД
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
	return toProtoCartridge(cartridge), nil
}

// ReceiveFromRefill принимает картридж с заправки
func (s *OperationServiceServer) ReceiveFromRefill(ctx context.Context, req *proto.ReceiveFromRefillRequest) (*proto.Cartridge, error) {
	slog.Info("Прием с заправки", "cartridge_id", req.CartridgeId, "comment", req.Comment)

	cartridge, err := getCartridgeByID(s.db, req.CartridgeId)
	if err != nil {
		return nil, err
	}

	// Проверяем валидность перехода
	if err := cartridge.CanReceiveFromRefill(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	// Транзакция БД
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
	var updatedCartridge models.Cartridge
	s.db.First(&updatedCartridge, "id = ?", req.CartridgeId)
	return toProtoCartridge(&updatedCartridge), nil
}

// RetireCartridge утилизирует картридж
func (s *OperationServiceServer) RetireCartridge(ctx context.Context, req *proto.RetireCartridgeRequest) (*proto.Cartridge, error) {
	slog.Info("Утилизация картриджа", "cartridge_id", req.CartridgeId, "comment", req.Comment)

	cartridge, err := getCartridgeByID(s.db, req.CartridgeId)
	if err != nil {
		return nil, err
	}

	// Проверяем валидность перехода
	if err := cartridge.CanRetire(); err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	}

	now := time.Now()

	// Транзакция БД
	err = s.db.Transaction(func(tx *gorm.DB) error {
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
	return toProtoCartridge(cartridge), nil
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

	// Генерируем HTML контент акта
	content := generateActHTML(cartridges)

	return &wrapperspb.BytesValue{Value: []byte(content)}, nil
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

	startTime := req.PeriodStart.AsTime().Local()
	endTime := req.PeriodEnd.AsTime().Local()

	// Считаем количество операций приема с заправки
	countQuery := s.db.Model(&models.Transaction{}).
		Where("type = ? AND timestamp BETWEEN ? AND ?",
			models.OperationTypeReceiveFromRefill, startTime, endTime)
	if err := countQuery.Count(&count).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при подсчете заправок: %v", err)
	}

	// Считаем уникальные картриджи
	distinctQuery := s.db.Model(&models.Transaction{}).
		Distinct("cartridge_id").
		Where("type = ? AND timestamp BETWEEN ? AND ?",
			models.OperationTypeReceiveFromRefill, startTime, endTime)
	if err := distinctQuery.Count(&uniqueCartridges).Error; err != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при подсчете уникальных картриджей: %v", err)
	}

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

	// Получаем транзакции заправок за период
	var transactions []models.Transaction
	query := s.db.Where("type = ? AND timestamp BETWEEN ? AND ?",
		models.OperationTypeReceiveFromRefill, startTime, endTime).
		Order("timestamp ASC").
		Find(&transactions)

	if query.Error != nil {
		return nil, status.Errorf(codes.Internal, "ошибка при получении данных: %v", query.Error)
	}

	// Агрегируем данные по картриджам
	// Считаем заправки за период по каждому картриджу
	cartridgeRefills := make(map[string]*CartridgeStats)
	for _, t := range transactions {
		if _, exists := cartridgeRefills[t.CartridgeID]; !exists {
			cartridgeRefills[t.CartridgeID] = &CartridgeStats{
				CartridgeID: t.CartridgeID,
			}
		}
		cartridgeRefills[t.CartridgeID].RefillsInPeriod++
	}

	// Получаем модели и общее количество заправок для каждого картриджа
	for id, stats := range cartridgeRefills {
		var cartridge models.Cartridge
		if err := s.db.First(&cartridge, "id = ?", id).Error; err == nil {
			stats.Model = cartridge.Model
			stats.TotalRefills = cartridge.TotalRefills
		}
	}

	// Преобразуем в слайс для сохранения порядка
	statsSlice := make([]CartridgeStats, 0, len(cartridgeRefills))
	for _, stats := range cartridgeRefills {
		statsSlice = append(statsSlice, *stats)
	}

	// Генерируем контент в зависимости от формата
	var content []byte
	if format == "csv" {
		content = s.exportRefillsCSV(transactions, statsSlice)
	} else {
		content = s.exportRefillsTXT(transactions, statsSlice)
	}

	return &wrapperspb.BytesValue{Value: content}, nil
}

// exportRefillsCSV экспортирует данные в CSV формате
func (s *AnalyticsServiceServer) exportRefillsCSV(transactions []models.Transaction, statsSlice []CartridgeStats) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = ';' // Используем точку с запятой для совместимости с Excel

	// === Таблица 1: Детализация транзакций ===
	if err := writeCSVHeader(writer, []string{"№ п/п", "ID транзакции", "ID картриджа", "Дата", "Комментарий"}); err != nil {
		return []byte{}
	}

	for i, t := range transactions {
		if err := writeCSVRow(writer, []string{
			fmt.Sprintf("%d", i+1),
			t.ID,
			t.CartridgeID,
			t.Timestamp.Format("2006-01-02 15:04:05"),
			t.Comment,
		}); err != nil {
			return []byte{}
		}
	}

	// Пустая строка-разделитель
	if err := writer.Write([]string{""}); err != nil {
		return []byte{}
	}

	// === Таблица 2: Статистика по картриджам ===
	if err := writeCSVHeader(writer, []string{"№ п/п", "ID картриджа", "Тип картриджа", "Заправок за период", "Заправок с начала регистрации"}); err != nil {
		return []byte{}
	}

	for i, stats := range statsSlice {
		if err := writeCSVRow(writer, []string{
			fmt.Sprintf("%d", i+1),
			stats.CartridgeID,
			stats.Model,
			fmt.Sprintf("%d", stats.RefillsInPeriod),
			fmt.Sprintf("%d", stats.TotalRefills),
		}); err != nil {
			return []byte{}
		}
	}

	writer.Flush()
	return buf.Bytes()
}

// exportRefillsTXT экспортирует данные в текстовом формате
func (s *AnalyticsServiceServer) exportRefillsTXT(transactions []models.Transaction, statsSlice []CartridgeStats) []byte {
	var buf bytes.Buffer

	// === Таблица 1: Детализация транзакций ===
	writeTXTHeader(&buf, "Отчет по заправкам картриджей", "==============================")

	// Период
	if len(transactions) > 0 {
		buf.WriteString(fmt.Sprintf("Период: %s - %s\n\n",
			transactions[0].Timestamp.Format("2006-01-02"),
			transactions[len(transactions)-1].Timestamp.Format("2006-01-02")))
	}

	buf.WriteString(fmt.Sprintf("%-6s %-40s %-20s %-25s %s\n", "№ п/п", "ID транзакции", "ID картриджа", "Дата", "Комментарий"))
	buf.WriteString(strings.Repeat("-", 115) + "\n")

	for i, t := range transactions {
		comment := t.Comment
		if len(comment) > 20 {
			comment = comment[:20] + "..."
		}
		writeTXTTableRow(&buf, "%-6d %-40s %-20s %-25s %s\n",
			i+1,
			t.ID,
			t.CartridgeID,
			t.Timestamp.Format("2006-01-02 15:04:05"),
			comment)
	}

	buf.WriteString(strings.Repeat("-", 115) + "\n")
	buf.WriteString(fmt.Sprintf("\nВсего заправок: %d\n\n", len(transactions)))

	// === Разделитель ===
	buf.WriteString("\n")

	// === Таблица 2: Статистика по картриджам ===
	buf.WriteString(fmt.Sprintf("%-6s %-25s %-20s %-20s %s\n", "№ п/п", "ID картриджа", "Тип картриджа", "Заправок за период", "Заправок всего"))
	buf.WriteString(strings.Repeat("-", 95) + "\n")

	totalPeriod := 0
	totalAll := 0

	for i, stats := range statsSlice {
		writeTXTTableRow(&buf, "%-6d %-25s %-20s %-20d %d\n",
			i+1,
			stats.CartridgeID,
			stats.Model,
			stats.RefillsInPeriod,
			stats.TotalRefills)
		totalPeriod += stats.RefillsInPeriod
		totalAll += stats.TotalRefills
	}

	buf.WriteString(strings.Repeat("-", 95) + "\n")
	writeTXTFooter(&buf, "==============================", fmt.Sprintf("Итого картриджей: %d, заправок за период: %d, всего заправок: %d", len(statsSlice), totalPeriod, totalAll))

	return buf.Bytes()
}

// ExportCartridgeHistory экспортирует историю картриджа в CSV или TXT формате
func (s *OperationServiceServer) ExportCartridgeHistory(ctx context.Context, req *proto.ExportCartridgeHistoryRequest) (*wrapperspb.BytesValue, error) {
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
func (s *OperationServiceServer) exportHistoryCSV(transactions []models.Transaction) []byte {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'

	if err := writeCSVHeader(writer, []string{"ID транзакции", "Тип операции", "Дата", "Комментарий"}); err != nil {
		return []byte{}
	}

	for _, t := range transactions {
		if err := writeCSVRow(writer, []string{
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
func (s *OperationServiceServer) exportHistoryTXT(transactions []models.Transaction, cartridgeID string) []byte {
	var buf bytes.Buffer

	writeTXTHeader(&buf, "История операций картриджа", "===========================")
	buf.WriteString(fmt.Sprintf("ID картриджа: %s\n\n", cartridgeID))

	writeTXTTableHeader(&buf, "%-40s %-25s %-25s %s\n", "ID транзакции", "Тип операции", "Дата", "Комментарий")
	buf.WriteString(strings.Repeat("-", 110) + "\n")

	for _, t := range transactions {
		comment := t.Comment
		if len(comment) > 25 {
			comment = comment[:25] + "..."
		}
		writeTXTTableRow(&buf, "%-40s %-25s %-25s %s\n", t.ID, string(t.Type), t.Timestamp.Format("2006-01-02 15:04:05"), comment)
	}

	writeTXTFooter(&buf, "===========================", fmt.Sprintf("Всего операций: %d", len(transactions)))

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
	offset, limit := calculatePagination(req.Page, req.PageSize)

	// Сортировка по популярности
	result := query.Order("usage_count DESC, name ASC").Offset(offset).Limit(limit).Find(&cartridgeModels)
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
