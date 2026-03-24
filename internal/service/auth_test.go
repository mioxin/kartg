package service

import (
	"context"
	"testing"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// TestAuthService_Login тестирует аутентификацию пользователя
func TestAuthService_Login(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	authSvc := NewAuthServiceServer(AuthConfig{
		DB:         testDB,
		JWTSecret:  "test-secret-key",
		TokenHours: 24,
	})
	ctx := context.Background()

	// Создаем тестового пользователя
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	testUser := models.User{
		Username: "testuser",
		Password: string(hashedPassword),
		FullName: "Test User",
		Role:     "user",
	}
	testDB.Create(&testUser)

	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{"Успешный вход", "testuser", "password123", false},
		{"Неверный пароль", "testuser", "wrongpassword", true},
		{"Пользователь не найден", "nonexistent", "password123", true},
		{"Пустой username", "", "password123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.LoginRequest{
				Username: tt.username,
				Password: tt.password,
			}

			resp, err := authSvc.Login(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Token == "" {
				t.Errorf("Токен не должен быть пустым")
			}

			if resp.User == nil {
				t.Errorf("Информация о пользователе не должна быть nil")
				return
			}

			if resp.User.Username != tt.username {
				t.Errorf("Ожидался username %s, получен %s", tt.username, resp.User.Username)
			}
		})
	}
}

// TestAuthService_RegisterUser тестирует регистрацию пользователя
func TestAuthService_RegisterUser(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	authSvc := NewAuthServiceServer(AuthConfig{
		DB:         testDB,
		JWTSecret:  "test-secret-key",
		TokenHours: 24,
	})
	ctx := context.Background()

	tests := []struct {
		name     string
		username string
		password string
		fullName string
		role     string
		wantErr  bool
	}{
		{"Успешная регистрация", "newuser", "password123", "New User", "user", false},
		{"Регистрация с пустой ролью", "admin2", "password123", "Admin Two", "", false},
		{"Дубликат username", "existinguser", "password123", "Existing User", "user", true},
	}

	// Создаем существующего пользователя
	existingUser := models.User{
		Username: "existinguser",
		Password: "hashedpassword",
		FullName: "Existing User",
		Role:     "user",
	}
	testDB.Create(&existingUser)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &proto.RegisterUserRequest{
				Username: tt.username,
				Password: tt.password,
				FullName: tt.fullName,
				Role:     tt.role,
			}

			resp, err := authSvc.RegisterUser(ctx, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Username != tt.username {
				t.Errorf("Ожидался username %s, получен %s", tt.username, resp.Username)
			}
		})
	}
}

// TestAuthService_ValidateToken тестирует валидацию JWT токена
func TestAuthService_ValidateToken(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	authSvc := NewAuthServiceServer(AuthConfig{
		DB:         testDB,
		JWTSecret:  "test-secret-key",
		TokenHours: 24,
	})

	// Создаем тестового пользователя
	user := models.User{
		Username: "tokenuser",
		Password: "hashedpassword",
		FullName: "Token User",
		Role:     "user",
	}
	testDB.Create(&user)

	// Генерируем токен
	token, err := authSvc.generateToken(user)
	if err != nil {
		t.Fatalf("Не удалось сгенерировать токен: %v", err)
	}

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"Валидный токен", token, false},
		{"Пустой токен", "", true},
		{"Невалидный токен", "invalid.token.here", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := authSvc.ValidateToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if claims == nil {
				t.Errorf("Claims не должны быть nil")
				return
			}

			if claims["username"] != "tokenuser" {
				t.Errorf("Ожидался username 'tokenuser', получен '%s'", claims["username"])
			}
		})
	}
}

// TestAuthService_GetCurrentUser тестирует получение текущего пользователя
func TestAuthService_GetCurrentUser(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB(t)

	authSvc := NewAuthServiceServer(AuthConfig{
		DB:         testDB,
		JWTSecret:  "test-secret-key",
		TokenHours: 24,
	})

	// Создаем тестового пользователя
	user := models.User{
		Username: "currentuser",
		Password: "hashedpassword",
		FullName: "Current User",
		Role:     "admin",
	}
	testDB.Create(&user)

	tests := []struct {
		name          string
		contextUserID interface{}
		wantErr       bool
	}{
		{"Успешное получение", uint(user.ID), false},
		{"Неаутентифицирован", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctxWithUser := context.Background()
			if tt.contextUserID != nil {
				ctxWithUser = context.WithValue(ctxWithUser, "user_id", tt.contextUserID)
			}

			req := &proto.GetCurrentUserRequest{}
			resp, err := authSvc.GetCurrentUser(ctxWithUser, req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Ожидалась ошибка, но получено nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Не ожидалась ошибка, но получено: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Ответ не должен быть nil")
				return
			}

			if resp.Username != user.Username {
				t.Errorf("Ожидался username %s, получен %s", user.Username, resp.Username)
			}
		})
	}
}
