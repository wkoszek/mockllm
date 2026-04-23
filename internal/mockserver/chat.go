package mockserver

import (
	"context"
	"net/http"
	"time"
)

type chatReply struct {
	ID                string         `json:"id"`
	Object            string         `json:"object"`
	Created           int64          `json:"created"`
	Model             string         `json:"model"`
	Choices           []chatChoice   `json:"choices"`
	Usage             chatUsageReply `json:"usage"`
	ServiceTier       string         `json:"service_tier"`
	SystemFingerprint any            `json:"system_fingerprint,omitempty"`
}

type chatChoice struct {
	Index        int              `json:"index"`
	Message      chatMessageReply `json:"message"`
	Logprobs     any              `json:"logprobs"`
	FinishReason string           `json:"finish_reason"`
}

type chatMessageReply struct {
	Role        string `json:"role"`
	Content     any    `json:"content"`
	Refusal     any    `json:"refusal"`
	Annotations []any  `json:"annotations,omitempty"`
}

type chatUsageReply struct {
	PromptTokens            int            `json:"prompt_tokens"`
	CompletionTokens        int            `json:"completion_tokens"`
	TotalTokens             int            `json:"total_tokens"`
	PromptTokensDetails     map[string]int `json:"prompt_tokens_details,omitempty"`
	CompletionTokensDetails map[string]int `json:"completion_tokens_details,omitempty"`
}

func (s *Server) buildChatReply(ctx context.Context, req chatRequest, fixture Fixture) chatReply {
	now := time.Now().Unix()
	model := req.Model
	if model == "" {
		model = s.cfg.DefaultModel
	}
	text := fixture.Replies.ChatText
	if text == "" {
		text = s.cfg.DefaultChatResponseText
	}
	usage := s.buildChatUsage(ctx, req, fixture, text)
	if isLLMSizePrompt(req.chatText()) {
		text, usage = s.buildChatSizeReplyAndUsage(ctx, req, fixture)
	}

	return chatReply{
		ID:      newID("chatcmpl"),
		Object:  "chat.completion",
		Created: now,
		Model:   model,
		Choices: []chatChoice{
			{
				Index: 0,
				Message: chatMessageReply{
					Role:        "assistant",
					Content:     text,
					Refusal:     nil,
					Annotations: fixture.Replies.ChatAnnotations,
				},
				Logprobs:     nil,
				FinishReason: "stop",
			},
		},
		Usage:       usage,
		ServiceTier: "default",
	}
}

func (s *Server) buildChatSizeReplyAndUsage(ctx context.Context, req chatRequest, fixture Fixture) (string, chatUsageReply) {
	promptTokens := approxTokenCount(req.chatText())
	cachedTokens := 0
	reasoningTokens := 0

	if u := fixture.Replies.ChatUsage; u != nil {
		promptTokens = u.PromptTokens
		cachedTokens = u.CachedTokens
		reasoningTokens = u.ReasoningTokens
	} else if s.cfg.EnableOpenAIInputTokens && s.cfg.OpenAIAPIKey != "" {
		if exact, err := s.countInputTokens(ctx, req.Model, req.Messages); err == nil {
			promptTokens = exact
		}
	}

	text, completionTokens, totalTokens := renderSizeReply(promptTokens, cachedTokens, reasoningTokens)
	return text, chatUsageReply{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		PromptTokensDetails: map[string]int{
			"cached_tokens": cachedTokens,
			"audio_tokens":  0,
		},
		CompletionTokensDetails: map[string]int{
			"reasoning_tokens":           reasoningTokens,
			"audio_tokens":               0,
			"accepted_prediction_tokens": 0,
			"rejected_prediction_tokens": 0,
		},
	}
}

func (s *Server) buildChatUsage(ctx context.Context, req chatRequest, fixture Fixture, outputText string) chatUsageReply {
	if u := fixture.Replies.ChatUsage; u != nil {
		total := u.TotalTokens
		if total == 0 {
			total = u.PromptTokens + u.CompletionTokens
		}
		return chatUsageReply{
			PromptTokens:     u.PromptTokens,
			CompletionTokens: u.CompletionTokens,
			TotalTokens:      total,
			PromptTokensDetails: map[string]int{
				"cached_tokens": u.CachedTokens,
				"audio_tokens":  u.PromptAudioTokens,
			},
			CompletionTokensDetails: map[string]int{
				"reasoning_tokens":           u.ReasoningTokens,
				"audio_tokens":               u.CompletionAudioTokens,
				"accepted_prediction_tokens": u.AcceptedPredictionTokens,
				"rejected_prediction_tokens": u.RejectedPredictionTokens,
			},
		}
	}

	promptTokens := approxTokenCount(req.chatText())
	if s.cfg.EnableOpenAIInputTokens && s.cfg.OpenAIAPIKey != "" {
		if exact, err := s.countInputTokens(ctx, req.Model, req.Messages); err == nil {
			promptTokens = exact
		}
	}
	completionTokens := approxTokenCount(outputText)
	return chatUsageReply{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
		PromptTokensDetails: map[string]int{
			"cached_tokens": 0,
			"audio_tokens":  0,
		},
		CompletionTokensDetails: map[string]int{
			"reasoning_tokens":           0,
			"audio_tokens":               0,
			"accepted_prediction_tokens": 0,
			"rejected_prediction_tokens": 0,
		},
	}
}

func (s *Server) writeChatStream(w http.ResponseWriter, resp chatReply) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is unsupported by this server")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	initial := map[string]any{
		"id":                 resp.ID,
		"object":             "chat.completion.chunk",
		"created":            resp.Created,
		"model":              resp.Model,
		"system_fingerprint": "fp_mockserver",
		"choices": []any{
			map[string]any{
				"index": 0,
				"delta": map[string]any{
					"role":    "assistant",
					"content": "",
				},
				"logprobs":      nil,
				"finish_reason": nil,
			},
		},
	}
	writeChatChunk(w, initial)
	flusher.Flush()

	text := ""
	if len(resp.Choices) > 0 {
		if content, ok := resp.Choices[0].Message.Content.(string); ok {
			text = content
		}
	}
	for _, chunk := range chunkString(text, 20) {
		writeChatChunk(w, map[string]any{
			"id":                 resp.ID,
			"object":             "chat.completion.chunk",
			"created":            resp.Created,
			"model":              resp.Model,
			"system_fingerprint": "fp_mockserver",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"content": chunk,
					},
					"logprobs":      nil,
					"finish_reason": nil,
				},
			},
		})
		flusher.Flush()
	}

	writeChatChunk(w, map[string]any{
		"id":                 resp.ID,
		"object":             "chat.completion.chunk",
		"created":            resp.Created,
		"model":              resp.Model,
		"system_fingerprint": "fp_mockserver",
		"choices": []any{
			map[string]any{
				"index":         0,
				"delta":         map[string]any{},
				"logprobs":      nil,
				"finish_reason": "stop",
			},
		},
	})
	flusher.Flush()
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}
