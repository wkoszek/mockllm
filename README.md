# mockai

A mock HTTP server for the OpenAI API.

Useful when you want tests that run fast, offline, and produce the same
answer every time.

## Use as a Go library in tests

```
go get github.com/wkoszek/mockai
```

```go
import "github.com/wkoszek/mockai/mockserver"
```

Start a server on a random port in one line. It shuts down automatically when
the test ends.

```go
func TestMyAIFeature(t *testing.T) {
    ts := mockserver.NewTestServer(t,
        mockserver.Fixture{
            ID: "summarize",
            Match: mockserver.FixtureMatch{
                Endpoint:      "responses",
                InputContains: "summarize",
            },
            Replies: mockserver.FixtureReply{
                ResponseText: "Short summary.",
                ResponseUsage: &mockserver.ResponsesUsage{
                    InputTokens:  100,
                    OutputTokens: 10,
                    TotalTokens:  110,
                },
            },
        },
    )
    // ts.BaseURL() → "http://127.0.0.1:<random-port>"
    client := openai.NewClient(
        option.WithBaseURL(ts.BaseURL()),
        option.WithAPIKey("mock"),
    )
    resp, _ := client.Responses.New(ctx, openai.ResponseNewParams{
        Model: openai.String("gpt-4o"),
        Input: openai.ResponseNewParamsInputUnion{...},
    })
    // assert on resp...
}
```

Without fixtures every request returns the default text (`"Mock response."` /
`"Mock chat response."`), which is enough for tests that only care about
whether the call was made at all.

To load fixtures from a JSON file instead of constructing them in Go, use
[`LoadFixtures`](mockserver/fixtures.go) and pass the result via
`Config.Fixtures`:

```go
fixtures, _ := mockserver.LoadFixtures("testdata/fixtures.json")
ts := mockserver.NewTestServer(t, fixtures...)
```

## Endpoints

```
POST /v1/chat/completions
POST /v1/responses
POST /v1/responses/input_tokens
GET  /healthz
```

Both endpoints support streaming.

## Build and run

```
go run ./cmd/mockai server
```

Listens on `:8080`. Set `OPENAI_MOCK_ADDR` to change it.

```
make run       # same, with debug logging
make check     # fmt, vet, test
```

## Running CLIs against the mock

Start the server in one terminal, then in another:

```
mockai codex  [args...]
mockai gemini [args...]
mockai claude [args...]
```

Each subcommand sets `OPENAI_BASE_URL` and `OPENAI_API_KEY=mock`, then
execs the real binary with your arguments. The binary takes over the
process — signals, stdin, stdout all work normally.

If `OPENAI_MOCK_ADDR` is set, the base URL is derived from it automatically.

## Configuration

All settings come from environment variables.

| Variable | Default | Meaning |
|---|---|---|
| `OPENAI_MOCK_ADDR` | `:8080` | listen address |
| `OPENAI_MOCK_FIXTURES` | `fixtures/fixtures.json` | fixture file |
| `OPENAI_MOCK_DEFAULT_RESPONSES_TEXT` | `Mock response.` | fallback reply for `/v1/responses` |
| `OPENAI_MOCK_DEFAULT_CHAT_TEXT` | `Mock chat response.` | fallback reply for `/v1/chat/completions` |
| `OPENAI_MOCK_DEFAULT_MODEL` | `gpt-5.4` | model name echoed back |
| `OPENAI_MOCK_DEBUG` | `0` | set to `1` for per-request log lines |
| `OPENAI_MOCK_ENABLE_OPENAI_INPUT_TOKENS` | `0` | set to `1` to proxy `/v1/responses/input_tokens` to real OpenAI |
| `OPENAI_API_KEY` | — | required when `OPENAI_MOCK_ENABLE_OPENAI_INPUT_TOKENS=1` |

## Fixtures

Fixtures are in `fixtures/fixtures.json`.

```json
{
  "fixtures": [
    {
      "id": "chat-hello",
      "match": {
        "endpoint": "chat_completions",
        "metadata": { "test_case": "hello" }
      },
      "replies": {
        "chat_text": "Hello.",
        "chat_usage": {
          "prompt_tokens": 15,
          "completion_tokens": 10,
          "total_tokens": 25
        }
      }
    }
  ]
}
```

A request matches a fixture when all specified `match` fields agree:

- `endpoint` — `responses` or `chat_completions`
- `metadata` — exact key/value subset of the request `metadata` object
- `input_contains` — substring match on the flattened prompt text
- `chat_contains` — substring match on the flattened chat messages
- `header_response_id` — exact match on the `X-Mock-Response-ID` request header

First match wins. No match returns the default fallback text.

Usage fields in fixtures are returned verbatim. Omit them and the server
approximates counts from word length. For exact billing tests, set them
explicitly.

## Size mode

Append `\nllm:size` to any prompt. The server replies with a line showing
the token counts it computed:

```
reply: input_tokens=42 cached_tokens=0 output_tokens=13 reasoning_tokens=0 total_tokens=55
```

Works on both endpoints.

## Pointing a client at it manually

Any API key is accepted. Set:

```
export OPENAI_BASE_URL=http://127.0.0.1:8080
export OPENAI_API_KEY=mock
```

Or use the `mockai codex / gemini / claude` subcommands which do this automatically.
