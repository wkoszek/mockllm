package mockserver

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	cfg        Config
	fixtures   []Fixture
	httpClient *http.Client
}

func New(cfg Config) (*Server, error) {
	var fixtures []Fixture
	if cfg.Fixtures != nil {
		fixtures = cfg.Fixtures
	} else {
		var err error
		fixtures, err = LoadFixtures(cfg.FixturesPath)
		if err != nil {
			return nil, err
		}
	}

	return &Server{
		cfg:      cfg,
		fixtures: fixtures,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/responses", s.handleResponses)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	mux.HandleFunc("/v1/responses/input_tokens", s.handleInputTokens)
	mux.HandleFunc("/healthz", s.handleHealthz)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req responsesRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("read body: %v", err))
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("parse body: %v", err))
		return
	}

	fixture, ok := s.findFixture(MatchRequest{
		Endpoint:         "responses",
		Metadata:         req.Metadata,
		InputText:        req.inputText(),
		HeaderResponseID: r.Header.Get("X-Mock-Response-ID"),
	})
	matchKind := "fixture"
	if !ok {
		fixture = Fixture{}
		matchKind = "default"
	}

	resp := s.buildResponsesReply(r.Context(), req, fixture)
	if isLLMSizePrompt(req.inputText()) {
		matchKind = "size"
	}
	s.debugLogResponses(req, fixture, matchKind, resp)
	if req.Stream {
		s.writeResponsesStream(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("parse body: %v", err))
		return
	}

	fixture, ok := s.findFixture(MatchRequest{
		Endpoint:         "chat_completions",
		Metadata:         req.Metadata,
		ChatText:         req.chatText(),
		HeaderResponseID: r.Header.Get("X-Mock-Response-ID"),
	})
	matchKind := "fixture"
	if !ok {
		fixture = Fixture{}
		matchKind = "default"
	}

	resp := s.buildChatReply(r.Context(), req, fixture)
	if isLLMSizePrompt(req.chatText()) {
		matchKind = "size"
	}
	s.debugLogChat(req, fixture, matchKind, resp)
	if req.Stream {
		s.writeChatStream(w, resp)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleInputTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("read body: %v", err))
		return
	}

	if s.cfg.EnableOpenAIInputTokens && s.cfg.OpenAIAPIKey != "" {
		respBody, statusCode, err := s.forwardInputTokens(r.Context(), body)
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_, _ = w.Write(respBody)
			return
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("parse body: %v", err))
		return
	}

	inputText := flattenAnyToText(payload["input"])
	count := approxTokenCount(inputText)
	writeJSON(w, http.StatusOK, map[string]any{
		"object":       "response.input_tokens",
		"input_tokens": count,
	})
}

func (s *Server) forwardInputTokens(ctx context.Context, body []byte) ([]byte, int, error) {
	url := strings.TrimRight(s.cfg.OpenAIBaseURL, "/") + "/v1/responses/input_tokens"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return respBody, resp.StatusCode, nil
}

func (s *Server) findFixture(req MatchRequest) (Fixture, bool) {
	for _, fixture := range s.fixtures {
		if fixture.Matches(req) {
			return fixture, true
		}
	}
	return Fixture{}, false
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSONError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
}

func writeJSONError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}

func newID(prefix string) string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return prefix + "_" + hex.EncodeToString(buf[:])
}

func (s *Server) debugLogResponses(req responsesRequest, fixture Fixture, matchKind string, resp responsesReply) {
	if !s.cfg.Debug {
		return
	}

	replyText := ""
	if len(resp.Output) > 0 && len(resp.Output[0].Content) > 0 {
		replyText = resp.Output[0].Content[0].Text
	}

	log.Printf(
		`responses match=%s fixture=%q stream=%t model=%q input=%q reply=%q usage={input=%d cached=%d output=%d reasoning=%d total=%d}`,
		matchKind,
		fixture.ID,
		req.Stream,
		resp.Model,
		shorten(req.inputText()),
		shorten(replyText),
		resp.Usage.InputTokens,
		resp.Usage.InputTokensDetails["cached_tokens"],
		resp.Usage.OutputTokens,
		resp.Usage.OutputTokensDetails["reasoning_tokens"],
		resp.Usage.TotalTokens,
	)
}

func (s *Server) debugLogChat(req chatRequest, fixture Fixture, matchKind string, resp chatReply) {
	if !s.cfg.Debug {
		return
	}

	replyText := ""
	if len(resp.Choices) > 0 {
		if content, ok := resp.Choices[0].Message.Content.(string); ok {
			replyText = content
		}
	}

	log.Printf(
		`chat match=%s fixture=%q stream=%t model=%q input=%q reply=%q usage={input=%d cached=%d output=%d reasoning=%d total=%d}`,
		matchKind,
		fixture.ID,
		req.Stream,
		resp.Model,
		shorten(req.chatText()),
		shorten(replyText),
		resp.Usage.PromptTokens,
		resp.Usage.PromptTokensDetails["cached_tokens"],
		resp.Usage.CompletionTokens,
		resp.Usage.CompletionTokensDetails["reasoning_tokens"],
		resp.Usage.TotalTokens,
	)
}

func shorten(text string) string {
	text = strings.ReplaceAll(text, "\n", "\\n")
	const limit = 140
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
