package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/i18n"
)

// Config содержит конфигурацию для gateway
type Config struct {
	GRPCAddress string
	HTTPAddress string
}

// Gateway представляет HTTP gateway сервер
type Gateway struct {
	mux        *runtime.ServeMux
	clientPool *ClientPool
	logger     *slog.Logger
	config     Config
	handler    http.Handler // Кэшированный handler с middleware
}

// ServeHTTP реализует интерфейс http.Handler
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.handler == nil {
		g.handler = i18n.LanguageMiddleware(withCORS(withAuthMetadata(g.mux)))
	}
	g.handler.ServeHTTP(w, r)
}

// ClientPool содержит пул gRPC клиентов
type ClientPool struct {
	conn            *grpc.ClientConn
	analyticsClient proto.AnalyticsServiceClient
	operationClient proto.OperationServiceClient
	cartridgeClient proto.CartridgeServiceClient
}

// NewClientPool создает пул gRPC клиентов
func NewClientPool(ctx context.Context, grpcAddress string) (*ClientPool, error) {
	// nolint:staticcheck // grpc.DialContext устарел, но будет поддерживаться в 1.x
	conn, err := grpc.DialContext(ctx, grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection: %w", err)
	}

	return &ClientPool{
		conn:            conn,
		analyticsClient: proto.NewAnalyticsServiceClient(conn),
		operationClient: proto.NewOperationServiceClient(conn),
		cartridgeClient: proto.NewCartridgeServiceClient(conn),
	}, nil
}

// Close закрывает подключение
func (p *ClientPool) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// NewGateway создает новый gateway сервер
func NewGateway(ctx context.Context, cfg Config) (*Gateway, error) {
	logger := slog.Default()

	pool, err := NewClientPool(ctx, cfg.GRPCAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create client pool: %w", err)
	}

	g := &Gateway{
		mux:        runtime.NewServeMux(),
		clientPool: pool,
		logger:     logger,
		config:     cfg,
	}

	if err := g.registerHandlers(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return g, nil
}

// registerHandlers регистрирует все HTTP handlers
func (g *Gateway) registerHandlers(ctx context.Context) error {
	// Регистрируем handlers для gRPC сервисов
	handlers := []struct {
		name     string
		register func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
	}{
		{"CartridgeService", proto.RegisterCartridgeServiceHandlerFromEndpoint},
		{"OperationService", proto.RegisterOperationServiceHandlerFromEndpoint},
		{"AnalyticsService", proto.RegisterAnalyticsServiceHandlerFromEndpoint},
		{"HealthService", proto.RegisterHealthServiceHandlerFromEndpoint},
		{"AuthService", proto.RegisterAuthServiceHandlerFromEndpoint},
		{"ModelService", proto.RegisterModelServiceHandlerFromEndpoint},
	}

	for _, h := range handlers {
		if err := h.register(ctx, g.mux, g.config.GRPCAddress, []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}); err != nil {
			return fmt.Errorf("failed to register %s: %w", h.name, err)
		}
	}

	// Регистрируем handlers для экспорта
	// nolint:errcheck // HandlePath не возвращает ошибку
	g.mux.HandlePath("GET", "/api/v1/export/refills", g.handleExportRefills())
	g.mux.HandlePath("GET", "/api/v1/export/cartridge/{cartridge_id}/history", g.handleExportCartridgeHistory()) // nolint:errcheck

	// Handler для генерации акта выдачи
	g.mux.HandlePath("POST", "/api/v1/operations/generate-act", g.handleGenerateAct()) // nolint:errcheck

	return nil
}

// Start запускает HTTP сервер
func (g *Gateway) Start() error {
	handler := i18n.LanguageMiddleware(withCORS(withAuthMetadata(g.mux)))

	g.logger.Info("HTTP Gateway started",
		"address", g.config.HTTPAddress,
		"grpc_address", g.config.GRPCAddress)

	return http.ListenAndServe(g.config.HTTPAddress, handler)
}

// Close закрывает gateway
func (g *Gateway) Close() error {
	return g.clientPool.Close()
}

// Run запускает HTTP gateway сервер (обратная совместимость)
func Run(ctx context.Context, cfg Config) error {
	gw, err := NewGateway(ctx, cfg)
	if err != nil {
		return err
	}
	defer gw.Close()

	return gw.Start()
}

