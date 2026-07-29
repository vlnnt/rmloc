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
	translationNumCtx = 4096

	singleTranslationNumPredict = 512

	batchTranslationNumPredict = 1536
	batchTranslationMaxTexts   = 8
	batchTranslationMaxRunes   = 2400

	dialogueTranslationNumPredict = 1536

	maxTranslationValidationAttempts = 3
	translationValidationRetryDelay  = 500 * time.Millisecond
)

type translationValidationError struct {
	attempts int
	err      error
}

type structuredResponseError struct {
	content string
	err     error
}

type batchTextEntry struct {
	text    string
	indexes []int
}

func NewClient(
	baseURL string,
	model string,
	prompts PromptConfig,
) (*Client, error) {
	baseURL = strings.TrimRight(
		strings.TrimSpace(baseURL),
		"/",
	)
	if baseURL == "" {
		return nil, fmt.Errorf(
			"Ollama base URL is empty",
		)
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf(
			"Ollama model is empty",
		)
	}

	if err := prompts.validate(); err != nil {
		return nil, fmt.Errorf(
			"validate prompts: %w",
			err,
		)
	}

	legacyPromptHash := prompts.LegacyFingerprint()

	return &Client{
		baseURL: baseURL,
		model:   model,

		singleTranslationPrompt: strings.TrimSpace(
			prompts.SingleTranslation,
		),
		batchTranslationPrompt: strings.TrimSpace(
			prompts.BatchTranslation,
		),
		dialogueTranslationPrompt: strings.TrimSpace(
			prompts.DialogueTranslation,
		),

		legacyPromptHash:   legacyPromptHash,
		singlePromptHash:   prompts.SingleFingerprint(),
		batchPromptHash:    prompts.BatchFingerprint(),
		dialoguePromptHash: prompts.DialogueFingerprint(),

		allowLegacyCache: legacyPromptHash ==
			legacyDefaultPromptHash,

		httpClient: &fasthttp.Client{
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 10 * time.Second,
		},
		timeout: 5 * time.Minute,
	}, nil
}

func (c *Client) Translate(
	text string,
) (string, error) {
	return c.TranslateContext(
		context.Background(),
		text,
	)
}

func (c *Client) TranslateContext(
	ctx context.Context,
	text string,
) (string, error) {
	return c.translate(
		normalizeContext(ctx),
		text,
		true,
	)
}

func (c *Client) translate(
	ctx context.Context,
	text string,
	recordInput bool,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if recordInput {
		c.recordInputTexts(1)
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf(
			"text is empty",
		)
	}

	if !needsTranslation(text) {
		c.recordSkippedTexts(1)

		return text, nil
	}

	if cached, exists := c.translationCache.Load(text); exists {
		translation, ok := cached.(string)
		if !ok {
			return "", fmt.Errorf(
				"invalid cached translation type",
			)
		}

		c.recordCacheHit(1)

		return translation, nil
	}

	requestKey := "translation:" + text

	value, err, _ := c.doSingleflightContext(
		ctx,
		requestKey,
		func() (any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			if cached, exists := c.translationCache.Load(text); exists {
				translation, ok := cached.(string)
				if !ok {
					return nil, fmt.Errorf(
						"invalid cached translation type",
					)
				}

				c.recordCacheHit(1)

				return translation, nil
			}

			protected := protectText(text)

			restoredTranslation, err := c.translateProtectedText(
				ctx,
				protected,
			)
			if err != nil {
				if !isTranslationValidationError(err) {
					return nil, err
				}

				if persistErr := c.persistTranslation(
					text,
					text,
				); persistErr != nil {
					return nil, fmt.Errorf(
						"persist unchanged translation: %w",
						persistErr,
					)
				}

				c.storeTranslationCacheEntry(
					text,
					text,
				)

				c.recordSafetyFallback(1)

				fmt.Printf(
					"Warning: the text could not be translated safely; "+
						"the original text was left unchanged. "+
						"Text: %q. Reason: %v\n",
					truncateForLog(
						text,
						180,
					),
					err,
				)

				return text, nil
			}

			if err := ctx.Err(); err != nil {
				return nil, err
			}

			if err := c.persistTranslation(
				text,
				restoredTranslation,
			); err != nil {
				return nil, fmt.Errorf(
					"persist translation: %w",
					err,
				)
			}

			c.storeTranslationCacheEntry(
				text,
				restoredTranslation,
			)

			c.recordModelTranslatedTexts(1)

			return restoredTranslation, nil
		},
	)
	if err != nil {
		return "", err
	}

	translation, ok := value.(string)
	if !ok {
		return "", fmt.Errorf(
			"invalid translation result type",
		)
	}

	return translation, nil
}

