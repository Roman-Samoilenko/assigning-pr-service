package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	pathHealth         = "/health"
	pathTeamAdd        = "/team/add"
	pathTeamGet        = "/team/get"
	pathTeamDeactivate = "/team/deactivate"
	pathUserActive     = "/users/setIsActive"
	pathUserReviews    = "/users/getReview"
	pathPRCreate       = "/pullRequest/create"
	pathPRMerge        = "/pullRequest/merge"
	pathPRReassign     = "/pullRequest/reassign"
	pathStats          = "/stats"
)

var (
	baseURL string
	client  *http.Client
)

func Init() {
	baseURL = os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	client = &http.Client{Timeout: 5 * time.Second}
}

func waitForService(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+pathHealth, nil)
			resp, err := client.Do(req)
			if err == nil {
				if err := resp.Body.Close(); err != nil {
					log.Printf("Ошибка при закрытии тела ответа: %v", err)
				}
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func TestMain(m *testing.M) {
	Init()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := waitForService(ctx); err != nil {
		cancel()
		log.Fatalf("сервис не готов: %v", err)
	}

	cancel()
	os.Exit(m.Run())
}

// Вспомогательные функции.
func doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(jsonBytes)
	} else if bodyStr, ok := body.(string); ok {
		bodyReader = bytes.NewBufferString(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func closeResp(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			log.Printf("Ошибка при чтении тела ответа: %v", err)
		}
		if err = resp.Body.Close(); err != nil {
			log.Printf("Ошибка при закрытии тела ответа: %v", err)
		}
	}
}

func post(ctx context.Context, path, body string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return client.Do(req)
}

func get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)

	if err != nil {
		return nil, err
	}

	return client.Do(req)
}

// Тесты

func TestHealthCheck(t *testing.T) {
	resp, err := doRequest(context.Background(), http.MethodGet, pathHealth, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp.StatusCode)
	}
}

func TestTeamAdd(t *testing.T) {
	ctx := context.Background()
	payload := map[string]interface{}{
		"team_name": "test_team",
		"members": []map[string]interface{}{
			{"user_id": "test_u1", "username": "Test1", "is_active": true},
		},
	}

	resp, err := doRequest(ctx, http.MethodPost, pathTeamAdd, payload)
	if err != nil {
		t.Fatal(err)
	}
	closeResp(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("ожидался 201, получили %d", resp.StatusCode)
	}
	resp2, err := doRequest(ctx, http.MethodPost, pathTeamAdd, payload)
	defer closeResp(resp2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("ожидался 400 при дубликате, получили %d", resp2.StatusCode)
	}
}

func TestTeamGet(t *testing.T) {
	teamName := fmt.Sprintf("team_get_%d", time.Now().UnixNano())
	_, err := doRequest(context.Background(), http.MethodPost, pathTeamAdd, map[string]interface{}{
		"team_name": teamName,
		"members":   []interface{}{},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := doRequest(context.Background(), http.MethodGet, pathTeamGet+"?team_name="+teamName, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp.StatusCode)
	}
}

func TestTeamGetNotFound(t *testing.T) {
	resp, err := get(context.Background(), "/team/get?team_name=nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался 404, получили %d", resp.StatusCode)
	}
}

func TestUsersSetIsActive(t *testing.T) {
	ctx := context.Background()

	resp, err := post(ctx, pathUserActive, `{"user_id":"user1","is_active":false}`)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp.StatusCode)
	}

	resp2, err := post(ctx, pathUserActive, `{"user_id":"user1","is_active":true}`)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200 при повторной активации, получили %d", resp2.StatusCode)
	}
}

func TestUsersSetIsActiveNotFound(t *testing.T) {
	resp, err := post(context.Background(), pathUserActive, `{"user_id":"nonexistent","is_active":false}`)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался 404, получили %d", resp.StatusCode)
	}
}

func TestPRCreate(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_test_%d", time.Now().UnixNano())

	body := fmt.Sprintf(
		`{"pull_request_id":"%s","pull_request_name":"Test PR","author_id":"user1"}`,
		prID,
	)

	resp, err := post(ctx, pathPRCreate, body)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("ожидался 201, получили %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	pr := result["pr"].(map[string]interface{})

	if pr["status"] != "OPEN" {
		t.Errorf("ожидался статус OPEN")
	}

	reviewers := pr["assigned_reviewers"].([]interface{})
	if len(reviewers) > 2 {
		t.Errorf("назначено слишком много ревьюеров")
	}

	for _, r := range reviewers {
		if r == "user1" {
			t.Errorf("автор не должен быть ревьюером")
		}
	}
}

func TestPRCreateDuplicate(t *testing.T) {
	ctx := context.Background()

	body := `{"pull_request_id":"pr_dup","pull_request_name":"Dup PR","author_id":"user1"}`

	resp1, _ := post(ctx, pathPRCreate, body)
	closeResp(resp1)

	resp, err := post(ctx, pathPRCreate, body)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("ожидался 409 при попытке создать дубликат, получили %d", resp.StatusCode)
	}
}

