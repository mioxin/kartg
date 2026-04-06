# Бэкенд
FROM golang:1.26-alpine AS backend

WORKDIR /app

# Установка зависимостей для сборки
RUN apk add --no-cache git make protoc

# Копирование go.mod и go.sum
COPY go.mod ./
RUN go mod tidy

# Копирование исходного кода
COPY . .

# Генерация кода из proto
ENV PATH=$PATH:/root/go/bin
RUN make gen || true

# Сборка бэкенда
RUN GOOS=linux go build -o server ./cmd/server

# Фронтенд
FROM node:22-alpine AS frontend

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
ENV JWT_SECRET=ваш_секретный_ключ
ENV ADMIN_PASSWORD=123
CMD ["sh", "-c", "export JWT_SECRET=${JWT_SECRET} && export ADMIN_PASSWORD=${ADMIN_PASSWORD} && nohup ./kartg-server > kartg.log 2>&1 &"]