func (c *Client) TranslateBatch(
	texts []string,
) ([]string, error) {
	return c.TranslateBatchContext(
		context.Background(),
		texts,
	)
}

func (c *Client) TranslateBatchContext(
	ctx context.Context,
	texts []string,
) ([]string, error) {
	ctx = normalizeContext(ctx)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.recordInputTexts(
		len(texts),
	)

	if len(texts) == 0 {
		return nil, fmt.Errorf(
			"texts are empty",
		)
	}

	result := copyStrings(texts)

	entries := make(
		[]batchTextEntry,
		0,
		len(texts),
	)

	entryByText := make(
		map[string]int,
		len(texts),
	)

	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if strings.TrimSpace(text) == "" {
			c.recordSkippedTexts(1)
			continue
		}

		if !needsTranslation(text) {
			c.recordSkippedTexts(1)
			continue
		}

		if cached, exists := c.translationCache.Load(text); exists {
			translation, ok := cached.(string)
			if !ok {
				return nil, fmt.Errorf(
					"invalid cached translation type",
				)
			}

			result[index] = translation
			c.recordCacheHit(1)

			continue
		}

		if entryIndex, exists := entryByText[text]; exists {
			entries[entryIndex].indexes = append(
				entries[entryIndex].indexes,
				index,
			)

			continue
		}

		entryByText[text] = len(entries)

		entries = append(
			entries,
			batchTextEntry{
				text: text,
				indexes: []int{
					index,
				},
			},
		)
	}

	if len(entries) == 0 {
		return result, nil
	}

	pendingTexts := make(
		[]string,
		len(entries),
	)

	for index := range entries {
		pendingTexts[index] = entries[index].text
	}

	translations, err := c.translateBatchTexts(
		ctx,
		pendingTexts,
	)
	if err != nil {
		return nil, err
	}

	if len(translations) != len(entries) {
		return nil, fmt.Errorf(
			"batch translation count mismatch: received %d, expected %d",
			len(translations),
			len(entries),
		)
	}

	for entryIndex, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		translation := translations[entryIndex]

		for _, resultIndex := range entry.indexes {
			result[resultIndex] = translation
		}

		if len(entry.indexes) > 1 {
			c.recordCacheHit(
				len(entry.indexes) - 1,
			)
		}
	}

	return result, nil
}

func (c *Client) translateBatchTexts(
	ctx context.Context,
	texts []string,
) ([]string, error) {
	result := make(
		[]string,
		0,
		len(texts),
	)

	for start := 0; start < len(texts); {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := batchTextEnd(
			texts,
			start,
		)

		translations, err := c.translateBatchChunk(
			ctx,
			texts[start:end],
		)
		if err != nil {
			return nil, fmt.Errorf(
				"translate batch elements %d-%d: %w",
				start,
				end-1,
				err,
			)
		}

		result = append(
			result,
			translations...,
		)

		start = end
	}

	return result, nil
}

func batchTextEnd(
	texts []string,
	start int,
) int {
	if start < 0 ||
		start >= len(texts) {
		return start
	}

	end := start
	totalRunes := 0

	for end < len(texts) &&
		end-start < batchTranslationMaxTexts {
		textRunes := utf8.RuneCountInString(
			texts[end],
		)

		if end > start &&
			totalRunes+textRunes > batchTranslationMaxRunes {
			break
		}

		totalRunes += textRunes
		end++
	}

	if end == start {
		return start + 1
	}

	return end
}

