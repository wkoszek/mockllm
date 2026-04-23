package mockserver

import (
	"fmt"
	"strings"
)

func isLLMSizePrompt(text string) bool {
	trimmed := strings.TrimRight(text, " \t\r\n")
	return strings.HasSuffix(trimmed, "\nllm:size")
}

func renderSizeReply(inputTokens, cachedTokens, reasoningTokens int) (string, int, int) {
	outputTokens := 0
	for range 8 {
		totalTokens := inputTokens + outputTokens
		text := fmt.Sprintf(
			"reply: input_tokens=%d cached_tokens=%d output_tokens=%d reasoning_tokens=%d total_tokens=%d",
			inputTokens,
			cachedTokens,
			outputTokens,
			reasoningTokens,
			totalTokens,
		)
		newOutputTokens := approxTokenCount(text)
		if newOutputTokens == outputTokens {
			return text, outputTokens, totalTokens
		}
		outputTokens = newOutputTokens
	}

	totalTokens := inputTokens + outputTokens
	text := fmt.Sprintf(
		"reply: input_tokens=%d cached_tokens=%d output_tokens=%d reasoning_tokens=%d total_tokens=%d",
		inputTokens,
		cachedTokens,
		outputTokens,
		reasoningTokens,
		totalTokens,
	)
	return text, outputTokens, totalTokens
}
