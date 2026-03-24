package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

	"github.com/mioxin/kartg/internal/database"
	"github.com/mioxin/kartg/internal/gateway"
	"github.com/mioxin/kartg/internal/models"
	"github.com/mioxin/kartg/internal/service"

	"github.com/mioxin/kartg/api/proto"
)

func main() {
	// Парсинг флагов командной строки
	dbPath := flag.String("db-path", getEnv("DB_PATH", "kartg.db"), "Путь к базе данных SQLite")
	grpcPort := flag.String("grpc-port", getEnv("GRPC_PORT", "50051"), "Порт gRPC сервера")
	httpPort := flag.String("http-port", getEnv("HTTP_PORT", "8080"), "Порт HTTP gateway")
	logLevel := flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Уровень логирования")
	flag.Parse()

	// Настройка логирования
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	slog.Info("Запуск сервера kartg",
		"db_path", *dbPath,
		"grpc_port", *grpcPort,
		"http_port", *httpPort,
		"log_level", *logLevel)

	// Подключение к базе данных
	db, err := database.New(database.Config{
		DBPath:   *dbPath,
		LogLevel: *logLevel,
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	// Автоматическая миграция
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("Ошибка миграции: %v", err)
	}

	// Выполняем миграцию моделей
	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}, &models.User{}); err != nil {
		log.Fatalf("Ошибка миграции моделей: %v", err)
	}

	// Создаем пользователя admin по умолчанию, если не существует
	createDefaultAdmin(db)

	// Контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()

	// Создаем сервис авторизации
	authService := service.NewAuthServiceServer(service.AuthConfig{
		DB:         db,
		JWTSecret:  getEnv("JWT_SECRET", "kartg-secret-key-change-in-production"),
		TokenHours: 24,
	})

	// Регистрируем сервисы
	proto.RegisterCartridgeServiceServer(grpcServer, service.NewCartridgeServiceServer(db))
	proto.RegisterOperationServiceServer(grpcServer, service.NewOperationServiceServer(db))
	proto.RegisterAnalyticsServiceServer(grpcServer, service.NewAnalyticsServiceServer(db))
	proto.RegisterHealthServiceServer(grpcServer, service.NewHealthServiceServer())
	proto.RegisterAuthServiceServer(grpcServer, authService)

	// Включаем reflection для отладки
	reflection.Register(grpcServer)

	// Запускаем gRPC сервер в отдельной горутине
	grpcAddr := ":" + *grpcPort
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Не удалось создать listener: %v", err)
	}

	go func() {
		slog.Info("gRPC сервер запущен", "address", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("Ошибка gRPC сервера", "error", err)
		}
	}()

	// Запускаем HTTP Gateway в отдельной горутине
	httpAddr := ":" + *httpPort
	go func() {
		slog.Info("Запуск HTTP Gateway", "address", httpAddr)
		if err := gateway.Run(ctx, gateway.Config{
			GRPCAddress: "localhost" + grpcAddr,
			HTTPAddress: httpAddr,
		}); err != nil {
			slog.Error("Ошибка HTTP Gateway", "error", err)
		}
	}()

	// Обработка сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Остановка сервера...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	grpcServer.GracefulStop()

	// Остановка HTTP сервера (нужно сохранить ссылку на сервер)
	// Для простоты просто отменяем контекст
	cancel()

	<-shutdownCtx.Done()
	slog.Info("Сервер остановлен")
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// createDefaultAdmin создает пользователя admin по умолчанию
func createDefaultAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)

	if result.Error == gorm.ErrRecordNotFound {
		// Создаем администратора по умолчанию
		// Пароль: admin123 (хешируется)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Ошибка при хешировании пароля администратора: %v", err)
			return
		}

		admin = models.User{
			Username: "admin",
			Password: string(hashedPassword),
			FullName: "Administrator",
			Role:     "admin",
		}

		if err := db.Create(&admin).Error; err != nil {
			log.Printf("Ошибка при создании пользователя admin: %v", err)
			return
		}

		slog.Info("Создан пользователь по умолчанию", "username", "admin", "password", "admin123")
	}
}

// serveSwagger обслуживает Swagger UI
func serveSwagger(w http.ResponseWriter, r *http.Request) {
	// В простой версии просто возвращаем сообщение
	// В полной версии здесь будет Swagger UI
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "Swagger UI доступен по адресу /swagger/index.html", "spec": "/api/openapi/v2/kartg.swagger.json"}`)
}
