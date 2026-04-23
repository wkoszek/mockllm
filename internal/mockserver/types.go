package mockserver

import (
	"encoding/json"
	"strings"
)

type responsesRequest struct {
	Model           string            `json:"model"`
	Input           any               `json:"input"`
	Instructions    *string           `json:"instructions"`
	Metadata        map[string]string `json:"metadata"`
	Stream          bool              `json:"stream"`
	Store           *bool             `json:"store"`
	Temperature     *float64          `json:"temperature"`
	TopP            *float64          `json:"top_p"`
	ToolChoice      any               `json:"tool_choice"`
	Tools           []any             `json:"tools"`
	Text            any               `json:"text"`
	Truncation      *string           `json:"truncation"`
	PreviousRespID  *string           `json:"previous_response_id"`
	Reasoning       any               `json:"reasoning"`
	MaxOutputTokens *int              `json:"max_output_tokens"`
	User            any               `json:"user"`
}

func (r responsesRequest) inputText() string {
	return flattenAnyToText(r.Input)
}

type chatRequest struct {
	Model          string            `json:"model"`
	Messages       []chatMessage     `json:"messages"`
	Metadata       map[string]string `json:"metadata"`
	Stream         bool              `json:"stream"`
	Temperature    *float64          `json:"temperature"`
	TopP           *float64          `json:"top_p"`
	Store          *bool             `json:"store"`
	Tools          []any             `json:"tools"`
	ToolChoice     any               `json:"tool_choice"`
	ResponseFormat any               `json:"response_format"`
	MaxTokens      *int              `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (r chatRequest) chatText() string {
	var parts []string
	for _, msg := range r.Messages {
		if text := flattenAnyToText(msg.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func flattenAnyToText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []any:
		var parts []string
		for _, item := range x {
			if text := flattenAnyToText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		if text, ok := x["text"].(string); ok {
			return text
		}
		if inputText, ok := x["input_text"].(string); ok {
			return inputText
		}
		if content, ok := x["content"]; ok {
			return flattenAnyToText(content)
		}
		raw, _ := json.Marshal(x)
		return string(raw)
	default:
		raw, _ := json.Marshal(x)
		return string(raw)
	}
}

func approxTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	// This is only a fallback. Exact billing-grade counts should come from fixtures
	// or the official /v1/responses/input_tokens endpoint.
	words := len(strings.Fields(text))
	if words == 0 {
		return 1
	}
	return words + max(1, len(text)/24)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