func (c *Client) translateBatchChunk(
	ctx context.Context,
	texts []string,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(texts) == 0 {
		return nil, nil
	}

	result := make(
		[]string,
		len(texts),
	)

	pendingTexts := make(
		[]string,
		0,
		len(texts),
	)

	pendingIndexes := make(
		[]int,
		0,
		len(texts),
	)

	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if cached, exists := c.translationCache.Load(text); exists {
			translation, ok := cached.(string)
			if !ok {
				return nil, fmt.Errorf(
					"invalid cached translation type",
				)
			}

			result[index] = translation
			c.recordCacheHit(1)

			continue
		}

		pendingTexts = append(
			pendingTexts,
			text,
		)

		pendingIndexes = append(
			pendingIndexes,
			index,
		)
	}

	if len(pendingTexts) == 0 {
		return result, nil
	}

	translations, err := c.translateBatchRequest(
		ctx,
		pendingTexts,
	)
	if err != nil {
		if !isTranslationValidationError(err) {
			return nil, err
		}

		if len(pendingTexts) == 1 {
			translation, lineErr := c.translate(
				ctx,
				pendingTexts[0],
				false,
			)
			if lineErr != nil {
				return nil, fmt.Errorf(
					"translate batch element separately: %w",
					lineErr,
				)
			}

			translations = []string{
				translation,
			}
		} else {
			fmt.Printf(
				"Warning: the batch translation of %d independent texts "+
					"failed validation; the batch will be split. "+
					"Reason: %v\n",
				len(pendingTexts),
				err,
			)

			middle := len(pendingTexts) / 2

			leftTranslations, leftErr := c.translateBatchChunk(
				ctx,
				pendingTexts[:middle],
			)
			if leftErr != nil {
				return nil, leftErr
			}

			rightTranslations, rightErr := c.translateBatchChunk(
				ctx,
				pendingTexts[middle:],
			)
			if rightErr != nil {
				return nil, rightErr
			}

			translations = append(
				leftTranslations,
				rightTranslations...,
			)
		}
	}

	if len(translations) != len(pendingTexts) {
		return nil, fmt.Errorf(
			"batch translation count mismatch after fallback: received %d, expected %d",
			len(translations),
			len(pendingTexts),
		)
	}

	for index, pendingIndex := range pendingIndexes {
		result[pendingIndex] = translations[index]
	}

	return result, nil
}

func (c *Client) translateBatchRequest(
	ctx context.Context,
	texts []string,
) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	protectedLines, protectedValues := protectLines(
		texts,
	)

	restoredTranslations, err := c.translateProtectedBatch(
		ctx,
		protectedLines,
		protectedValues,
	)
	if err != nil {
		return nil, err
	}

	for index, translation := range restoredTranslations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if err := c.persistBatchTranslation(
			texts[index],
			translation,
		); err != nil {
			return nil, fmt.Errorf(
				"persist batch translation %d: %w",
				index,
				err,
			)
		}

		c.storeTranslationCacheEntry(
			texts[index],
			translation,
		)
	}

	c.recordModelTranslatedTexts(
		len(texts),
	)

	return copyStrings(
		restoredTranslations,
	), nil
}

func (c *Client) TranslateLines(
	lines []string,
) ([]string, error) {
	return c.TranslateLinesContext(
		context.Background(),
		lines,
	)
}

