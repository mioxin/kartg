// Load test script for kartg API
// Запуск: go run scripts/loadtest.go
// Требования: сервер kartg должен быть запущен на localhost:8080

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	BaseURL         = "http://localhost:8080/api/v1"
	NumUsers        = 10
	RequestsPerUser = 5
)

type Cartridge struct {
	ID           string `json:"id"`
	Model        string `json:"model"`
	Status       string `json:"status"`
	TotalRefills int    `json:"total_refills"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func main() {
	fmt.Println("🚀 Нагрузочное тестирование kartg API")
	fmt.Printf("   Пользователей: %d\n", NumUsers)
	fmt.Printf("   Запросов на пользователя: %d\n", RequestsPerUser)
	fmt.Println()

	var (
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		totalDuration   time.Duration
		mu              sync.Mutex
	)

	// Барьер для одновременного старта
	var barrier sync.WaitGroup
	barrier.Add(NumUsers)

	// Запуск пользователей
	var wg sync.WaitGroup
	for i := 0; i < NumUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			// Ждем сигнала старта
			barrier.Done()

			client := &http.Client{Timeout: 10 * time.Second}

			for j := 0; j < RequestsPerUser; j++ {
				reqNum := j + 1
				cartridgeID := fmt.Sprintf("LOAD-%d-%d", userID, reqNum)

				start := time.Now()

				// 1. Регистрация картриджа
				success := registerCartridge(client, cartridgeID, userID, reqNum)

				duration := time.Since(start)

				// Обновляем статистику
				mu.Lock()
				totalRequests++
				totalDuration += duration
				if success {
					successRequests++
				} else {
					failedRequests++
				}
				mu.Unlock()

				// Небольшая задержка между запросами
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}

	// Старт!
	barrier.Wait()
	wg.Wait()

	// Вывод результатов
	fmt.Println()
	fmt.Println("📊 Результаты тестирования:")
	fmt.Printf("   Всего запросов: %d\n", totalRequests)
	fmt.Printf("   Успешно: %d (%.1f%%)\n", successRequests, float64(successRequests)/float64(totalRequests)*100)
	fmt.Printf("   Ошибок: %d (%.1f%%)\n", failedRequests, float64(failedRequests)/float64(totalRequests)*100)
	fmt.Printf("   Среднее время запроса: %v\n", totalDuration/time.Duration(totalRequests))
	fmt.Printf("   Запросов в секунду: %.1f\n", float64(totalRequests)/totalDuration.Seconds()*float64(time.Second))
	fmt.Println()

	if successRequests == totalRequests {
		fmt.Println("✅ Все запросы выполнены успешно!")
	} else {
		fmt.Println("⚠️  Часть запросов завершилась с ошибками")
	}
}

func registerCartridge(client *http.Client, cartridgeID string, userID, reqNum int) bool {
	url := BaseURL + "/cartridges"

	payload := map[string]string{
		"id":    cartridgeID,
		"model": fmt.Sprintf("HP-%d", userID),
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("   [User %d, Req %d] Ошибка создания запроса: %v\n", userID, reqNum, err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("   [User %d, Req %d] Ошибка выполнения: %v\n", userID, reqNum, err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("   [User %d, Req %d] HTTP %d: %s\n", userID, reqNum, resp.StatusCode, string(body))
		return false
	}

	// Парсим ответ
	var cartridge Cartridge
	if err := json.Unmarshal(body, &cartridge); err != nil {
		fmt.Printf("   [User %d, Req %d] Ошибка парсинга: %v\n", userID, reqNum, err)
		return false
	}

	fmt.Printf("   ✅ [User %d, Req %d] Картридж %s зарегистрирован\n", userID, reqNum, cartridgeID)
	return true
}
