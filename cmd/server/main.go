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
	"strings"
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
	Lang          string
}

// loadConfig загружает конфигурацию из флагов и переменных окружения
func loadConfig() *Config {
	// Получаем язык из переменной окружения или используем русский по умолчанию
	defaultLang := getEnv("LANG_CHOICE", getEnv("LANG", "ru"))
	if len(defaultLang) >= 2 {
		defaultLang = defaultLang[:2]
	}
	if defaultLang != "ru" && defaultLang != "en" {
		defaultLang = "ru"
	}

	// Создаем новый FlagSet для контроля над выводом help
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Проверяем, указан ли флаг -lang в аргументах командной строки
	cmdLang := defaultLang
	for i, arg := range os.Args[1:] {
		if arg == "-lang" && i+2 < len(os.Args) {
			cmdLang = os.Args[i+2]
		} else if strings.HasPrefix(arg, "-lang=") {
			cmdLang = strings.TrimPrefix(arg, "-lang=")
		}
	}
	if cmdLang != "ru" && cmdLang != "en" {
		cmdLang = defaultLang
	}

	// Определяем все флаги с локализованными описаниями на выбранном языке
	lang := fs.String("lang", defaultLang, i18n.TR(i18n.Language(cmdLang), "cli.flag.lang", nil))
	dbPath := fs.String("db-path", getEnv("DB_PATH", "data/kartg.db"), i18n.TR(i18n.Language(cmdLang), "cli.flag.db_path", nil))
	grpcPort := fs.String("grpc-port", getEnv("GRPC_PORT", "50051"), i18n.TR(i18n.Language(cmdLang), "cli.flag.grpc_port", nil))
	httpPort := fs.String("http-port", getEnv("HTTP_PORT", "8080"), i18n.TR(i18n.Language(cmdLang), "cli.flag.http_port", nil))
	logLevel := fs.String("log-level", getEnv("LOG_LEVEL", "info"), i18n.TR(i18n.Language(cmdLang), "cli.flag.log_level", nil))
	jwtSecret := fs.String("jwt-secret", getEnv("JWT_SECRET", ""), i18n.TR(i18n.Language(cmdLang), "cli.flag.jwt_secret", nil))
	adminPassword := fs.String("admin-password", getEnv("ADMIN_PASSWORD", ""), i18n.TR(i18n.Language(cmdLang), "cli.flag.admin_password", nil))

	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Если язык указан в аргументах, используем его
	actualLang := *lang
	if actualLang != "ru" && actualLang != "en" {
		actualLang = defaultLang
	}

	return &Config{
		DBPath:        *dbPath,
		GRPCPort:      *grpcPort,
		HTTPPort:      *httpPort,
		LogLevel:      *logLevel,
		JWTSecret:     *jwtSecret,
		AdminPassword: *adminPassword,
		Lang:          actualLang,
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

// createDefaultAdmin создает пользователя admin с паролем из CLI (если указан)
func createDefaultAdmin(db *gorm.DB, adminPassword string) error {
	var count int64
	db.Model(&models.User{}).Where("username = ? AND deleted_at IS NULL", "admin").Count(&count)

	if count == 0 {
		// Создаем пользователя admin с паролем из CLI или пустым
		password := ""
		if adminPassword != "" {
			// Хешируем пароль из CLI
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("failed to hash admin password: %w", err)
			}
			password = string(hashedPassword)
		}

		admin := models.User{
			Username: "admin",
			Password: password,
			FullName: "Administrator",
			Role:     "admin",
		}

		if err := db.Create(&admin).Error; err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		if password != "" {
			slog.Info("✅ Создан пользователь admin (с паролем из CLI)")
		} else {
			slog.Info("✅ Создан пользователь admin (без пароля)")
		}
		return nil
	}

	// Пользователь уже существует - обновляем пароль если указан новый
	if adminPassword != "" {
		var admin models.User
		result := db.Where("username = ?", "admin").First(&admin)
		if result.Error == nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("failed to hash new admin password: %w", err)
			}
			db.Model(&admin).Update("password", string(hashedPassword))
			slog.Info("🔄 Обновлен пароль пользователя admin")
		}
	}

	return nil // Пользователь уже существует
}

// createDefaultUser создает пользователя user с пустым паролем
func createDefaultUser(db *gorm.DB) error {
	var count int64
	db.Model(&models.User{}).Where("username = ? AND deleted_at IS NULL", "user").Count(&count)

	if count == 0 {
		// Создаем пользователя user с пустым паролем
		user := models.User{
			Username: "user",
			Password: "", // Пустой пароль
			FullName: "User",
			Role:     "user",
		}

		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to create user user: %w", err)
		}

		slog.Info("✅ Создан пользователь user (без пароля)")
		return nil
	}

	return nil // Пользователь уже существует
}

func main() {
	// Инициализация локализатора (должна быть до loadConfig)
	if err := i18n.InitLocalizer(); err != nil {
		log.Fatalf("Ошибка инициализации локализатора: %v", err)
	}

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

	// Подключение к базе данных
	db, err := database.New(database.Config{
		DBPath:   cfg.DBPath,
		LogLevel: cfg.LogLevel,
	})
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}

	// Выполняем миграцию моделей
	if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}, &models.User{}, &models.CartridgeModel{}); err != nil {
		log.Fatalf("Ошибка миграции моделей: %v", err)
	}

	// Создаем пользователя admin по умолчанию (с паролем из CLI если указан)
	if err := createDefaultAdmin(db, cfg.AdminPassword); err != nil {
		log.Fatalf("Ошибка создания пользователя admin: %v", err)
	}

	// Создаем пользователя user по умолчанию
	if err := createDefaultUser(db); err != nil {
		log.Fatalf("Ошибка создания пользователя user: %v", err)
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
	proto.RegisterModelServiceServer(grpcServer, service.NewModelServiceServer(db))

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
