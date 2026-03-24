package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CartridgeStatus представляет статус картриджа
type CartridgeStatus string

const (
	CartridgeStatusInUse     CartridgeStatus = "InUse"
	CartridgeStatusRefilling CartridgeStatus = "Refilling"
	CartridgeStatusRetired   CartridgeStatus = "Retired"
)

// Cartridge представляет картридж для лазерного принтера
type Cartridge struct {
	ID           string          `gorm:"primaryKey;size:100" json:"id"`
	Model        string          `gorm:"size:200;not null" json:"model"`
	Status       CartridgeStatus `gorm:"type:varchar(20);not null;default:'InUse'" json:"status"`
	TotalRefills int             `gorm:"default:0" json:"total_refills"`
	CreatedAt    time.Time       `json:"created_at"`
	RetiredAt    *time.Time      `json:"retired_at,omitempty"`
}

// TableName указывает имя таблицы для модели
func (Cartridge) TableName() string {
	return "cartridges"
}

// OperationType представляет тип операции
type OperationType string

const (
	OperationTypeRegistration      OperationType = "Registration"
	OperationTypeSendToRefill      OperationType = "SendToRefill"
	OperationTypeReceiveFromRefill OperationType = "ReceiveFromRefill"
	OperationTypeRetirement        OperationType = "Retirement"
)

// Transaction представляет операцию с картриджем
type Transaction struct {
	ID          string         `gorm:"primaryKey;size:36" json:"id"` // UUID
	CartridgeID string         `gorm:"size:100;not null;index" json:"cartridge_id"`
	Type        OperationType  `gorm:"type:varchar(50);not null" json:"type"`
	Timestamp   time.Time      `gorm:"not null" json:"timestamp"`
	Comment     string         `gorm:"size:500" json:"comment"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName указывает имя таблицы для модели
func (Transaction) TableName() string {
	return "transactions"
}

// IsValidTransition проверяет, является ли переход статуса допустимым
// и возвращает ошибку с описанием, если переход недопустим
func IsValidTransition(from CartridgeStatus, to CartridgeStatus) error {
	transitions := map[CartridgeStatus][]CartridgeStatus{
		CartridgeStatusInUse:     {CartridgeStatusRefilling, CartridgeStatusRetired},
		CartridgeStatusRefilling: {CartridgeStatusInUse, CartridgeStatusRetired},
		CartridgeStatusRetired:   {}, // Утилизированный картридж не может менять статус
	}

	allowed, exists := transitions[from]
	if !exists {
		return fmt.Errorf("неизвестный исходный статус: %s", from)
	}

	for _, status := range allowed {
		if status == to {
			return nil
		}
	}

	return fmt.Errorf("недопустимый переход статуса: %s → %s", from, to)
}

// CanSendToRefill проверяет, можно ли отправить картридж на заправку
func (c *Cartridge) CanSendToRefill() error {
	if c.Status == CartridgeStatusRetired {
		return fmt.Errorf("нельзя отправить утилизированный картридж на заправку")
	}
	if c.Status == CartridgeStatusRefilling {
		return fmt.Errorf("картридж уже находится на заправке")
	}
	return nil
}

// CanReceiveFromRefill проверяет, можно ли принять картридж с заправки
func (c *Cartridge) CanReceiveFromRefill() error {
	if c.Status != CartridgeStatusRefilling {
		return fmt.Errorf("картридж не находится на заправке (текущий статус: %s)", c.Status)
	}
	return nil
}

// CanRetire проверяет, можно ли утилизировать картридж
func (c *Cartridge) CanRetire() error {
	if c.Status == CartridgeStatusRetired {
		return fmt.Errorf("картридж уже утилизирован")
	}
	return nil
}
