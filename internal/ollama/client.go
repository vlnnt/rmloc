package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/valyala/fasthttp"
)

const (
	translationNumCtx        = 4096
	singleNumPredict         = 512
	multipleNumPredict       = 1536
	batchTranslationMaxTexts = 8
	batchTranslationMaxRunes = 2400
	validationAttempts       = 3
	requestAttempts          = 2
)

func NewClient(
	baseURL, model string,
	prompts PromptConfig,
) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	model = strings.TrimSpace(model)

	if err := prompts.validate(); err != nil {
		return nil, err
	}

	return &Client{
		baseURL:    baseURL,
		model:      model,
		prompt:     strings.TrimSpace(prompts.Prompt),
		promptHash: prompts.Fingerprint(),
		httpClient: &fasthttp.Client{
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 10 * time.Second,
		},
		timeout: 5 * time.Minute,
	}, nil
}

func (c *Client) Translate(
	ctx context.Context,
	text string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is empty")
	}

	if !needsTranslation(text) {
		return text, nil
	}

	if cached, ok := c.translationCache.Load(text); ok {
		return cached.(string), nil
	}

	protected := protectText(text)
	var lastErr error

	for attempt := 0; attempt < validationAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		prompt := c.prompt
		if attempt > 0 {
			prompt += "\nThe previous translation was invalid. Preserve every protected marker exactly once and return valid JSON."
		}

		var response translationResponse
		if err := c.chat(
			ctx,
			prompt,
			protected.Text,
			responseFormat,
			singleNumPredict,
			&response,
		); err != nil {
			return "", err
		}

		if strings.TrimSpace(response.Translation) == "" {
			lastErr = fmt.Errorf("ollama returned an empty translation")
			continue
		}

		translated, err := protected.restore(
			strings.TrimSpace(response.Translation),
		)
		if err != nil {
			lastErr = err
			continue
		}

		if strings.TrimSpace(translated) == "" {
			lastErr = fmt.Errorf("translation is empty after restoring markers")
			continue
		}

		if err := ctx.Err(); err != nil {
			return "", err
		}

		if err := c.persistTranslation(text, translated); err != nil {
			return "", fmt.Errorf("save translation cache: %w", err)
		}

		c.storeTranslationCacheEntry(text, translated)
		return translated, nil
	}

	fmt.Printf(
		"Warning: translation failed for %q: %v. Keeping original.\n",
		text,
		lastErr,
	)

	return text, nil
}

func (c *Client) TranslateBatch(
	ctx context.Context,
	texts []string,
) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("texts are empty")
	}

	result := copyStrings(texts)
	indexes := make(map[string][]int, len(texts))
	pending := make([]string, 0, len(texts))

	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if strings.TrimSpace(text) == "" || !needsTranslation(text) {
			continue
		}

		if cached, ok := c.translationCache.Load(text); ok {
			result[i] = cached.(string)
			continue
		}

		if _, exists := indexes[text]; !exists {
			pending = append(pending, text)
		}

		indexes[text] = append(indexes[text], i)
	}

	for start := 0; start < len(pending); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end, runes := start, 0
		for end < len(pending) && end-start < batchTranslationMaxTexts {
			count := utf8.RuneCountInString(pending[end])
			if end > start && runes+count > batchTranslationMaxRunes {
				break
			}

			runes += count
			end++
		}

		chunk := pending[start:end]
		var translations []string

		if len(chunk) == 1 {
			one, err := c.Translate(ctx, chunk[0])
			if err != nil {
				return nil, err
			}
			translations = []string{one}

		} else {
			var err error
			translations, err = c.translateMultiple(ctx, chunk, false)
			if err != nil {
				return nil, fmt.Errorf(
					"translate batch %d-%d: %w",
					start,
					end-1,
					err,
				)
			}
		}

		for i, original := range chunk {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			translated := translations[i]
			if _, cached := c.translationCache.Load(original); !cached {
				if err := c.persistTranslation(original, translated); err != nil {
					return nil, fmt.Errorf("save batch translation: %w", err)
				}

				c.storeTranslationCacheEntry(original, translated)
			}

			for _, resultIndex := range indexes[original] {
				result[resultIndex] = translated
			}
		}
		start = end
	}

	return result, nil
}

func (c *Client) TranslateLines(
	ctx context.Context,
	lines []string,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("lines are empty")
	}

	if !needsAnyTranslation(lines) {
		return copyStrings(lines), nil
	}

	body, err := json.Marshal(translationLinesRequest{
		Lines: lines,
	})
	if err != nil {
		return nil, err
	}

	key := string(body)
	if cached, ok := c.dialogueCache.Load(key); ok {
		return copyStrings(cached.([]string)), nil
	}

	translated, err := c.translateMultiple(ctx, lines, true)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := c.persistDialogue(key, translated); err != nil {
		return nil, fmt.Errorf("save dialogue cache: %w", err)
	}

	c.storeDialogueCacheEntry(key, translated)
	return copyStrings(translated), nil
}

