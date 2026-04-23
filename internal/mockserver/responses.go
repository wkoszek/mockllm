package mockserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type responsesReply struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	CreatedAt         int64             `json:"created_at"`
	Status            string            `json:"status"`
	CompletedAt       int64             `json:"completed_at"`
	Error             any               `json:"error"`
	IncompleteDetails any               `json:"incomplete_details"`
	Instructions      *string           `json:"instructions"`
	MaxOutputTokens   *int              `json:"max_output_tokens"`
	Model             string            `json:"model"`
	Output            []responseItem    `json:"output"`
	ParallelToolCalls bool              `json:"parallel_tool_calls"`
	PreviousResponse  *string           `json:"previous_response_id"`
	Reasoning         map[string]any    `json:"reasoning"`
	Store             bool              `json:"store"`
	Temperature       float64           `json:"temperature"`
	Text              map[string]any    `json:"text"`
	ToolChoice        any               `json:"tool_choice"`
	Tools             []any             `json:"tools"`
	TopP              float64           `json:"top_p"`
	Truncation        string            `json:"truncation"`
	Usage             responsesUsage    `json:"usage"`
	User              any               `json:"user"`
	Metadata          map[string]string `json:"metadata"`
}

type responseItem struct {
	Type    string                `json:"type"`
	ID      string                `json:"id"`
	Status  string                `json:"status"`
	Role    string                `json:"role"`
	Content []responseContentPart `json:"content"`
}

type responseContentPart struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	Annotations []any  `json:"annotations"`
}

type responsesUsage struct {
	InputTokens         int            `json:"input_tokens"`
	InputTokensDetails  map[string]int `json:"input_tokens_details"`
	OutputTokens        int            `json:"output_tokens"`
	OutputTokensDetails map[string]int `json:"output_tokens_details"`
	TotalTokens         int            `json:"total_tokens"`
}

func (s *Server) buildResponsesReply(ctx context.Context, req responsesRequest, fixture Fixture) responsesReply {
	now := time.Now().Unix()
	model := req.Model
	if model == "" {
		model = s.cfg.DefaultModel
	}

	text := fixture.Replies.ResponseText
	if text == "" {
		text = s.cfg.DefaultResponseText
	}
	usage := s.buildResponsesUsage(ctx, req, fixture, text)
	if isLLMSizePrompt(req.inputText()) {
		text, usage = s.buildResponsesSizeReplyAndUsage(ctx, req, fixture)
	}
	store := req.Store == nil || *req.Store
	temperature := 1.0
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	topP := 1.0
	if req.TopP != nil {
		topP = *req.TopP
	}
	truncation := "disabled"
	if req.Truncation != nil && *req.Truncation != "" {
		truncation = *req.Truncation
	}
	toolChoice := req.ToolChoice
	if toolChoice == nil {
		toolChoice = "auto"
	}
	textFormat := req.Text
	if textFormat == nil {
		textFormat = map[string]any{"format": map[string]any{"type": "text"}}
	}

	return responsesReply{
		ID:                newID("resp"),
		Object:            "response",
		CreatedAt:         now,
		Status:            "completed",
		CompletedAt:       now,
		Error:             nil,
		IncompleteDetails: nil,
		Instructions:      req.Instructions,
		MaxOutputTokens:   req.MaxOutputTokens,
		Model:             model,
		Output: []responseItem{
			{
				Type:   "message",
				ID:     newID("msg"),
				Status: "completed",
				Role:   "assistant",
				Content: []responseContentPart{
					{
						Type:        "output_text",
						Text:        text,
						Annotations: fixture.Replies.ResponseAnnotations,
					},
				},
			},
		},
		ParallelToolCalls: true,
		PreviousResponse:  req.PreviousRespID,
		Reasoning:         map[string]any{"effort": nil, "summary": nil},
		Store:             store,
		Temperature:       temperature,
		Text:              asMap(textFormat),
		ToolChoice:        toolChoice,
		Tools:             req.Tools,
		TopP:              topP,
		Truncation:        truncation,
		Usage:             usage,
		User:              req.User,
		Metadata:          req.Metadata,
	}
}

