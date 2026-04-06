.PHONY: gen clean build run start stop restart

# Пути
PROTOC := protoc
#$(HOME)/local/bin/protoc
PROTO_PATH := api/proto
INCLUDE_PATH := third_party
#:$(HOME)/local/include

# ==================== Генерация ====================

# Генерация кода из proto файлов
gen:
	@echo "Генерация Go кода gRPC..."
	$(PROTOC) -I $(PROTO_PATH) -I $(INCLUDE_PATH) \
		--go_out=$(PROTO_PATH) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_PATH) --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(PROTO_PATH) --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(PROTO_PATH) --openapiv2_opt=allow_merge=true,merge_file_name=kartg \
		$(PROTO_PATH)/service.proto
	@echo "Генерация завершена!"

# ==================== Сборка ====================

# Сборка бэкенда
build:
	@echo "Сборка бэкенда..."
	go build -o kartg-server ./cmd/server/main.go
	@echo "Сборка завершена!"

# ==================== Запуск ====================

# Запуск бэкенд сервера (в фоне)
start: build
	@echo "Запуск сервера kartg..."
	export JWT_SECRET=ваш_секретный_ключ && export ADMIN_PASSWORD="123" && nohup ./kartg-server > kartg.log 2>&1 &
	@sleep 2
	@echo "Сервер запущен на http://localhost:8080"
	@echo "gRPC на localhost:50051"
	@echo "Лог: kartg.log"

# Запуск бэкенд сервера (в интерактивном режиме)
run: build
	@echo "Запуск сервера kartg..."
	export JWT_SECRET=ваш_секретный_ключ && export ADMIN_PASSWORD="123" && ./kartg-server

# Остановка бэкенд сервера
stop:
	@echo "Остановка сервера kartg..."
	@if pgrep -x kartg-server > /dev/null 2>&1; then \
		kill $$(pgrep -x kartg-server) 2>/dev/null || true; \
		sleep 1; \
		echo "Сервер остановлен"; \
	else \
		echo "Сервер не запущен"; \
	fi

# Перезапуск сервера
restart: stop start
	@sleep 2
	@echo "Сервер перезапущен"

# ==================== Очистка ====================

# Очистка сгенерированных файлов
clean:
	rm -f api/proto/*.pb.go api/proto/*.pb.gw.go
	rm -f api/proto/*.swagger.json
	rm -f kartg-server
	rm -rf data/*.db data/*.db-wal data/*.db-shm

# Очистка и сборка
rebuild: clean build

# ==================== Инструменты ====================

# Установка инструментов генерации
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

# Установка golangci-lint
install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# ==================== Линтер ====================

# Запуск golangci-lint
lint:
	golangci-lint run ./...

# Запуск golangci-lint с автоисправлением
lint-fix:
	golangci-lint run ./... --fix

# ==================== Тесты ====================

# Запуск тестов
test:
	go test ./...

# Запуск тестов с покрытием
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Отчет о покрытии: coverage.html"

# ==================== Фронтенд ====================

# Сборка фронтенда
web-build:
	@echo "Сборка фронтенда..."
	@export NVM_DIR="$(HOME)/.nvm" && \
		[ -s "$$NVM_DIR/nvm.sh" ] && \. "$$NVM_DIR/nvm.sh" && \
		nvm use 22 && \
		cd web && npm run build
	@echo "Фронтенд собран!"

# Запуск фронтенда (режим разработки)
web-dev:
	@echo "Запуск фронтенда (dev mode)..."
	@export NVM_DIR="$(HOME)/.nvm" && \
		[ -s "$$NVM_DIR/nvm.sh" ] && \. "$$NVM_DIR/nvm.sh" && \
		nvm use 22 && \
		cd web && npm run dev

# Установка зависимостей фронтенда
web-install:
	@echo "Установка зависимостей фронтенда..."
	@export NVM_DIR="$(HOME)/.nvm" && \
		[ -s "$$NVM_DIR/nvm.sh" ] && \. "$$NVM_DIR/nvm.sh" && \
		nvm use 22 && \
		cd web && npm install

# ==================== Docker ====================

# Запуск в Docker
docker-up:
	docker-compose up -d --build

# Остановка Docker
docker-down:
	docker-compose down

# Просмотр логов Docker
docker-logs:
	docker-compose logs -f

# Пересборка Docker
docker-rebuild:
	docker-compose down
	docker-compose build --no-cache
	docker-compose up -d