func (c *Client) translateMultiple(
	ctx context.Context,
	lines []string,
	related bool,
) ([]string, error) {
	protectedLines, protectedValues := protectLines(lines)
	body, err := json.Marshal(translationLinesRequest{Lines: protectedLines})
	if err != nil {
		return nil, err
	}

	prompt := c.prompt
	if related {
		prompt += "\nUse neighboring lines as context. Preserve the array length and order."
	} else {
		prompt += "\nTranslate array elements independently. Preserve the array length and order."
	}

	var lastErr error
	for attempt := 0; attempt < validationAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		currentPrompt := prompt
		if attempt > 0 {
			currentPrompt += "\nThe previous translation was invalid. Keep every protected marker in its original array element and return valid JSON."
		}

		var response translationLinesResponse
		if err := c.chat(
			ctx,
			currentPrompt,
			string(body),
			translationLinesResponseFormat,
			multipleNumPredict,
			&response,
		); err != nil {
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				return nil, err
			}

			lastErr = err
			break
		}

		if len(response.Translations) != len(lines) {
			lastErr = fmt.Errorf(
				"ollama returned %d lines, expected %d",
				len(response.Translations),
				len(lines),
			)
			continue
		}

		translated := make([]string, len(lines))
		valid := true

		for i, value := range response.Translations {
			if strings.TrimSpace(value) == "" {
				lastErr = fmt.Errorf("empty translation at index %d", i)
				valid = false
				break
			}

			restored, err := protectedValues[i].restore(
				strings.TrimSpace(value),
			)
			if err != nil {
				lastErr = fmt.Errorf("line %d: %w", i, err)
				valid = false
				break
			}

			if strings.TrimSpace(restored) == "" {
				lastErr = fmt.Errorf("empty translation after restoring line %d", i)
				valid = false
				break
			}

			translated[i] = restored
		}

		if valid {
			return translated, nil
		}
	}

	result := make([]string, len(lines))
	for i, line := range lines {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if strings.TrimSpace(line) == "" {
			result[i] = line
			continue
		}

		translated, err := c.Translate(ctx, line)
		if err != nil {
			return nil, fmt.Errorf(
				"group validation failed (%v); translate line %d: %w",
				lastErr,
				i,
				err,
			)
		}

		result[i] = translated
	}

	return result, nil
}

func (c *Client) chat(
	ctx context.Context,
	system, user string,
	format json.RawMessage,
	numPredict int,
	result any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	body, err := json.Marshal(chatRequest{
		Model:     c.model,
		Stream:    false,
		Think:     false,
		KeepAlive: "10m",
		Format:    format,
		Options: chatOptions{
			Temperature: 0,
			NumPredict:  numPredict,
			NumCtx:      translationNumCtx,
		},
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return fmt.Errorf("encode Ollama request: %w", err)
	}

	for attempt := 0; attempt < requestAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		retry, err := c.executeChatRequest(ctx, body, result)
		if err == nil || !retry || attempt == requestAttempts-1 {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(300 * time.Millisecond):
		}
	}

	return fmt.Errorf("ollama request attempts exhausted")
}

func (c *Client) executeChatRequest(
	ctx context.Context,
	body []byte,
	result any,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	request.Header.SetMethod(fasthttp.MethodPost)
	request.Header.SetContentType("application/json")
	request.SetRequestURI(c.baseURL + "/api/chat")
	request.SetBody(body)

	err := c.httpClient.DoTimeout(
		request,
		response,
		c.timeout,
	)

	status := response.StatusCode()
	responseBody := append([]byte(nil), response.Body()...)

	defer fasthttp.ReleaseRequest(request)
	defer fasthttp.ReleaseResponse(response)

	if err := ctx.Err(); err != nil {
		return false, err
	}

	if err != nil {
		return true, fmt.Errorf("request Ollama: %w", err)
	}

	if status != fasthttp.StatusOK {
		return status == fasthttp.StatusTooManyRequests ||
				status >= fasthttp.StatusInternalServerError,
			fmt.Errorf(
				"ollama returned HTTP %d: %s",
				status,
				strings.TrimSpace(string(responseBody)),
			)
	}

	var responseData chatResponse
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		return true, fmt.Errorf("decode Ollama response: %w", err)
	}

	if responseData.Error != "" {
		return false, fmt.Errorf("ollama error: %s", responseData.Error)
	}

	content := strings.TrimSpace(responseData.Message.Content)
	if content == "" {
		return true, fmt.Errorf("ollama returned empty content")
	}

	if err := json.Unmarshal([]byte(content), result); err != nil {
		return true, fmt.Errorf("decode translation JSON: %w", err)
	}

	return false, nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}
