package ollama

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

const (
	singleTranslationNumPredict      = 192
	dialogueTranslationNumPredict    = 512
	translationNumCtx                = 2048
	maxTranslationValidationAttempts = 3
	translationValidationRetryDelay  = 500 * time.Millisecond
)

type translationValidationError struct {
	attempts int
	err      error
}

func NewClient(
	baseURL, model string,
) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &fasthttp.Client{
			ReadTimeout:  5 * time.Minute,
			WriteTimeout: 10 * time.Second,
		},
		timeout: 5 * time.Minute,
	}
}

func (c *Client) Translate(
	text string,
) (string, error) {
	return c.translate(text, true)
}

func (c *Client) translate(
	text string,
	recordInput bool,
) (string, error) {
	if recordInput {
		c.recordInputTexts(1)
	}

	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is empty")
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
	value, err, _ := c.requestGroup.Do(
		requestKey,
		func() (any, error) {
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
				protected,
			)
			if err != nil {
				if !isTranslationValidationError(err) {
					return nil, err
				}

				c.translationCache.Store(
					text,
					text,
				)

				c.recordSafetyFallback(1)
				fmt.Printf(
					"Предупреждение: строку не удалось безопасно перевести; "+
						"она оставлена без изменений. Текст: %q. Причина: %v\n",
					truncateForLog(text, 180),
					err,
				)

				return text, nil
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

			c.translationCache.Store(
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

func (c *Client) TranslateLines(
	lines []string,
) ([]string, error) {
	c.recordInputTexts(len(lines))
	if len(lines) == 0 {
		return nil, fmt.Errorf("lines are empty")
	}

	if !needsAnyTranslation(lines) {
		c.recordSkippedTexts(len(lines))
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

	cacheKey := string(originalRequestBody)
	if cached, exists := c.dialogueCache.Load(cacheKey); exists {
		translations, ok := cached.([]string)
		if !ok {
			return nil, fmt.Errorf(
				"invalid cached dialogue translation type",
			)
		}

		c.recordCacheHit(len(lines))
		return copyStrings(translations), nil
	}

	requestKey := "dialogue:" + cacheKey
	value, err, _ := c.requestGroup.Do(
		requestKey,
		func() (any, error) {
			if cached, exists := c.dialogueCache.Load(cacheKey); exists {
				translations, ok := cached.([]string)
				if !ok {
					return nil, fmt.Errorf(
						"invalid cached dialogue translation type",
					)
				}

				c.recordCacheHit(len(lines))
				return copyStrings(translations), nil
			}

			protectedLines, protectedValues := protectLines(
				lines,
			)

			restoredTranslations, err := c.translateProtectedLines(
				protectedLines,
				protectedValues,
			)
			if err != nil {
				if !isTranslationValidationError(err) {
					return nil, err
				}

				fmt.Printf(
					"Предупреждение: групповой перевод из %d строк "+
						"не прошёл проверку; строки будут переведены по одной. "+
						"Причина: %v\n",
					len(lines),
					err,
				)

				fallbackTranslations := make(
					[]string,
					len(lines),
				)

				for index, line := range lines {
					translation, lineErr := c.translate(
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

				c.dialogueCache.Store(
					cacheKey,
					copyStrings(fallbackTranslations),
				)

				return copyStrings(fallbackTranslations), nil
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

			c.dialogueCache.Store(
				cacheKey,
				copyStrings(restoredTranslations),
			)

			c.recordModelTranslatedTexts(len(lines))
			return copyStrings(restoredTranslations), nil
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

	return copyStrings(translations), nil
}

func (c *Client) translateProtectedText(
	protected protectedText,
) (string, error) {
	var lastValidationErr error
	for attempt := 1; attempt <= maxTranslationValidationAttempts; attempt++ {
		prompt := systemPrompt
		if attempt > 1 {
			prompt = validationRetryPrompt(
				systemPrompt,
				lastValidationErr,
				false,
			)
		}

		var result translationResponse
		if err := c.chat(
			prompt,
			protected.Text,
			responseFormat,
			singleTranslationNumPredict,
			translationNumCtx,
			&result,
		); err != nil {
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
			time.Sleep(translationValidationRetryDelay)
		}
	}

	return "", &translationValidationError{
		attempts: maxTranslationValidationAttempts,
		err:      lastValidationErr,
	}
}

func (c *Client) translateProtectedLines(
	protectedLines []string,
	protectedValues []protectedText,
) ([]string, error) {
	protectedRequestBody, err := json.Marshal(
		translationLinesRequest{
			Lines: protectedLines,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal protected dialogue lines: %w",
			err,
		)
	}

	var lastValidationErr error
	for attempt := 1; attempt <= maxTranslationValidationAttempts; attempt++ {
		prompt := dialogueLinesPrompt

		if attempt > 1 {
			prompt = validationRetryPrompt(
				dialogueLinesPrompt,
				lastValidationErr,
				true,
			)
		}

		var result translationLinesResponse
		if err := c.chat(
			prompt,
			string(protectedRequestBody),
			dialogueLinesResponseFormat,
			dialogueTranslationNumPredict,
			translationNumCtx,
			&result,
		); err != nil {
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
			time.Sleep(translationValidationRetryDelay)
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
	translation = strings.TrimSpace(translation)

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
			"Ollama returned %d dialogue lines, expected %d",
			len(translations),
			len(protectedValues),
		)
	}

	cleanedTranslations := copyStrings(translations)
	for index := range cleanedTranslations {
		cleanedTranslations[index] = strings.TrimSpace(
			cleanedTranslations[index],
		)

		if cleanedTranslations[index] == "" {
			return nil, fmt.Errorf(
				"Ollama returned empty dialogue line at index %d",
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
			"restore protected dialogue lines: %w",
			err,
		)
	}

	for index := range restoredTranslations {
		restoredTranslations[index] = strings.TrimSpace(
			restoredTranslations[index],
		)

		if restoredTranslations[index] == "" {
			return nil, fmt.Errorf(
				"dialogue line %d is empty after restoring protected values",
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

	return string(runes[:maxRunes]) + "…"
}

func (c *Client) chat(
	system, user string,
	format json.RawMessage,
	numPredict, numCtx int,
	result any,
) error {
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
		func() error {
			return c.executeChatRequest(
				body,
				result,
			)
		},
	)
}

func (c *Client) executeChatRequest(
	body []byte,
	result any,
) error {
	c.recordHTTPAttempt()

	startedAt := time.Now()
	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()

	defer fasthttp.ReleaseRequest(request)
	defer fasthttp.ReleaseResponse(response)

	request.Header.SetMethod(fasthttp.MethodPost)
	request.Header.SetContentType("application/json")
	request.SetRequestURI(c.baseURL + "/api/chat")
	request.SetBody(body)

	if err := c.httpClient.DoTimeout(
		request,
		response,
		c.timeout,
	); err != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			fmt.Errorf(
				"request Ollama: %w",
				err,
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
				string(response.Body()),
			),
		)

		if isRetryableStatus(response.StatusCode()) {
			return markRetryable(statusErr)
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
		[]byte(chatResult.Message.Content),
		result,
	); err != nil {
		c.recordFailedHTTPAttempt(
			time.Since(startedAt),
		)

		return markRetryable(
			fmt.Errorf(
				"unmarshal structured response %q: %w",
				chatResult.Message.Content,
				err,
			),
		)
	}

	c.recordSuccessfulHTTPResponse(
		chatResult,
		time.Since(startedAt),
	)

	return nil
}

func copyStrings(
	values []string,
) []string {
	result := make(
		[]string,
		len(values),
	)

	copy(result, values)
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
	return errors.As(err, &target)
}
