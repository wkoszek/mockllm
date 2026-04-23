// Package mockserver provides a mock HTTP server that implements the OpenAI API.
//
// Use [NewTestServer] to spin up a server inside a Go test — it binds to a
// random port and shuts down automatically when the test ends:
//
//	func TestMyFeature(t *testing.T) {
//	    ts := mockserver.NewTestServer(t,
//	        mockserver.Fixture{
//	            ID:      "summarize",
//	            Match:   mockserver.FixtureMatch{Endpoint: "responses", InputContains: "summarize"},
//	            Replies: mockserver.FixtureReply{ResponseText: "Short summary."},
//	        },
//	    )
//	    client := openai.NewClient(
//	        option.WithBaseURL(ts.BaseURL()),
//	        option.WithAPIKey("mock"),
//	    )
//	    // ... test assertions
//	}
//
// For standalone use, create a [Server] directly with [New] and attach its
// [Server.Handler] to a [net/http.Server].  Configure it via [Config] or load
// settings from environment variables with [LoadConfigFromEnv].
//
// Fixture matching
//
// Each request is compared against the fixture list in order; the first match
// wins. A fixture matches when every field it specifies agrees with the
// request: endpoint name, metadata key/value pairs, substrings in the prompt
// or chat messages, and an optional X-Mock-Response-ID header. Unmatched
// requests return the default response text.
package mockserver