func (s *Server) buildResponsesSizeReplyAndUsage(ctx context.Context, req responsesRequest, fixture Fixture) (string, responsesUsage) {
	inputTokens := approxTokenCount(req.inputText())
	cachedTokens := 0
	reasoningTokens := 0

	if u := fixture.Replies.ResponseUsage; u != nil {
		inputTokens = u.InputTokens
		cachedTokens = u.CachedTokens
		reasoningTokens = u.ReasoningTokens
	} else if s.cfg.EnableOpenAIInputTokens && s.cfg.OpenAIAPIKey != "" {
		if exact, err := s.countInputTokens(ctx, req.Model, req.Input); err == nil {
			inputTokens = exact
		}
	}

	text, outputTokens, totalTokens := renderSizeReply(inputTokens, cachedTokens, reasoningTokens)
	return text, responsesUsage{
		InputTokens:         inputTokens,
		InputTokensDetails:  map[string]int{"cached_tokens": cachedTokens},
		OutputTokens:        outputTokens,
		OutputTokensDetails: map[string]int{"reasoning_tokens": reasoningTokens},
		TotalTokens:         totalTokens,
	}
}

func (s *Server) buildResponsesUsage(ctx context.Context, req responsesRequest, fixture Fixture, outputText string) responsesUsage {
	if u := fixture.Replies.ResponseUsage; u != nil {
		total := u.TotalTokens
		if total == 0 {
			total = u.InputTokens + u.OutputTokens
		}
		return responsesUsage{
			InputTokens:         u.InputTokens,
			InputTokensDetails:  map[string]int{"cached_tokens": u.CachedTokens},
			OutputTokens:        u.OutputTokens,
			OutputTokensDetails: map[string]int{"reasoning_tokens": u.ReasoningTokens},
			TotalTokens:         total,
		}
	}

	inputTokens := approxTokenCount(req.inputText())
	if s.cfg.EnableOpenAIInputTokens && s.cfg.OpenAIAPIKey != "" {
		if exact, err := s.countInputTokens(ctx, req.Model, req.Input); err == nil {
			inputTokens = exact
		}
	}
	outputTokens := approxTokenCount(outputText)
	return responsesUsage{
		InputTokens:         inputTokens,
		InputTokensDetails:  map[string]int{"cached_tokens": 0},
		OutputTokens:        outputTokens,
		OutputTokensDetails: map[string]int{"reasoning_tokens": 0},
		TotalTokens:         inputTokens + outputTokens,
	}
}

func (s *Server) countInputTokens(ctx context.Context, model string, input any) (int, error) {
	payload := map[string]any{
		"model": model,
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	respBody, statusCode, err := s.forwardInputTokens(ctx, body)
	if err != nil {
		return 0, err
	}
	if statusCode >= 300 {
		return 0, fmt.Errorf("token counter returned status %d", statusCode)
	}
	var parsed struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return 0, err
	}
	return parsed.InputTokens, nil
}

func (s *Server) writeResponsesStream(w http.ResponseWriter, resp responsesReply) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is unsupported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	inProgress := resp
	inProgress.Status = "in_progress"
	inProgress.CompletedAt = 0
	inProgress.Output = []responseItem{}
	inProgress.Usage = responsesUsage{}

	writeSSE(w, "response.created", map[string]any{
		"type":     "response.created",
		"response": inProgress,
	})
	flusher.Flush()

	writeSSE(w, "response.in_progress", map[string]any{
		"type":     "response.in_progress",
		"response": inProgress,
	})
	flusher.Flush()

	item := resp.Output[0]
	part := item.Content[0]

	writeSSE(w, "response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"output_index": 0,
		"item": map[string]any{
			"id":      item.ID,
			"type":    item.Type,
			"status":  "in_progress",
			"role":    item.Role,
			"content": []any{},
		},
	})
	flusher.Flush()

	writeSSE(w, "response.content_part.added", map[string]any{
		"type":          "response.content_part.added",
		"item_id":       item.ID,
		"output_index":  0,
		"content_index": 0,
		"part": map[string]any{
			"type":        part.Type,
			"text":        "",
			"annotations": part.Annotations,
		},
	})
	flusher.Flush()

	for _, chunk := range chunkString(part.Text, 24) {
		writeSSE(w, "response.output_text.delta", map[string]any{
			"type":          "response.output_text.delta",
			"item_id":       item.ID,
			"output_index":  0,
			"content_index": 0,
			"delta":         chunk,
		})
		flusher.Flush()
	}

	writeSSE(w, "response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       item.ID,
		"output_index":  0,
		"content_index": 0,
		"text":          part.Text,
	})
	writeSSE(w, "response.content_part.done", map[string]any{
		"type":          "response.content_part.done",
		"item_id":       item.ID,
		"output_index":  0,
		"content_index": 0,
		"part":          part,
	})
	writeSSE(w, "response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"output_index": 0,
		"item":         item,
	})
	writeSSE(w, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": resp,
	})
	flusher.Flush()
}
