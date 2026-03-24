package database

import (
	"fmt"
	"log/slog"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config содержит конфигурацию для подключения к БД
type Config struct {
	DBPath  string
	LogLevel string
}

// New создает новое подключение к SQLite с WAL режимом
func New(cfg Config) (*gorm.DB, error) {
	// Настройка уровня логирования GORM
	var logLevel logger.LogLevel
	switch cfg.LogLevel {
	case "debug":
		logLevel = logger.Info
	case "info":
		logLevel = logger.Warn
	default:
		logLevel = logger.Error
	}

	// DSN с WAL режимом для конкурентной записи
	// _journal_mode=WAL - включает Write-Ahead Logging
	// _busy_timeout=5000 - таймаут ожидания разблокировки БД (5 секунд)
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", cfg.DBPath)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Получаем SQL подключение для настройки
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Настройка пула подключений
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	slog.Info("Подключение к базе данных установлено", "path", cfg.DBPath, "wal_mode", true)

	return db, nil
}

// AutoMigrate создает/обновляет таблицы в базе данных
func AutoMigrate(db *gorm.DB) error {
	slog.Info("Запуск миграции базы данных")

	// Миграция будет добавлена после создания моделей
	// Пример:
	// if err := db.AutoMigrate(&models.Cartridge{}, &models.Transaction{}); err != nil {
	// 	return fmt.Errorf("migration failed: %w", err)
	// }

	slog.Info("Миграция базы данных завершена")
	return nil
}
