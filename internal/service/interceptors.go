package service

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// publicMethods список методов, не требующих аутентификации
var publicMethods = map[string]bool{
	"/kartg.api.HealthService/Check":       true,
	"/kartg.api.AuthService/Login":         true,
	"/kartg.api.AuthService/RegisterUser":  true,
}

// LoggingInterceptor логирует каждый gRPC вызов
func LoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()

	// Вызываем handler
	resp, err := handler(ctx, req)

	// Логируем результат
	duration := time.Since(start)
	code := status.Code(err)

	logLevel := slog.LevelInfo
	if code == codes.Internal || code == codes.Unknown {
		logLevel = slog.LevelError
	}

	slog.Log(ctx, logLevel, "gRPC call",
		"method", info.FullMethod,
		"code", code.String(),
		"duration", duration.String(),
	)

	return resp, err
}

// AuthInterceptor проверяет JWT токен из metadata
func AuthInterceptor(jwtSecret []byte) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Проверяем, является ли метод публичным
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Получаем metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "отсутствует metadata")
		}

		// Ищем токен в Authorization header
		tokenValues := md.Get("authorization")
		if len(tokenValues) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "отсутствует токен авторизации")
		}

		tokenString := strings.TrimPrefix(tokenValues[0], "Bearer ")
		if tokenString == tokenValues[0] {
			return nil, status.Errorf(codes.Unauthenticated, "неверный формат токена, ожидается 'Bearer <token>'")
		}

		// Валидируем токен
		claims, err := validateToken(tokenString, jwtSecret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "недействительный токен: %v", err)
		}

		// Добавляем user_id в контекст
		userID, ok := claims["user_id"].(float64)
		if !ok {
			return nil, status.Errorf(codes.Internal, "неверный формат claims токена")
		}

		// Сохраняем user_id в контексте
		ctx = context.WithValue(ctx, userKey, uint(userID))

		return handler(ctx, req)
	}
}

// validateToken проверяет JWT токен и возвращает claims
func validateToken(tokenString string, jwtSecret []byte) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, status.Errorf(codes.Unauthenticated, "неверный метод подписи токена")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, status.Errorf(codes.Unauthenticated, "недействительный токен")
}

// RecoveryInterceptor восстанавливает сервер после panic
func RecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "Panic recovered in gRPC handler",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = status.Errorf(codes.Internal, "внутренняя ошибка сервера")
		}
	}()

	return handler(ctx, req)
}
