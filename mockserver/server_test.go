package mockserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResponsesJSON(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": "Hello there",
		"metadata": map[string]string{
			"test_case": "hello",
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"object\": \"response\"") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"input_tokens\": 12") {
		t.Fatalf("usage missing fixture tokens: %s", rec.Body.String())
	}
}

func TestResponsesFallbackWithoutFixture(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": "something unmatched",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"text\": \"Mock response.\"") {
		t.Fatalf("fallback response text missing: %s", rec.Body.String())
	}
}

func TestResponsesSizeMode(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"input": "hello\nllm:size",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "reply: input_tokens=") {
		t.Fatalf("size reply missing: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"total_tokens\":") {
		t.Fatalf("usage missing: %s", rec.Body.String())
	}
}

func TestChatStreaming(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"messages": []map[string]any{
			{"role": "user", "content": "Ping"},
		},
		"metadata": map[string]string{
			"test_case": "chat-hello",
		},
		"stream": true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "chat.completion.chunk") {
		t.Fatalf("stream body missing chunk: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "[DONE]") {
		t.Fatalf("stream body missing done marker: %s", rec.Body.String())
	}
}

func TestChatFallbackWithoutFixture(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"messages": []map[string]any{
			{"role": "user", "content": "unmatched"},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"content\": \"Mock chat response.\"") {
		t.Fatalf("fallback chat text missing: %s", rec.Body.String())
	}
}

func TestChatSizeMode(t *testing.T) {
	server := newTestServer(t)
	reqBody := map[string]any{
		"model": "gpt-5.4",
		"messages": []map[string]any{
			{"role": "user", "content": "hello\nllm:size"},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "reply: input_tokens=") {
		t.Fatalf("size reply missing: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"completion_tokens\":") {
		t.Fatalf("usage missing: %s", rec.Body.String())
	}
}

func newTestServer(t *testing.T) *Server {
	t.Helper()

	dir := t.TempDir()
	fixturesPath := filepath.Join(dir, "fixtures.json")
	if err := os.WriteFile(fixturesPath, []byte(testFixtures), 0o644); err != nil {
		t.Fatalf("write fixtures: %v", err)
	}

	server, err := New(Config{
		ListenAddr:              ":0",
		FixturesPath:            fixturesPath,
		DefaultModel:            "gpt-5.4",
		DefaultResponseText:     "Mock response.",
		DefaultChatResponseText: "Mock chat response.",
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server
}

const testFixtures = `{
  "fixtures": [
    {
      "id": "hello-response",
      "match": {
        "endpoint": "responses",
        "metadata": {
          "test_case": "hello"
        }
      },
      "replies": {
        "response_text": "Hello from the responses endpoint.",
        "response_usage": {
          "input_tokens": 12,
          "cached_tokens": 0,
          "output_tokens": 8,
          "reasoning_tokens": 0,
          "total_tokens": 20
        }
      }
    },
    {
      "id": "hello-chat",
      "match": {
        "endpoint": "chat_completions",
        "metadata": {
          "test_case": "chat-hello"
        }
      },
      "replies": {
        "chat_text": "Hello from the chat completions endpoint.",
        "chat_usage": {
          "prompt_tokens": 10,
          "cached_tokens": 0,
          "prompt_audio_tokens": 0,
          "completion_tokens": 9,
          "reasoning_tokens": 0,
          "completion_audio_tokens": 0,
          "accepted_prediction_tokens": 0,
          "rejected_prediction_tokens": 0,
          "total_tokens": 19
        }
      }
    }
  ]
}`

func TestNewTestServer(t *testing.T) {
	ts := NewTestServer(t, Fixture{
		ID:      "greet",
		Match:   FixtureMatch{Endpoint: "responses", InputContains: "hello"},
		Replies: FixtureReply{ResponseText: "Hi there!"},
	})

	body, _ := json.Marshal(map[string]any{"model": "gpt-4o", "input": "say hello"})
	resp, err := http.Post(ts.BaseURL()+"/v1/responses", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(got), "Hi there!") {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestMain(m *testing.M) {
	io.Discard.Write(nil)
	os.Exit(m.Run())
}
