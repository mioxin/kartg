package models

import (
	"time"

	"gorm.io/gorm"
)

// User представляет пользователя системы
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"` // Хэш пароля
	FullName  string         `gorm:"size:100" json:"full_name"`
	Role      string         `gorm:"size:20;default:'user'" json:"role"` // admin, user
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName указывает имя таблицы для модели
func (User) TableName() string {
	return "users"
}
