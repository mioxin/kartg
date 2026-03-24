package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/database"
	"github.com/mioxin/kartg/internal/gateway"
	"github.com/mioxin/kartg/internal/i18n"
	"github.com/mioxin/kartg/internal/models"
	"github.com/mioxin/kartg/internal/service"
)

// Config содержит конфигурацию сервера
type Config struct {
	DBPath        string
	GRPCPort      string
	HTTPPort      string
	LogLevel      string
	JWTSecret     string
	AdminPassword string
}

// loadConfig загружает конфигурацию из флагов и переменных окружения
func loadConfig() *Config {
	dbPath := flag.String("db-path", getEnv("DB_PATH", "kartg.db"), "Путь к базе данных SQLite")
	grpcPort := flag.String("grpc-port", getEnv("GRPC_PORT", "50051"), "Порт gRPC сервера")
	httpPort := flag.String("http-port", getEnv("HTTP_PORT", "8080"), "Порт HTTP gateway")
	logLevel := flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Уровень логирования")
	jwtSecret := flag.String("jwt-secret", getEnv("JWT_SECRET", ""), "Секретный ключ JWT (обязательно)")
	adminPassword := flag.String("admin-password", getEnv("ADMIN_PASSWORD", ""), "Пароль администратора (если не указан, будет сгенерирован)")
	flag.Parse()

	return &Config{
		DBPath:        *dbPath,
		GRPCPort:      *grpcPort,
		HTTPPort:      *httpPort,
		LogLevel:      *logLevel,
		JWTSecret:     *jwtSecret,
		AdminPassword: *adminPassword,
	}
}

// setupLogging настраивает логирование
func setupLogging(level string) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))
}

// validateConfig проверяет конфигурацию
func validateConfig(cfg *Config) error {
	// JWT_SECRET обязателен для production
	if cfg.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required. Generate one with: openssl rand -base64 32")
	}

	// Минимальная длина JWT_SECRET
	if len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters long for security")
	}

	return nil
}

// generateSecurePassword генерирует безопасный случайный пароль
func generateSecurePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// createDefaultAdmin создает пользователя admin со случайным паролем
func createDefaultAdmin(db *gorm.DB, providedPassword string) (string, error) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)

	if result.Error == gorm.ErrRecordNotFound {
		// Генерируем случайный пароль или используем предоставленный
		password := providedPassword
		if password == "" {
			var err error
			password, err = generateSecurePassword(16)
			if err != nil {
				return "", fmt.Errorf("failed to generate password: %w", err)
			}
		}

		// Хешируем пароль
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return "", fmt.Errorf("failed to hash password: %w", err)
		}

		admin = models.User{
			Username: "admin",
			Password: string(hashedPassword),
			FullName: "Administrator",
			Role:     "admin",
		}

		if err := db.Create(&admin).Error; err != nil {
			return "", fmt.Errorf("failed to create admin user: %w", err)
		}

		return password, nil
	}

	return "", nil // Пользователь уже существует
}

func main() {
	// Загрузка конфигурации
	cfg := loadConfig()

	// Настройка логирования
	setupLogging(cfg.LogLevel)

	// Валидация конфигурации
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	slog.Info("Запуск сервера kartg",
		"db_path", cfg.DBPath,
		"grpc_port", cfg.GRPCPort,
		"http_port", cfg.HTTPPort,
		"log_level", cfg.LogLevel)

	// Инициализация локализатора
	if err := i18n.InitLocalizer(); err != nil {
		log.Fatalf("Ошибка инициализации локализатора: %v", err)
	}
	slog.Info("Локализатор инициализирован", "languages", []string{"ru", "en"})

	// Подключение к базе данных
	db, err := database.New(database.Config{
		DBPath:   cfg.DBPath,
		LogLevel: cfg.LogLevel,
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	// Выполняем миграцию моделей (убрано дублирование)
	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}, &models.User{}); err != nil {
		log.Fatalf("Ошибка миграции моделей: %v", err)
	}

	// Создаем пользователя admin по умолчанию
	adminPassword, err := createDefaultAdmin(db, cfg.AdminPassword)
	if err != nil {
		log.Fatalf("Ошибка создания пользователя admin: %v", err)
	}
	if adminPassword != "" {
		// Выводим пароль только один раз при создании
		slog.Info("🔐 СОЗДАН ПОЛЬЗОВАТЕЛЬ ADMIN",
			"username", "admin",
			"password", adminPassword,
			"warning", "СОХРАНИТЕ ЭТОТ ПАРОЛЬ В БЕЗОПАСНОМ МЕСТЕ!")
	}

	// Контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Создаем gRPC сервер
	grpcServer := grpc.NewServer()

	// Создаем сервис авторизации с безопасным JWT secret
	authService := service.NewAuthServiceServer(service.AuthConfig{
		DB:         db,
		JWTSecret:  cfg.JWTSecret,
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

	// Запускаем gRPC сервер
	grpcAddr := ":" + cfg.GRPCPort
	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("Не удалось создать listener: %v", err)
	}

	// Канал для ошибок gRPC сервера
	grpcErr := make(chan error, 1)
	go func() {
		slog.Info("gRPC сервер запущен", "address", grpcAddr)
		if err := grpcServer.Serve(grpcListener); err != nil {
			slog.Error("Ошибка gRPC сервера", "error", err)
			grpcErr <- err
		}
	}()

	// Создаем HTTP сервер с правильным shutdown
	httpAddr := ":" + cfg.HTTPPort
	httpServer := &http.Server{
		Addr:         httpAddr,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Канал для ошибок HTTP сервера
	httpErr := make(chan error, 1)
	go func() {
		slog.Info("HTTP Gateway запущен", "address", httpAddr)

		// Создаем gateway с правильным контекстом
		gw, err := gateway.NewGateway(ctx, gateway.Config{
			GRPCAddress: "localhost" + grpcAddr,
			HTTPAddress: httpAddr,
		})
		if err != nil {
			slog.Error("Ошибка создания gateway", "error", err)
			httpErr <- err
			return
		}
		defer gw.Close()

		httpServer.Handler = gw
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Ошибка HTTP Gateway", "error", err)
			httpErr <- err
		}
	}()

	// Обработка сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Ждем сигнала или ошибки
	select {
	case sig := <-quit:
		slog.Info("Получен сигнал завершения", "signal", sig.String())
	case err := <-grpcErr:
		if err != nil {
			slog.Error("gRPC сервер завершился с ошибкой", "error", err)
		}
	case err := <-httpErr:
		if err != nil {
			slog.Error("HTTP сервер завершился с ошибкой", "error", err)
		}
	}

	slog.Info("Остановка сервера...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Останавливаем gRPC сервер
	gracefulStop := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(gracefulStop)
	}()

	select {
	case <-gracefulStop:
		slog.Info("gRPC сервер остановлен")
	case <-shutdownCtx.Done():
		slog.Warn("Принудительная остановка gRPC сервера")
		grpcServer.Stop()
	}

	// Останавливаем HTTP сервер с shutdown
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("Ошибка при остановке HTTP сервера", "error", err)
	} else {
		slog.Info("HTTP сервер остановлен")
	}

	slog.Info("Сервер остановлен")
}

// getEnv возвращает значение переменной окружения или значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// serveSwagger обслуживает Swagger UI (заглушка для совместимости)
func serveSwagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"message": "Swagger UI доступен по адресу /swagger/index.html", "spec": "/api/openapi/v2/kartg.swagger.json"}`)
}