func (c *Client) TranslateLinesContext(
	ctx context.Context,
	lines []string,
) ([]string, error) {
	ctx = normalizeContext(ctx)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.recordInputTexts(
		len(lines),
	)

	if len(lines) == 0 {
		return nil, fmt.Errorf(
			"lines are empty",
		)
	}

	if !needsAnyTranslation(lines) {
		c.recordSkippedTexts(
			len(lines),
		)

		return copyStrings(lines), nil
	}

	originalRequestBody, err := json.Marshal(
		translationLinesRequest{
			Lines: lines,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal original dialogue lines: %w",
			err,
		)
	}

	cacheKey := string(
		originalRequestBody,
	)

	if cached, exists := c.dialogueCache.Load(cacheKey); exists {
		translations, ok := cached.([]string)
		if !ok {
			return nil, fmt.Errorf(
				"invalid cached dialogue translation type",
			)
		}

		c.recordCacheHit(
			len(lines),
		)

		return copyStrings(
			translations,
		), nil
	}

	requestKey := "dialogue:" + cacheKey

	value, err, _ := c.doSingleflightContext(
		ctx,
		requestKey,
		func() (any, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			if cached, exists := c.dialogueCache.Load(cacheKey); exists {
				translations, ok := cached.([]string)
				if !ok {
					return nil, fmt.Errorf(
						"invalid cached dialogue translation type",
					)
				}

				c.recordCacheHit(
					len(lines),
				)

				return copyStrings(
					translations,
				), nil
			}

			protectedLines, protectedValues := protectLines(
				lines,
			)

			restoredTranslations, err := c.translateProtectedLines(
				ctx,
				protectedLines,
				protectedValues,
			)
			if err != nil {
				if !isTranslationValidationError(err) {
					return nil, err
				}

				fmt.Printf(
					"Warning: the grouped translation of %d lines "+
						"failed validation; the lines will be translated "+
						"individually. Reason: %v\n",
					len(lines),
					err,
				)

				fallbackTranslations := make(
					[]string,
					len(lines),
				)

				for index, line := range lines {
					if err := ctx.Err(); err != nil {
						return nil, err
					}

					translation, lineErr := c.translate(
						ctx,
						line,
						false,
					)
					if lineErr != nil {
						return nil, fmt.Errorf(
							"translate dialogue line %d separately: %w",
							index,
							lineErr,
						)
					}

					fallbackTranslations[index] = translation
				}

				if persistErr := c.persistDialogue(
					cacheKey,
					fallbackTranslations,
				); persistErr != nil {
					return nil, fmt.Errorf(
						"persist fallback dialogue translation: %w",
						persistErr,
					)
				}

				c.storeDialogueCacheEntry(
					cacheKey,
					fallbackTranslations,
				)

				return copyStrings(
					fallbackTranslations,
				), nil
			}

			if err := ctx.Err(); err != nil {
				return nil, err
			}

			if err := c.persistDialogue(
				cacheKey,
				restoredTranslations,
			); err != nil {
				return nil, fmt.Errorf(
					"persist dialogue translation: %w",
					err,
				)
			}

			c.storeDialogueCacheEntry(
				cacheKey,
				restoredTranslations,
			)

			c.recordModelTranslatedTexts(
				len(lines),
			)

			return copyStrings(
				restoredTranslations,
			), nil
		},
	)
	if err != nil {
		return nil, err
	}

	translations, ok := value.([]string)
	if !ok {
		return nil, fmt.Errorf(
			"invalid dialogue translation result type",
		)
	}

	return copyStrings(
		translations,
	), nil
}

func (c *Client) translateProtectedText(
	ctx context.Context,
	protected protectedText,
) (string, error) {
	var lastValidationErr error

	for attempt := 1; attempt <= maxTranslationValidationAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}

		prompt := c.singleTranslationPrompt

		if attempt > 1 {
			prompt = validationRetryPrompt(
				c.singleTranslationPrompt,
				lastValidationErr,
				false,
			)
		}

		var result translationResponse

		if err := c.chat(
			ctx,
			prompt,
			protected.Text,
			responseFormat,
			singleTranslationNumPredict,
			translationNumCtx,
			&result,
		); err != nil {
			if isStructuredResponseError(err) {
				return "", &translationValidationError{
					attempts: maxRequestAttempts,
					err:      err,
				}
			}

			return "", err
		}

		restoredTranslation, validationErr := validateTranslation(
			result.Translation,
			protected,
		)
		if validationErr == nil {
			return restoredTranslation, nil
		}

		lastValidationErr = validationErr

		if attempt < maxTranslationValidationAttempts {
			c.recordRetryAttempt()

			if err := waitForRetry(
				ctx,
				translationValidationRetryDelay,
			); err != nil {
				return "", err
			}
		}
	}

	return "", &translationValidationError{
		attempts: maxTranslationValidationAttempts,
		err:      lastValidationErr,
	}
}

func (c *Client) translateProtectedBatch(
	ctx context.Context,
	protectedLines []string,
	protectedValues []protectedText,
) ([]string, error) {
	return c.translateProtectedMultiple(
		ctx,
		c.batchTranslationPrompt,
		protectedLines,
		protectedValues,
		translationLinesResponseFormat,
		batchTranslationNumPredict,
		translationNumCtx,
	)
}

func (c *Client) translateProtectedLines(
	ctx context.Context,
	protectedLines []string,
	protectedValues []protectedText,
) ([]string, error) {
	return c.translateProtectedMultiple(
		ctx,
		c.dialogueTranslationPrompt,
		protectedLines,
		protectedValues,
		dialogueLinesResponseFormat,
		dialogueTranslationNumPredict,
		translationNumCtx,
	)
}

