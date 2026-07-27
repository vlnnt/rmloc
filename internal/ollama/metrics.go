package ollama

import (
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

type clientMetrics struct {
	inputTexts               atomic.Int64
	cacheHits                atomic.Int64
	skippedTexts             atomic.Int64
	modelTranslatedTexts     atomic.Int64
	safetyFallbackTexts      atomic.Int64
	httpAttempts             atomic.Int64
	successfulHTTPRequests   atomic.Int64
	failedHTTPAttempts       atomic.Int64
	retryAttempts            atomic.Int64
	promptTokens             atomic.Int64
	generatedTokens          atomic.Int64
	ollamaTotalDuration      atomic.Int64
	ollamaLoadDuration       atomic.Int64
	promptEvaluationDuration atomic.Int64
	generationDuration       atomic.Int64
	httpWallDuration         atomic.Int64
}

type MetricsSnapshot struct {
	InputTexts               int64
	CacheHits                int64
	SkippedTexts             int64
	ModelTranslatedTexts     int64
	SafetyFallbackTexts      int64
	HTTPAttempts             int64
	SuccessfulHTTPRequests   int64
	FailedHTTPAttempts       int64
	RetryAttempts            int64
	PromptTokens             int64
	GeneratedTokens          int64
	OllamaTotalDuration      time.Duration
	OllamaLoadDuration       time.Duration
	PromptEvaluationDuration time.Duration
	GenerationDuration       time.Duration
	HTTPWallDuration         time.Duration
}

func (c *Client) Metrics() MetricsSnapshot {
	return MetricsSnapshot{
		InputTexts:   c.metrics.inputTexts.Load(),
		CacheHits:    c.metrics.cacheHits.Load(),
		SkippedTexts: c.metrics.skippedTexts.Load(),
		ModelTranslatedTexts: c.metrics.
			modelTranslatedTexts.Load(),
		SafetyFallbackTexts: c.metrics.
			safetyFallbackTexts.Load(),
		HTTPAttempts: c.metrics.httpAttempts.Load(),
		SuccessfulHTTPRequests: c.metrics.
			successfulHTTPRequests.Load(),
		FailedHTTPAttempts: c.metrics.
			failedHTTPAttempts.Load(),
		RetryAttempts: c.metrics.retryAttempts.Load(),
		PromptTokens:  c.metrics.promptTokens.Load(),
		GeneratedTokens: c.metrics.
			generatedTokens.Load(),
		OllamaTotalDuration: time.Duration(
			c.metrics.ollamaTotalDuration.Load(),
		),
		OllamaLoadDuration: time.Duration(
			c.metrics.ollamaLoadDuration.Load(),
		),
		PromptEvaluationDuration: time.Duration(
			c.metrics.promptEvaluationDuration.Load(),
		),
		GenerationDuration: time.Duration(
			c.metrics.generationDuration.Load(),
		),
		HTTPWallDuration: time.Duration(
			c.metrics.httpWallDuration.Load(),
		),
	}
}

func (c *Client) PrintMetrics(
	writer io.Writer,
) {
	metrics := c.Metrics()

	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Translation statistics:")

	fmt.Fprintf(
		writer,
		"  Processed text entries: %d\n",
		metrics.InputTexts,
	)

	fmt.Fprintf(
		writer,
		"  Translated by model: %d\n",
		metrics.ModelTranslatedTexts,
	)

	fmt.Fprintf(
		writer,
		"  Left unchanged after protection errors: %d\n",
		metrics.SafetyFallbackTexts,
	)

	fmt.Fprintf(
		writer,
		"  Loaded from cache: %d\n",
		metrics.CacheHits,
	)

	fmt.Fprintf(
		writer,
		"  Skipped without translation: %d\n",
		metrics.SkippedTexts,
	)

	fmt.Fprintf(
		writer,
		"  HTTP requests: %d\n",
		metrics.HTTPAttempts,
	)

	fmt.Fprintf(
		writer,
		"  Successful HTTP requests: %d\n",
		metrics.SuccessfulHTTPRequests,
	)

	fmt.Fprintf(
		writer,
		"  Failed HTTP attempts: %d\n",
		metrics.FailedHTTPAttempts,
	)

	fmt.Fprintf(
		writer,
		"  Retry attempts: %d\n",
		metrics.RetryAttempts,
	)

	fmt.Fprintf(
		writer,
		"  Prompt tokens: %d\n",
		metrics.PromptTokens,
	)

	fmt.Fprintf(
		writer,
		"  Generated tokens: %d\n",
		metrics.GeneratedTokens,
	)

	fmt.Fprintf(
		writer,
		"  Ollama processing time: %s\n",
		formatMetricDuration(
			metrics.OllamaTotalDuration,
		),
	)

	fmt.Fprintf(
		writer,
		"  HTTP wait time: %s\n",
		formatMetricDuration(
			metrics.HTTPWallDuration,
		),
	)

	fmt.Fprintf(
		writer,
		"  Model loading time: %s\n",
		formatMetricDuration(
			metrics.OllamaLoadDuration,
		),
	)

	if metrics.PromptEvaluationDuration > 0 {
		fmt.Fprintf(
			writer,
			"  Prompt processing speed: %.1f tokens/s\n",
			tokensPerSecond(
				metrics.PromptTokens,
				metrics.PromptEvaluationDuration,
			),
		)
	}

	if metrics.GenerationDuration > 0 {
		fmt.Fprintf(
			writer,
			"  Generation speed: %.1f tokens/s\n",
			tokensPerSecond(
				metrics.GeneratedTokens,
				metrics.GenerationDuration,
			),
		)
	}
}

func (c *Client) recordInputTexts(
	count int,
) {
	c.metrics.inputTexts.Add(
		int64(count),
	)
}

func (c *Client) recordCacheHit(
	count int,
) {
	c.metrics.cacheHits.Add(
		int64(count),
	)
}

func (c *Client) recordSkippedTexts(
	count int,
) {
	c.metrics.skippedTexts.Add(
		int64(count),
	)
}

func (c *Client) recordModelTranslatedTexts(
	count int,
) {
	c.metrics.modelTranslatedTexts.Add(
		int64(count),
	)
}

func (c *Client) recordSafetyFallback(
	count int,
) {
	c.metrics.safetyFallbackTexts.Add(
		int64(count),
	)
}

func (c *Client) recordHTTPAttempt() {
	c.metrics.httpAttempts.Add(1)
}

func (c *Client) recordSuccessfulHTTPResponse(
	response chatResponse,
	wallDuration time.Duration,
) {
	c.metrics.successfulHTTPRequests.Add(1)

	c.metrics.promptTokens.Add(
		int64(response.PromptEvalCount),
	)

	c.metrics.generatedTokens.Add(
		int64(response.EvalCount),
	)

	c.metrics.ollamaTotalDuration.Add(
		response.TotalDuration,
	)

	c.metrics.ollamaLoadDuration.Add(
		response.LoadDuration,
	)

	c.metrics.promptEvaluationDuration.Add(
		response.PromptEvalDuration,
	)

	c.metrics.generationDuration.Add(
		response.EvalDuration,
	)

	c.metrics.httpWallDuration.Add(
		int64(wallDuration),
	)
}

func (c *Client) recordFailedHTTPAttempt(
	wallDuration time.Duration,
) {
	c.metrics.failedHTTPAttempts.Add(1)

	c.metrics.httpWallDuration.Add(
		int64(wallDuration),
	)
}

func (c *Client) recordRetryAttempt() {
	c.metrics.retryAttempts.Add(1)
}

func tokensPerSecond(
	tokens int64,
	duration time.Duration,
) float64 {
	if tokens <= 0 || duration <= 0 {
		return 0
	}

	return float64(tokens) /
		duration.Seconds()
}

func formatMetricDuration(
	duration time.Duration,
) string {
	if duration <= 0 {
		return "0s"
	}

	return duration.
		Round(time.Millisecond).
		String()
}
