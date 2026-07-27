package ollama

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const legacyTranslationCacheVersion = 1

const legacyDefaultPromptHash = "db1e77c67289a7e530646eef5310e10f3be21696710b2a73032302385ef39124"

type PromptConfig struct {
	Name                string
	SingleTranslation   string
	DialogueTranslation string
}

type promptConfigFile struct {
	Name                string          `json:"name"`
	SingleTranslation   json.RawMessage `json:"single_translation"`
	DialogueTranslation json.RawMessage `json:"dialogue_translation"`
}

func LoadPromptConfig(
	path string,
) (PromptConfig, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return PromptConfig{}, fmt.Errorf(
			"prompt path is empty",
		)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return PromptConfig{}, fmt.Errorf(
			"resolve prompt path: %w",
			err,
		)
	}

	data, err := os.ReadFile(absolutePath)
	if err != nil {
		return PromptConfig{}, fmt.Errorf(
			"read prompt file %s: %w",
			absolutePath,
			err,
		)
	}

	var file promptConfigFile
	if err := json.Unmarshal(
		data,
		&file,
	); err != nil {
		return PromptConfig{}, fmt.Errorf(
			"decode prompt file %s: %w",
			absolutePath,
			err,
		)
	}

	singleTranslation, err := decodePromptText(
		file.SingleTranslation,
		"single_translation",
	)
	if err != nil {
		return PromptConfig{}, fmt.Errorf(
			"decode prompt file %s: %w",
			absolutePath,
			err,
		)
	}

	dialogueTranslation, err := decodePromptText(
		file.DialogueTranslation,
		"dialogue_translation",
	)
	if err != nil {
		return PromptConfig{}, fmt.Errorf(
			"decode prompt file %s: %w",
			absolutePath,
			err,
		)
	}

	config := PromptConfig{
		Name: strings.TrimSpace(
			file.Name,
		),
		SingleTranslation:   singleTranslation,
		DialogueTranslation: dialogueTranslation,
	}

	if err := config.validate(); err != nil {
		return PromptConfig{}, fmt.Errorf(
			"validate prompt file %s: %w",
			absolutePath,
			err,
		)
	}

	return config, nil
}

func (p PromptConfig) Fingerprint() string {
	hash := sha256.Sum256(
		[]byte(
			strings.TrimSpace(
				p.SingleTranslation,
			) +
				"\x00" +
				strings.TrimSpace(
					p.DialogueTranslation,
				),
		),
	)

	return hex.EncodeToString(hash[:])
}

func (p PromptConfig) validate() error {
	if strings.TrimSpace(
		p.SingleTranslation,
	) == "" {
		return fmt.Errorf(
			"single_translation is empty",
		)
	}

	if strings.TrimSpace(
		p.DialogueTranslation,
	) == "" {
		return fmt.Errorf(
			"dialogue_translation is empty",
		)
	}

	return nil
}

func decodePromptText(
	raw json.RawMessage,
	field string,
) (string, error) {
	if len(raw) == 0 ||
		string(raw) == "null" {
		return "", fmt.Errorf(
			"%s is missing",
			field,
		)
	}

	var text string
	if err := json.Unmarshal(
		raw,
		&text,
	); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return "", fmt.Errorf(
				"%s is empty",
				field,
			)
		}

		return text, nil
	}

	var lines []string
	if err := json.Unmarshal(
		raw,
		&lines,
	); err != nil {
		return "", fmt.Errorf(
			"%s must be either a string or an array of strings",
			field,
		)
	}

	for index := range lines {
		lines[index] = strings.TrimRight(
			lines[index],
			" \t",
		)
	}

	text = strings.TrimSpace(
		strings.Join(
			lines,
			"\n",
		),
	)

	if text == "" {
		return "", fmt.Errorf(
			"%s is empty",
			field,
		)
	}

	return text, nil
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

var dialogueLinesResponseFormat = json.RawMessage(`{
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
