package mockserver

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FixtureSet struct {
	Fixtures []Fixture `json:"fixtures"`
}

type Fixture struct {
	ID      string       `json:"id"`
	Match   FixtureMatch `json:"match"`
	Replies FixtureReply `json:"replies"`
}

type FixtureMatch struct {
	Endpoint         string            `json:"endpoint"`
	Metadata         map[string]string `json:"metadata"`
	InputContains    string            `json:"input_contains"`
	ChatContains     string            `json:"chat_contains"`
	HeaderResponseID string            `json:"header_response_id"`
}

type FixtureReply struct {
	ResponseText        string          `json:"response_text"`
	ChatText            string          `json:"chat_text"`
	ResponseUsage       *ResponsesUsage `json:"response_usage"`
	ChatUsage           *ChatUsage      `json:"chat_usage"`
	ResponseAnnotations []any           `json:"response_annotations"`
	ChatAnnotations     []any           `json:"chat_annotations"`
}

type ResponsesUsage struct {
	InputTokens     int `json:"input_tokens"`
	CachedTokens    int `json:"cached_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ReasoningTokens int `json:"reasoning_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

type ChatUsage struct {
	PromptTokens             int `json:"prompt_tokens"`
	CachedTokens             int `json:"cached_tokens"`
	PromptAudioTokens        int `json:"prompt_audio_tokens"`
	CompletionTokens         int `json:"completion_tokens"`
	ReasoningTokens          int `json:"reasoning_tokens"`
	CompletionAudioTokens    int `json:"completion_audio_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
	TotalTokens              int `json:"total_tokens"`
}

func LoadFixtures(path string) ([]Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read fixtures %q: %w", path, err)
	}

	var set FixtureSet
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("parse fixtures %q: %w", path, err)
	}

	for i := range set.Fixtures {
		if set.Fixtures[i].ID == "" {
			return nil, fmt.Errorf("fixture %d is missing id", i)
		}
		if set.Fixtures[i].Match.Endpoint == "" {
			return nil, fmt.Errorf("fixture %q is missing match.endpoint", set.Fixtures[i].ID)
		}
	}

	return set.Fixtures, nil
}

func (f Fixture) Matches(req MatchRequest) bool {
	if !strings.EqualFold(f.Match.Endpoint, req.Endpoint) {
		return false
	}
	if f.Match.HeaderResponseID != "" && f.Match.HeaderResponseID != req.HeaderResponseID {
		return false
	}
	for key, want := range f.Match.Metadata {
		if req.Metadata[key] != want {
			return false
		}
	}
	if f.Match.InputContains != "" && !strings.Contains(req.InputText, f.Match.InputContains) {
		return false
	}
	if f.Match.ChatContains != "" && !strings.Contains(req.ChatText, f.Match.ChatContains) {
		return false
	}
	return true
}

type MatchRequest struct {
	Endpoint         string
	Metadata         map[string]string
	InputText        string
	ChatText         string
	HeaderResponseID string
}
