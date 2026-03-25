package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mioxin/kartg/api/proto"
	"github.com/mioxin/kartg/internal/models"
	"gorm.io/gorm"
)

// AuthServiceServer реализует сервис авторизации
type AuthServiceServer struct {
	proto.UnimplementedAuthServiceServer
	db         *gorm.DB
	jwtSecret  []byte
	tokenHours int
}

// AuthConfig содержит конфигурацию для авторизации
type AuthConfig struct {
	DB         *gorm.DB
	JWTSecret  string
	TokenHours int
}

// NewAuthServiceServer создает новый сервис авторизации
func NewAuthServiceServer(cfg AuthConfig) *AuthServiceServer {
	// Если секрет не указан, используем значение по умолчанию
	jwtSecret := []byte(cfg.JWTSecret)
	if len(jwtSecret) == 0 {
		jwtSecret = []byte(os.Getenv("JWT_SECRET"))
		if len(jwtSecret) == 0 {
			jwtSecret = []byte("kartg-default-secret-key-change-in-production")
		}
	}

	tokenHours := cfg.TokenHours
	if tokenHours <= 0 {
		tokenHours = 24 // Токен действителен 24 часа
	}

	return &AuthServiceServer{
		db:         cfg.DB,
		jwtSecret:  jwtSecret,
		tokenHours: tokenHours,
	}
}

// LoginRequest запрос на вход
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse ответ на вход
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	User      *User  `json:"user"`
}

// User информация о пользователе
type User struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
}

// Login выполняет аутентификацию пользователя
func (s *AuthServiceServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	slog.Info("Попытка входа", "username", req.Username)

	// Ищем пользователя
	var user models.User
	result := s.db.Where("username = ?", req.Username).First(&user)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			slog.Warn("Пользователь не найден", "username", req.Username)
			return nil, status.Errorf(codes.Unauthenticated, "неверное имя пользователя или пароль")
		}
		slog.Error("Ошибка при поиске пользователя", "error", result.Error)
		return nil, status.Errorf(codes.Internal, "ошибка при входе: %v", result.Error)
	}

	// Проверяем пароль (если пароль не пустой)
	if req.Password != "" && user.Password != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			slog.Warn("Неверный пароль", "username", req.Username)
			return nil, status.Errorf(codes.Unauthenticated, "неверное имя пользователя или пароль")
		}
	}
	// Если пароль пустой или у пользователя не установлен пароль - пропускаем проверку

	// Генерируем JWT токен
	token, err := s.generateToken(user)
	if err != nil {
		slog.Error("Ошибка при генерации токена", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка при генерации токена")
	}

	slog.Info("Успешный вход", "username", req.Username, "user_id", user.ID)

	return &proto.LoginResponse{
		Token:     token,
		ExpiresIn: int32(s.tokenHours * 3600),
		User: &proto.UserInfo{
			Id:       int32(user.ID),
			Username: user.Username,
			FullName: user.FullName,
			Role:     user.Role,
		},
	}, nil
}

// generateToken генерирует JWT токен для пользователя
func (s *AuthServiceServer) generateToken(user models.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(s.tokenHours) * time.Hour)

	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      expiresAt.Unix(),
		"iat":      now.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken проверяет JWT токен и возвращает claims
func (s *AuthServiceServer) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("неверный метод подписи токена")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("неверный токен")
}

// RegisterUser регистрирует нового пользователя (только для администраторов)
func (s *AuthServiceServer) RegisterUser(ctx context.Context, req *proto.RegisterUserRequest) (*proto.UserInfo, error) {
	slog.Info("Регистрация пользователя", "username", req.Username)

	// Проверяем, существует ли пользователь
	var existingUser models.User
	result := s.db.Where("username = ?", req.Username).First(&existingUser)
	if result.Error == nil {
		return nil, status.Errorf(codes.AlreadyExists, "пользователь с таким именем уже существует")
	}

	// Хешируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Ошибка при хешировании пароля", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка при регистрации")
	}

	// Создаем пользователя
	user := models.User{
		Username: req.Username,
		Password: string(hashedPassword),
		FullName: req.FullName,
		Role:     "user", // По умолчанию роль user
	}

	if req.Role != "" {
		user.Role = req.Role
	}

	if err := s.db.Create(&user).Error; err != nil {
		slog.Error("Ошибка при создании пользователя", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка при регистрации: %v", err)
	}

	slog.Info("Пользователь зарегистрирован", "user_id", user.ID, "username", user.Username)

	return &proto.UserInfo{
		Id:       int32(user.ID),
		Username: user.Username,
		FullName: user.FullName,
		Role:     user.Role,
	}, nil
}

// GetCurrentUser возвращает информацию о текущем пользователе
func (s *AuthServiceServer) GetCurrentUser(ctx context.Context, req *proto.GetCurrentUserRequest) (*proto.UserInfo, error) {
	// Получаем user_id из контекста (должен быть добавлен middleware)
	userID, ok := ctx.Value("user_id").(uint)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "пользователь не аутентифицирован")
	}

	var user models.User
	result := s.db.First(&user, userID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "пользователь не найден")
		}
		return nil, status.Errorf(codes.Internal, "ошибка при получении пользователя: %v", result.Error)
	}

	return &proto.UserInfo{
		Id:       int32(user.ID),
		Username: user.Username,
		FullName: user.FullName,
		Role:     user.Role,
	}, nil
}

// ChangePassword меняет пароль текущего пользователя
func (s *AuthServiceServer) ChangePassword(ctx context.Context, req *proto.ChangePasswordRequest) (*proto.ChangePasswordResponse, error) {
	// Получаем user_id из контекста
	userID, ok := ctx.Value("user_id").(uint)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "пользователь не аутентифицирован")
	}

	slog.Info("Смена пароля", "user_id", userID)

	var user models.User
	result := s.db.First(&user, userID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, status.Errorf(codes.NotFound, "пользователь не найден")
		}
		return nil, status.Errorf(codes.Internal, "ошибка при получении пользователя: %v", result.Error)
	}

	// Проверяем старый пароль (если он установлен)
	if user.Password != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
			slog.Warn("Неверный старый пароль", "user_id", userID)
			return &proto.ChangePasswordResponse{
				Success: false,
				Message: "Неверный старый пароль",
			}, nil
		}
	}

	// Хешируем новый пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("Ошибка при хешировании нового пароля", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка при смене пароля")
	}

	// Обновляем пароль
	if err := s.db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		slog.Error("Ошибка при обновлении пароля", "error", err)
		return nil, status.Errorf(codes.Internal, "ошибка при смене пароля: %v", err)
	}

	slog.Info("Пароль успешно изменен", "user_id", userID, "username", user.Username)

	return &proto.ChangePasswordResponse{
		Success: true,
		Message: "Пароль успешно изменен",
	}, nil
}