func (c *Client) translateProtectedMultiple(
	ctx context.Context,
	basePrompt string,
	protectedLines []string,
	protectedValues []protectedText,
	responseSchema json.RawMessage,
	numPredict int,
	numCtx int,
) ([]string, error) {
	protectedRequestBody, err := json.Marshal(
		translationLinesRequest{
			Lines: protectedLines,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal protected translation elements: %w",
			err,
		)
	}

	var lastValidationErr error

	for attempt := 1; attempt <= maxTranslationValidationAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		prompt := basePrompt

		if attempt > 1 {
			prompt = validationRetryPrompt(
				basePrompt,
				lastValidationErr,
				true,
			)
		}

		var result translationLinesResponse

		if err := c.chat(
			ctx,
			prompt,
			string(protectedRequestBody),
			responseSchema,
			numPredict,
			numCtx,
			&result,
		); err != nil {
			if isStructuredResponseError(err) {
				return nil, &translationValidationError{
					attempts: maxRequestAttempts,
					err:      err,
				}
			}

			return nil, err
		}

		restoredTranslations, validationErr := validateTranslations(
			result.Translations,
			protectedValues,
		)
		if validationErr == nil {
			return restoredTranslations, nil
		}

		lastValidationErr = validationErr

		if attempt < maxTranslationValidationAttempts {
			c.recordRetryAttempt()

			if err := waitForRetry(
				ctx,
				translationValidationRetryDelay,
			); err != nil {
				return nil, err
			}
		}
	}

	return nil, &translationValidationError{
		attempts: maxTranslationValidationAttempts,
		err:      lastValidationErr,
	}
}

func validateTranslation(
	translation string,
	protected protectedText,
) (string, error) {
	translation = strings.TrimSpace(
		translation,
	)

	if translation == "" {
		return "", fmt.Errorf(
			"Ollama returned empty translation",
		)
	}

	restoredTranslation, err := protected.restore(
		translation,
	)
	if err != nil {
		return "", fmt.Errorf(
			"restore protected translation: %w",
			err,
		)
	}

	restoredTranslation = strings.TrimSpace(
		restoredTranslation,
	)

	if restoredTranslation == "" {
		return "", fmt.Errorf(
			"Ollama returned empty translation after restoring protected values",
		)
	}

	return restoredTranslation, nil
}

func validateTranslations(
	translations []string,
	protectedValues []protectedText,
) ([]string, error) {
	if len(translations) != len(protectedValues) {
		return nil, fmt.Errorf(
			"Ollama returned %d translation elements, expected %d",
			len(translations),
			len(protectedValues),
		)
	}

	cleanedTranslations := copyStrings(
		translations,
	)

	for index := range cleanedTranslations {
		cleanedTranslations[index] = strings.TrimSpace(
			cleanedTranslations[index],
		)

		if cleanedTranslations[index] == "" {
			return nil, fmt.Errorf(
				"Ollama returned empty translation element at index %d",
				index,
			)
		}
	}

	restoredTranslations, err := restoreLines(
		cleanedTranslations,
		protectedValues,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"restore protected translation elements: %w",
			err,
		)
	}

	for index := range restoredTranslations {
		restoredTranslations[index] = strings.TrimSpace(
			restoredTranslations[index],
		)

		if restoredTranslations[index] == "" {
			return nil, fmt.Errorf(
				"translation element %d is empty after restoring protected values",
				index,
			)
		}
	}

	return restoredTranslations, nil
}

func validationRetryPrompt(
	basePrompt string,
	validationErr error,
	multipleLines bool,
) string {
	lineRule := "Keep every protected marker in the translated string."

	if multipleLines {
		lineRule = "Keep every protected marker in the same array element in which it appeared."
	}

	return fmt.Sprintf(
		`%s

CRITICAL RETRY INSTRUCTION:
The previous response failed automatic validation: %v
Correct the response instead of repeating the same mistake.
- Copy every protected marker such as ⟦RMLOC_0000⟧ exactly, character for character.
- Every protected marker must occur exactly once.
- Do not translate, remove, duplicate, rename, reorder, or alter protected markers.
- %s
- Return only the JSON object required above.`,
		basePrompt,
		validationErr,
		lineRule,
	)
}

func truncateForLog(
	text string,
	maxRunes int,
) string {
	if maxRunes <= 0 {
		return ""
	}

	runes := []rune(text)

	if len(runes) <= maxRunes {
		return text
	}

	return string(
		runes[:maxRunes],
	) + "…"
}

