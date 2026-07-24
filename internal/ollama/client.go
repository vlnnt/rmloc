package ollama

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

func NewClient(
	baseURL, model string,
) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		httpClient: &fasthttp.Client{
			ReadTimeout:  2 * time.Minute,
			WriteTimeout: 10 * time.Second,
		},
		timeout: 2 * time.Minute,
	}
}

func (c *Client) Translate(
	text string,
) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("text is empty")
	}

	body, err := json.Marshal(chatRequest{
		Model:     c.model,
		Stream:    false,
		Think:     false,
		KeepAlive: "10m",
		Format:    responseFormat,
		Options: chatOptions{
			Temperature: 0,
			NumPredict:  64,
		},
		Messages: []message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: text,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	request := fasthttp.AcquireRequest()
	response := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(request)
	defer fasthttp.ReleaseResponse(response)

	request.Header.SetMethod(fasthttp.MethodPost)
	request.Header.SetContentType("application/json")
	request.SetRequestURI(c.baseURL + "/api/chat")
	request.SetBody(body)

	if err := c.httpClient.DoTimeout(request, response, c.timeout); err != nil {
		return "", fmt.Errorf("request Ollama: %w", err)
	}

	if response.StatusCode() != fasthttp.StatusOK {
		return "", fmt.Errorf(
			"Ollama returned status %d: %s",
			response.StatusCode(),
			strings.TrimSpace(string(response.Body())),
		)
	}

	var chatResult chatResponse
	if err := json.Unmarshal(response.Body(), &chatResult); err != nil {
		return "", fmt.Errorf("unmarshal Ollama response: %w", err)
	}

	if chatResult.Error != "" {
		return "", fmt.Errorf("Ollama error: %s", chatResult.Error)
	}

	var result translationResponse
	if err := json.Unmarshal(
		[]byte(chatResult.Message.Content), &result,
	); err != nil {
		return "", fmt.Errorf(
			"unmarshal translation %q: %w",
			chatResult.Message.Content,
			err,
		)
	}

	result.Translation = strings.TrimSpace(result.Translation)
	if result.Translation == "" {
		return "", fmt.Errorf("Ollama returned empty translation")
	}

	return result.Translation, nil
}
