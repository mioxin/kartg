package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// BenchmarkOldCreateConnectionPerRequest бенчмарк старой версии (создание подключения на каждый запрос)
func BenchmarkOldCreateConnectionPerRequest(b *testing.B) {
	ctx := context.Background()
	grpcAddress := "localhost:50051"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Старый подход: создание подключения на каждый запрос
		// nolint:staticcheck // grpc.DialContext устарел, но будет поддерживаться в 1.x
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
	// nolint:staticcheck // grpc.DialContext устарел, но будет поддерживаться в 1.x
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
