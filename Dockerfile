# Бэкенд
FROM golang:1.22-alpine AS backend

WORKDIR /app

# Установка зависимостей для сборки
RUN apk add --no-cache git make protoc

# Копирование go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Генерация кода из proto
ENV PATH=$PATH:/root/go/bin
RUN make generate || true

# Сборка бэкенда
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server

# Фронтенд
FROM node:20-alpine AS frontend

WORKDIR /app/web

COPY web/package*.json ./
RUN npm install

COPY web/ ./
RUN npm run build

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Установка зависимостей для SQLite
RUN apk add --no-cache ca-certificates

# Копирование бэкенда
COPY --from=backend /app/server .

# Копирование фронтенда (статика)
COPY --from=frontend /app/web/dist ./web

# Создание директории для данных
RUN mkdir -p /app/data

# Порт для HTTP gateway
EXPOSE 8080

# Запуск сервера
CMD ["./server", "-http-port", ":8080", "-db-path", "/app/data/kartg.db"]
