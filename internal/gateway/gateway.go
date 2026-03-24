package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mioxin/kartg/api/proto"
)

// Config содержит конфигурацию для gateway
type Config struct {
	GRPCAddress string
	HTTPAddress string
}

// Run запускает HTTP gateway сервер
func Run(ctx context.Context, cfg Config) error {
	// Создаем mux для обработки HTTP запросов
	mux := runtime.NewServeMux()

	// Создаем подключение к gRPC серверу
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Регистрируем handlers для каждого сервиса
	if err := proto.RegisterCartridgeServiceHandlerFromEndpoint(ctx, mux, cfg.GRPCAddress, opts); err != nil {
		return fmt.Errorf("failed to register CartridgeService: %w", err)
	}

	if err := proto.RegisterOperationServiceHandlerFromEndpoint(ctx, mux, cfg.GRPCAddress, opts); err != nil {
		return fmt.Errorf("failed to register OperationService: %w", err)
	}

	if err := proto.RegisterAnalyticsServiceHandlerFromEndpoint(ctx, mux, cfg.GRPCAddress, opts); err != nil {
		return fmt.Errorf("failed to register AnalyticsService: %w", err)
	}

	if err := proto.RegisterHealthServiceHandlerFromEndpoint(ctx, mux, cfg.GRPCAddress, opts); err != nil {
		return fmt.Errorf("failed to register HealthService: %w", err)
	}

	// Добавляем HTTP обработчики для экспорта
	mux.HandlePath("GET", "/api/v1/export/refills", handleExportRefills(cfg.GRPCAddress))
	mux.HandlePath("GET", "/api/v1/export/cartridge/{cartridge_id}/history", handleExportCartridgeHistory(cfg.GRPCAddress))

	// Добавляем CORS заголовки
	handler := withCORS(mux)

	// Запускаем HTTP сервер
	addr := cfg.HTTPAddress
	fmt.Printf("HTTP Gateway запущен на %s\n", addr)
	return http.ListenAndServe(addr, handler)
}

// handleExportRefills обрабатывает запросы на экспорт статистики заправок
func handleExportRefills(grpcAddress string) func(http.ResponseWriter, *http.Request, map[string]string) {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		ctx := r.Context()

		// Получаем подключение к gRPC
		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка подключения: %v", err), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := proto.NewAnalyticsServiceClient(conn)

		// Получаем параметры
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "csv"
		}

		periodStartStr := r.URL.Query().Get("period_start")
		periodEndStr := r.URL.Query().Get("period_end")

		var periodStart, periodEnd *time.Time
		if periodStartStr != "" {
			t, _ := time.Parse("2006-01-02", periodStartStr)
			periodStart = &t
		}
		if periodEndStr != "" {
			t, _ := time.Parse("2006-01-02", periodEndStr)
			periodEnd = &t
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
		}

		// Запрос к gRPC сервису
		resp, err := client.ExportRefillsStats(ctx, &proto.ExportRefillsStatsRequest{
			PeriodStart: timestamppb.New(*periodStart),
			PeriodEnd:   timestamppb.New(*periodEnd),
			Format:      format,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка при экспорте: %v", err), http.StatusInternalServerError)
			return
		}

		// Устанавливаем заголовки для скачивания файла
		filename := fmt.Sprintf("refills_stats_%s_%s.%s",
			periodStart.Format("20060102"),
			periodEnd.Format("20060102"),
			format,
		)
		contentType := "text/csv; charset=utf-8"
		if format != "csv" {
			contentType = "text/plain; charset=utf-8"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.Value)))
		w.Write(resp.Value)
	}
}

// handleExportCartridgeHistory обрабатывает запросы на экспорт истории картриджа
func handleExportCartridgeHistory(grpcAddress string) func(http.ResponseWriter, *http.Request, map[string]string) {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		ctx := r.Context()

		// Получаем подключение к gRPC
		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка подключения: %v", err), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := proto.NewOperationServiceClient(conn)

		// Получаем параметры
		cartridgeID := pathParams["cartridge_id"]
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "csv"
		}

		// Запрос к gRPC сервису
		resp, err := client.ExportCartridgeHistory(ctx, &proto.ExportCartridgeHistoryRequest{
			CartridgeId: cartridgeID,
			Format:      format,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("Ошибка при экспорте: %v", err), http.StatusInternalServerError)
			return
		}

		// Устанавливаем заголовки для скачивания файла
		filename := fmt.Sprintf("cartridge_%s_history.%s", cartridgeID, format)
		contentType := "text/csv; charset=utf-8"
		if format != "csv" {
			contentType = "text/plain; charset=utf-8"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(resp.Value)))
		w.Write(resp.Value)
	}
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
