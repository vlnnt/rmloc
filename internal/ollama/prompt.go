package ollama

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type PromptConfig struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

func LoadPromptConfig(
	path string,
) (PromptConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return PromptConfig{}, fmt.Errorf("prompt path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return PromptConfig{}, fmt.Errorf("read prompt file %w", err)
	}

	var config PromptConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return PromptConfig{}, fmt.Errorf("decode prompt file: %w", err)
	}

	config.Name = strings.TrimSpace(config.Name)
	config.Prompt = strings.TrimSpace(config.Prompt)

	if err := config.validate(); err != nil {
		return PromptConfig{}, err
	}

	return config, nil
}

func (p PromptConfig) validate() error {
	if strings.TrimSpace(p.Prompt) == "" {
		return fmt.Errorf("prompt is empty")
	}

	return nil
}

func (p PromptConfig) Fingerprint() string {
	hash := sha256.Sum224([]byte(strings.TrimSpace(p.Prompt)))
	return fmt.Sprintf("%x", hash)
}

var responseFormat = json.RawMessage(`{
	"type": "object",
	"properties": {
		"translation": {
			"type": "string"
		}
	},
	"required": ["translation"],
	"additionalProperties": false
}`)

var translationLinesResponseFormat = json.RawMessage(`{
	"type": "object",
	"properties": {
		"translations": {
			"type": "array",
			"items": {
				"type": "string"
			}
		}
	},
	"required": ["translations"],
	"additionalProperties": false
}`)