// handleExportRefills обрабатывает запросы на экспорт статистики заправок
func (g *Gateway) handleExportRefills() func(http.ResponseWriter, *http.Request, map[string]string) {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Получаем параметры
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "csv"
		}

		periodStart, err := parseDate(r.URL.Query().Get("period_start"))
		if err != nil {
			g.sendError(w, "Invalid period_start format", http.StatusBadRequest)
			return
		}

		periodEnd, err := parseDate(r.URL.Query().Get("period_end"))
		if err != nil {
			g.sendError(w, "Invalid period_end format", http.StatusBadRequest)
			return
		}

		// Если период не указан, используем текущий месяц
		if periodStart == nil {
			now := time.Now()
			t := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			periodStart = &t
		}
		if periodEnd == nil {
			now := time.Now()
			periodEnd = &now
		} else {
			endOfDay := time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(), 23, 59, 59, int(999*time.Millisecond), periodEnd.Location())
			periodEnd = &endOfDay
		}

		// Запрос к gRPC сервису
		resp, err := g.clientPool.analyticsClient.ExportRefillsStats(ctx, &proto.ExportRefillsStatsRequest{
			PeriodStart: timestamppb.New(*periodStart),
			PeriodEnd:   timestamppb.New(*periodEnd),
			Format:      format,
		})
		if err != nil {
			g.logger.Error("Export refills stats failed",
				"error", err,
				"format", format,
				"period_start", periodStart.Format("2006-01-02"),
				"period_end", periodEnd.Format("2006-01-02"))
			g.sendError(w, "export.error", http.StatusInternalServerError)
			return
		}

		// Отправляем файл
		filename := fmt.Sprintf("refills_stats_%s_%s.%s",
			periodStart.Format("20060102"),
			periodEnd.Format("20060102"),
			format,
		)
		g.sendFile(w, resp.Value, filename, format)
	}
}

// handleExportCartridgeHistory обрабатывает запросы на экспорт истории картриджа
func (g *Gateway) handleExportCartridgeHistory() func(http.ResponseWriter, *http.Request, map[string]string) {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		cartridgeID := pathParams["cartridge_id"]
		if cartridgeID == "" {
			g.sendError(w, "cartridge_id is required", http.StatusBadRequest)
			return
		}

		format := r.URL.Query().Get("format")
		if format == "" {
			format = "csv"
		}

		// Запрос к gRPC сервису
		resp, err := g.clientPool.operationClient.ExportCartridgeHistory(ctx, &proto.ExportCartridgeHistoryRequest{
			CartridgeId: cartridgeID,
			Format:      format,
		})
		if err != nil {
			g.logger.Error("Export cartridge history failed",
				"error", err,
				"cartridge_id", cartridgeID,
				"format", format)
			g.sendError(w, "export.error", http.StatusInternalServerError)
			return
		}

		// Отправляем файл
		filename := fmt.Sprintf("cartridge_%s_history.%s", cartridgeID, format)
		g.sendFile(w, resp.Value, filename, format)
	}
}

// handleGenerateAct обрабатывает запросы на генерацию акта выдачи
func (g *Gateway) handleGenerateAct() func(http.ResponseWriter, *http.Request, map[string]string) {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		if r.Method != http.MethodPost {
			g.sendError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		// Читаем тело запроса
		body, err := io.ReadAll(r.Body)
		if err != nil {
			g.sendError(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Парсим запрос
		var req proto.GenerateActRequest
		if err := json.Unmarshal(body, &req); err != nil {
			g.sendError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		// Запрос к gRPC сервису
		resp, err := g.clientPool.operationClient.GenerateAct(ctx, &req)
		if err != nil {
			g.logger.Error("Generate act failed", "error", err)
			st, _ := status.FromError(err)
			switch st.Code() {
			case codes.InvalidArgument:
				g.sendError(w, st.Message(), http.StatusBadRequest)
			case codes.NotFound:
				g.sendError(w, st.Message(), http.StatusNotFound)
			case codes.FailedPrecondition:
				g.sendError(w, st.Message(), http.StatusConflict)
			default:
				g.sendError(w, st.Message(), http.StatusInternalServerError)
			}
			return
		}

		// Отправляем HTML напрямую
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.Value)))
		if _, err := w.Write(resp.Value); err != nil {
			g.logger.Error("Failed to write response", "error", err)
		}
	}
}

// parseDate парсит дату из строки
func parseDate(dateStr string) (*time.Time, error) {
	if dateStr == "" {
		return nil, nil
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// sendFile отправляет файл с правильными заголовками
func (g *Gateway) sendFile(w http.ResponseWriter, data []byte, filename string, format string) {
	contentType := "text/csv; charset=utf-8"
	if format != "csv" {
		contentType = "text/plain; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Используем буферизированную запись для эффективности
	buf := &bytes.Buffer{}
	buf.Write(data)
	if _, err := w.Write(buf.Bytes()); err != nil {
		g.logger.Error("Failed to write response", "error", err)
	}
}

// sendError отправляет ошибку с локализацией
func (g *Gateway) sendError(w http.ResponseWriter, message string, statusCode int) {
	// Для простых ошибок используем plain text
	http.Error(w, message, statusCode)
}

// withCORS добавляет CORS заголовки
func withCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// withAuthMetadata добавляет HTTP заголовки авторизации в gRPC metadata
func withAuthMetadata(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем токен из HTTP заголовка
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// Добавляем в контекст для передачи в gRPC
			ctx := r.Context()
			md := metadata.Pairs("authorization", authHeader)
			ctx = metadata.NewOutgoingContext(ctx, md)
			r = r.WithContext(ctx)
		}

		handler.ServeHTTP(w, r)
	})
}
