package mockserver

import "os"

type Config struct {
	ListenAddr              string
	FixturesPath            string
	OpenAIBaseURL           string
	OpenAIAPIKey            string
	EnableOpenAIInputTokens bool
	Debug                   bool
	DefaultModel            string
	DefaultResponseText     string
	DefaultChatResponseText string
}

func LoadConfigFromEnv() Config {
	cfg := Config{
		ListenAddr:              getenv("OPENAI_MOCK_ADDR", ":8080"),
		FixturesPath:            getenv("OPENAI_MOCK_FIXTURES", "fixtures/fixtures.json"),
		OpenAIBaseURL:           getenv("OPENAI_MOCK_OPENAI_BASE_URL", "https://api.openai.com"),
		OpenAIAPIKey:            os.Getenv("OPENAI_API_KEY"),
		EnableOpenAIInputTokens: getenv("OPENAI_MOCK_ENABLE_OPENAI_INPUT_TOKENS", "0") == "1",
		Debug:                   getenv("OPENAI_MOCK_DEBUG", "0") == "1",
		DefaultModel:            getenv("OPENAI_MOCK_DEFAULT_MODEL", "gpt-5.4"),
		DefaultResponseText:     getenv("OPENAI_MOCK_DEFAULT_RESPONSES_TEXT", "Mock response."),
		DefaultChatResponseText: getenv("OPENAI_MOCK_DEFAULT_CHAT_TEXT", "Mock chat response."),
	}
	return cfg
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
