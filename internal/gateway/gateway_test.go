package gateway

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mioxin/kartg/internal/database"
	"github.com/mioxin/kartg/internal/models"
	"github.com/mioxin/kartg/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// setupTestGateway создает тестовый gateway
func setupTestGateway(b *testing.B) (*Gateway, func()) {
	// Создаем тестовую БД
	db, err := database.New(database.Config{
		DBPath:   ":memory:",
		LogLevel: "error",
	})
	if err != nil {
		b.Fatalf("Failed to create test DB: %v", err)
	}

	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}); err != nil {
		b.Fatalf("Failed to migrate: %v", err)
	}

	// Создаем тестовый gRPC сервер
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()

	_ = service.NewCartridgeServiceServer(db)
	_ = service.NewOperationServiceServer(db)
	_ = service.NewAnalyticsServiceServer(db)

	cleanup := func() {
		grpcServer.Stop()
		lis.Close()
	}

	// Создаем gateway с mock подключением
	ctx := context.Background()
	gw := &Gateway{
		config: Config{
			GRPCAddress: "bufnet",
			HTTPAddress: ":8080",
		},
		logger: nil, // В тестах не используем логгер
	}

	// Создаем подключение для тестов
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cleanup()
		b.Skipf("Skipping benchmark: %v", err)
	}

	gw.clientPool = &ClientPool{
		conn: conn,
	}

	return gw, cleanup
}

// BenchmarkOldCreateConnectionPerRequest бенчмарк старой версии (создание подключения на каждый запрос)
func BenchmarkOldCreateConnectionPerRequest(b *testing.B) {
	ctx := context.Background()
	grpcAddress := "localhost:50051"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Старый подход: создание подключения на каждый запрос
		conn, err := grpc.DialContext(ctx, grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			b.Fatal(err)
		}

		// Эмуляция работы
		time.Sleep(1 * time.Millisecond)

		conn.Close()
	}
}

// BenchmarkNewPooledConnection бенчмарк новой версии (пул подключений)
func BenchmarkNewPooledConnection(b *testing.B) {
	ctx := context.Background()
	grpcAddress := "localhost:50051"

	// Новый подход: одно подключение на все запросы
	conn, err := grpc.DialContext(ctx, grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Новый подход: используем существующее подключение
		// Эмуляция работы
		time.Sleep(1 * time.Millisecond)
	}
}

// BenchmarkParseDate бенчмарк парсинга дат
func BenchmarkParseDate(b *testing.B) {
	dateStr := "2024-01-15"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseDate(dateStr)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseDateInvalid бенчмарк парсинга некорректных дат
func BenchmarkParseDateInvalid(b *testing.B) {
	dateStr := "invalid-date"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parseDate(dateStr)
		if err == nil {
			b.Fatal("Expected error for invalid date")
		}
	}
}

// BenchmarkSendFile бенчмарк отправки файлов
func BenchmarkSendFile(b *testing.B) {
	gw := &Gateway{}
	data := make([]byte, 1024*1024) // 1MB
	filename := "test.csv"
	format := "csv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		gw.sendFile(w, data, filename, format)
	}
}

// BenchmarkSendFileSmall бенчмарк отправки маленьких файлов
func BenchmarkSendFileSmall(b *testing.B) {
	gw := &Gateway{}
	data := make([]byte, 1024) // 1KB
	filename := "test.csv"
	format := "csv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		gw.sendFile(w, data, filename, format)
	}
}

// BenchmarkWithCORS бенчмарк CORS middleware
func BenchmarkWithCORS(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := withCORS(handler)
	req := httptest.NewRequest("GET", "http://example.com", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		corsHandler.ServeHTTP(w, req)
	}
}

// BenchmarkWithCORSOptions бенчмарк CORS preflight запросов
func BenchmarkWithCORSOptions(b *testing.B) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := withCORS(handler)
	req := httptest.NewRequest("OPTIONS", "http://example.com", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		corsHandler.ServeHTTP(w, req)
	}
}