func (c *Client) chat(
	ctx context.Context,
	system string,
	user string,
	format json.RawMessage,
	numPredict int,
	numCtx int,
	result any,
) error {
	ctx = normalizeContext(ctx)

	if err := ctx.Err(); err != nil {
		return err
	}

	body, err := json.Marshal(
		chatRequest{
			Model:     c.model,
			Stream:    false,
			Think:     false,
			KeepAlive: "10m",
			Format:    format,
			Options: chatOptions{
				Temperature: 0,
				NumPredict:  numPredict,
				NumCtx:      numCtx,
			},
			Messages: []message{
				{
					Role:    "system",
					Content: system,
				},
				{
					Role:    "user",
					Content: user,
				},
			},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"marshal request: %w",
			err,
		)
	}

	return c.retryOperation(
		ctx,
		func(
			ctx context.Context,
		) error {
			return c.executeChatRequest(
				ctx,
				body,
				result,
			)
		},
	)
}

func (c *Client) executeChatRequest(
	ctx context.Context,
	body []byte,
	result any,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.recordHTTPAttempt()

	startedAt := time.Now()

	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	defer fasthttp.ReleaseRequest(
		request,
	)

	defer fasthttp.ReleaseResponse(
		response,
	)

	request.Header.SetMethod(
		fasthttp.MethodPost,
	)

	request.Header.SetContentType(
		"application/json",
	)

	request.SetRequestURI(
		c.baseURL + "/api/chat",
	)

	request.SetBody(body)

	requestErr := c.httpClient.DoTimeout(
		request,
		response,
		c.timeout,
	)

	if ctxErr := ctx.Err(); ctxErr != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return ctxErr
	}

	if requestErr != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			fmt.Errorf(
				"request Ollama: %w",
				requestErr,
			),
		)
	}

	if response.StatusCode() != fasthttp.StatusOK {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		statusErr := fmt.Errorf(
			"Ollama returned status %d: %s",
			response.StatusCode(),
			strings.TrimSpace(
				string(
					response.Body(),
				),
			),
		)

		if isRetryableStatus(
			response.StatusCode(),
		) {
			return markRetryable(
				statusErr,
			)
		}

		return statusErr
	}

	var chatResult chatResponse

	if err := json.Unmarshal(
		response.Body(),
		&chatResult,
	); err != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			fmt.Errorf(
				"unmarshal Ollama response: %w",
				err,
			),
		)
	}

	if chatResult.Error != "" {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return fmt.Errorf(
			"Ollama error: %s",
			chatResult.Error,
		)
	}

	if strings.TrimSpace(
		chatResult.Message.Content,
	) == "" {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			fmt.Errorf(
				"Ollama returned empty message content",
			),
		)
	}

	if err := json.Unmarshal(
		[]byte(
			chatResult.Message.Content,
		),
		result,
	); err != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			&structuredResponseError{
				content: truncateForLog(
					chatResult.Message.Content,
					300,
				),
				err: err,
			},
		)
	}

	c.recordSuccessfulHTTPResponse(
		chatResult,
		time.Since(startedAt),
	)

	return nil
}

func (c *Client) doSingleflightContext(
	ctx context.Context,
	key string,
	operation func() (any, error),
) (
	any,
	error,
	bool,
) {
	ctx = normalizeContext(ctx)

	resultChannel := c.requestGroup.DoChan(
		key,
		operation,
	)

	select {
	case <-ctx.Done():
		return nil, ctx.Err(), false

	case result := <-resultChannel:
		return result.Val, result.Err, result.Shared
	}
}

func normalizeContext(
	ctx context.Context,
) context.Context {
	if ctx == nil {
		return context.Background()
	}

	return ctx
}

func (e *structuredResponseError) Error() string {
	return fmt.Sprintf(
		"unmarshal structured response %q: %v",
		e.content,
		e.err,
	)
}

func (e *structuredResponseError) Unwrap() error {
	return e.err
}

func isStructuredResponseError(
	err error,
) bool {
	var target *structuredResponseError

	return errors.As(
		err,
		&target,
	)
}

func copyStrings(
	values []string,
) []string {
	result := make(
		[]string,
		len(values),
	)

	copy(
		result,
		values,
	)

	return result
}

func (e *translationValidationError) Error() string {
	return fmt.Sprintf(
		"translation validation failed after %d attempts: %v",
		e.attempts,
		e.err,
	)
}

func (e *translationValidationError) Unwrap() error {
	return e.err
}

func isTranslationValidationError(
	err error,
) bool {
	var target *translationValidationError

	return errors.As(
		err,
		&target,
	)
}
