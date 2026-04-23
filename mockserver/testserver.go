package mockserver

import (
	"net/http/httptest"
	"testing"
)

// TestServer is a mock LLM HTTP server backed by httptest.Server.
// Create one with NewTestServer; it shuts down automatically when the test ends.
type TestServer struct {
	*httptest.Server
}

// NewTestServer starts a mock LLM server on a random port and registers
// t.Cleanup to shut it down. Pass fixtures for deterministic per-request
// responses; omit them to get the default response text for every request.
//
// Example:
//
//	ts := mockserver.NewTestServer(t,
//	    mockserver.Fixture{
//	        ID:      "greet",
//	        Match:   mockserver.FixtureMatch{Endpoint: "responses", InputContains: "hello"},
//	        Replies: mockserver.FixtureReply{ResponseText: "Hi there!"},
//	    },
//	)
//	client := openai.NewClient(openai.WithBaseURL(ts.BaseURL()), openai.WithAPIKey("mock"))
func NewTestServer(t testing.TB, fixtures ...Fixture) *TestServer {
	t.Helper()
	cfg := Config{
		DefaultModel:            "gpt-4o",
		DefaultResponseText:     "Mock response.",
		DefaultChatResponseText: "Mock chat response.",
		Fixtures:                fixtures,
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("mockserver.NewTestServer: %v", err)
	}
	hs := httptest.NewServer(s.Handler())
	t.Cleanup(hs.Close)
	return &TestServer{Server: hs}
}

// BaseURL returns the root URL of the mock server (e.g. "http://127.0.0.1:PORT").
// Pass it as OPENAI_BASE_URL or the equivalent SDK option in the code under test.
func (ts *TestServer) BaseURL() string {
	return ts.URL
}
