# mockllm

A mock HTTP server for the OpenAI API.

Useful when you want tests that run fast, offline, and produce the same
answer every time.

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
go run ./cmd/mockllm server
```

Listens on `:8080`. Set `OPENAI_MOCK_ADDR` to change it.

```
make run       # same, with debug logging
make check     # fmt, vet, test
```

## Running CLIs against the mock

Start the server in one terminal, then in another:

```
mockllm codex  [args...]
mockllm gemini [args...]
mockllm claude [args...]
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

Or use the `mockllm codex / gemini / claude` subcommands which do this automatically.