func TestPRCreateAuthorNotFound(t *testing.T) {
	resp, err := post(
		context.Background(),
		pathPRCreate,
		`{"pull_request_id":"pr_noauthor","pull_request_name":"No Author","author_id":"nonexistent"}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался 404, получили %d", resp.StatusCode)
	}
}

func TestPRMerge(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_merge_%d", time.Now().UnixNano())

	resp1, _ := post(ctx, pathPRCreate,
		fmt.Sprintf(
			`{"pull_request_id":"%s","pull_request_name":"Merge PR","author_id":"user1"}`,
			prID,
		),
	)
	closeResp(resp1)

	resp, err := post(ctx, pathPRMerge, fmt.Sprintf(`{"pull_request_id":"%s"}`, prID))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	pr := result["pr"].(map[string]interface{})
	if pr["status"] != "MERGED" {
		t.Errorf("ожидался статус MERGED")
	}

	resp2, err := post(ctx, pathPRMerge, fmt.Sprintf(`{"pull_request_id":"%s"}`, prID))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("повторный merge должен возвращать 200, получили %d", resp2.StatusCode)
	}
}

func TestPRMergeNotFound(t *testing.T) {
	resp, err := post(context.Background(), pathPRMerge, `{"pull_request_id":"nonexistent_pr"}`)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("ожидался 404, получили %d", resp.StatusCode)
	}
}

func TestPRReassign(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_reassign_%d", time.Now().UnixNano())

	resp, _ := post(ctx, pathPRCreate,
		fmt.Sprintf(
			`{"pull_request_id":"%s","pull_request_name":"Reassign PR","author_id":"user1"}`,
			prID,
		),
	)
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	closeResp(resp)

	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Skip("ревьюеры отсутствуют — пропускаем тест")
	}

	oldReviewer := reviewers[0].(string)

	resp2, err := post(ctx, pathPRReassign,
		fmt.Sprintf(`{"pull_request_id":"%s","old_user_id":"%s"}`, prID, oldReviewer),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp2.StatusCode)
	}
}

func TestPRReassignMerged(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_reassign_merged_%d", time.Now().UnixNano())

	resp, _ := post(ctx, pathPRCreate,
		fmt.Sprintf(
			`{"pull_request_id":"%s","pull_request_name":"Merged PR","author_id":"user1"}`,
			prID,
		),
	)
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	closeResp(resp)

	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Skip("ревьюеры отсутствуют — пропускаем тест")
	}

	resp1, _ := post(ctx, pathPRMerge, fmt.Sprintf(`{"pull_request_id":"%s"}`, prID))
	closeResp(resp1)

	resp2, err := post(ctx, pathPRReassign,
		fmt.Sprintf(`{"pull_request_id":"%s","old_user_id":"%s"}`, prID, reviewers[0].(string)),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp2)

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("ожидался 409 PR_MERGED, получили %d", resp2.StatusCode)
	}
}

func TestPRReassignNotAssigned(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_reassign_na_%d", time.Now().UnixNano())

	resp1, _ := post(ctx, pathPRCreate,
		fmt.Sprintf(
			`{"pull_request_id":"%s","pull_request_name":"NA PR","author_id":"user5"}`,
			prID,
		),
	)
	closeResp(resp1)

	resp, err := post(ctx, pathPRReassign,
		fmt.Sprintf(`{"pull_request_id":"%s","old_user_id":"user1"}`, prID),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("ожидался 409 NOT_ASSIGNED, получили %d", resp.StatusCode)
	}
}

func TestUsersGetReview(t *testing.T) {
	ctx := context.Background()
	prID := fmt.Sprintf("pr_getreview_%d", time.Now().UnixNano())

	resp, _ := post(ctx, pathPRCreate,
		fmt.Sprintf(
			`{"pull_request_id":"%s","pull_request_name":"GetReview PR","author_id":"user1"}`,
			prID,
		),
	)
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	closeResp(resp)

	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Skip("ревьюеры отсутствуют — пропускаем тест")
	}

	reviewer := reviewers[0].(string)

	resp2, err := get(ctx, pathUserReviews+"?user_id="+reviewer)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp2.StatusCode)
	}

	var result2 map[string]interface{}
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}

	prs := result2["pull_requests"].([]interface{})
	found := false

	for _, p := range prs {
		if p.(map[string]interface{})["pull_request_id"] == prID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("PR должен присутствовать в списке ревью пользователя")
	}
}

func TestStats(t *testing.T) {
	resp, err := get(context.Background(), pathStats)
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("ожидался 200, получили %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	if result["total_teams"] == nil {
		t.Errorf("нет поля total_teams в ответе")
	}
}

func TestTeamDeactivate(t *testing.T) {
	ctx := context.Background()

	teamName := fmt.Sprintf("deact_team_%d", time.Now().UnixNano())
	ts := time.Now().UnixNano()

	teamBody := fmt.Sprintf(
		`{"team_name":"%s","members":[
			{"user_id":"deact_u1_%d","username":"D1","is_active":true},
			{"user_id":"deact_u2_%d","username":"D2","is_active":true},
			{"user_id":"deact_u3_%d","username":"D3","is_active":true}
		]}`,
		teamName, ts, ts+1, ts+2,
	)

	resp1, _ := post(ctx, pathTeamAdd, teamBody)
	closeResp(resp1)

	resp, err := post(ctx, pathTeamDeactivate, fmt.Sprintf(`{"team_name":"%s"}`, teamName))
	if err != nil {
		t.Fatal(err)
	}
	defer closeResp(resp)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("ожидался 200, получили %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}

	deactivated := result["deactivated_users"].([]interface{})
	if len(deactivated) == 0 {
		t.Errorf("должны быть деактивированные пользователи")
	}
}
