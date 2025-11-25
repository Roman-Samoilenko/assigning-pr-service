//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var baseURL = "http://localhost:8081"

func main() {
	if url := os.Getenv("BASE_URL"); url != "" {
		baseURL = url
	}

	setupTestData()
	testCreatePR()
	testGetTeam()
	testStats()
	testTeamDeactivation()
}

func setupTestData() {
	for i := 1; i <= 3; i++ {
		team := map[string]interface{}{
			"team_name": fmt.Sprintf("loadtest_team%d", i),
			"members": []map[string]interface{}{
				{"user_id": fmt.Sprintf("lt_u%d_1", i), "username": fmt.Sprintf("User%d_1", i), "is_active": true},
				{"user_id": fmt.Sprintf("lt_u%d_2", i), "username": fmt.Sprintf("User%d_2", i), "is_active": true},
				{"user_id": fmt.Sprintf("lt_u%d_3", i), "username": fmt.Sprintf("User%d_3", i), "is_active": true},
				{"user_id": fmt.Sprintf("lt_u%d_4", i), "username": fmt.Sprintf("User%d_4", i), "is_active": true},
			},
		}
		body, _ := json.Marshal(team)
		http.Post(baseURL+"/team/add", "application/json", bytes.NewReader(body))
	}
}

func testCreatePR() {
	fmt.Println("Тест 1: Создание PR (5000 запросов, 500 параллельных)")
	runLoadTest("POST", "/pullRequest/create", func(id int) io.Reader {
		pr := map[string]string{
			"pull_request_id":   fmt.Sprintf("pr_load_%d_%d", time.Now().Unix(), id),
			"pull_request_name": "Load Test PR",
			"author_id":         "lt_u1_1",
		}
		body, _ := json.Marshal(pr)
		return bytes.NewReader(body)
	}, 5000, 500)
}

func testGetTeam() {
	fmt.Println("\nТест 2: Получение команды (5000 запросов, 500 параллельных)")
	runLoadTest("GET", "/team/get?team_name=loadtest_team1", nil, 5000, 500)
}

func testStats() {
	fmt.Println("\nТест 3: Получение статистики (5000 запросов, 500 параллельных)")
	runLoadTest("GET", "/stats", nil, 5000, 500)
}

func testTeamDeactivation() {
	fmt.Println("\nТест 4: Деактивация команды")

	team := map[string]interface{}{
		"team_name": "deact_load_test",
		"members": []map[string]interface{}{
			{"user_id": "deact_1", "username": "Deact1", "is_active": true},
			{"user_id": "deact_2", "username": "Deact2", "is_active": true},
			{"user_id": "deact_3", "username": "Deact3", "is_active": true},
		},
	}
	body, _ := json.Marshal(team)
	http.Post(baseURL+"/team/add", "application/json", bytes.NewReader(body))

	for i := range 10 {
		pr := map[string]string{
			"pull_request_id":   fmt.Sprintf("pr_deact_%d", i),
			"pull_request_name": "Deact Test PR",
			"author_id":         "deact_1",
		}
		prBody, _ := json.Marshal(pr)
		http.Post(baseURL+"/pullRequest/create", "application/json", bytes.NewReader(prBody))
	}

	start := time.Now()
	req := map[string]string{"team_name": "deact_load_test"}
	reqBody, _ := json.Marshal(req)
	resp, _ := http.Post(baseURL+"/team/deactivate", "application/json", bytes.NewReader(reqBody))
	duration := time.Since(start)

	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Printf("Деактивация завершена за %dмс\n", duration.Milliseconds())
			if duration.Milliseconds() < 100 {
				fmt.Println("  В пределах целевых 100мс")
			} else {
				fmt.Println("  Превышены целевые 100мс")
			}
		} else {
			fmt.Printf("Ошибка с кодом статуса %d\n", resp.StatusCode)
		}
	}
}

func runLoadTest(method, path string, bodyFn func(int) io.Reader, totalReqs, concurrent int) {
	var (
		success   int64
		failures  int64
		durations []time.Duration
		mu        sync.Mutex
		wg        sync.WaitGroup
		sem       = make(chan struct{}, concurrent)
	)

	start := time.Now()

	for i := range totalReqs {
		wg.Add(1)
		sem <- struct{}{}

		go func(id int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()
			var body io.Reader
			if bodyFn != nil {
				body = bodyFn(id)
			}

			req, err := http.NewRequest(method, baseURL+path, body)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := http.DefaultClient.Do(req)
			reqDuration := time.Since(reqStart)

			mu.Lock()
			durations = append(durations, reqDuration)
			mu.Unlock()

			if err != nil || resp == nil || resp.StatusCode >= 400 {
				atomic.AddInt64(&failures, 1)
			} else {
				atomic.AddInt64(&success, 1)
			}

			if resp != nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(start)

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	avg := sum / time.Duration(len(durations))

	fmt.Printf("Результаты:\n")
	fmt.Printf("  Общее время: %v\n", totalTime)
	fmt.Printf("  Запросы: %d\n", totalReqs)
	fmt.Printf("  Успешно: %d\n", success)
	fmt.Printf("  Ошибки: %d\n", failures)
	fmt.Printf("  Средняя задержка: %v\n", avg)
	fmt.Printf("  Пропускная способность: %.2f rps\n", float64(totalReqs)/totalTime.Seconds())
}
