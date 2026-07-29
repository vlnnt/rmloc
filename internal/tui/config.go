package tui

import (
	"fmt"
	"strings"
)

type Config struct {
	GameDir     string
	OllamaURL   string
	Model       string
	PromptPath  string
	PromptName  string
	CachePath   string
	WorkerCount int
}

func (c Config) Validate() error {
	if strings.TrimSpace(
		c.GameDir,
	) == "" {
		return fmt.Errorf(
			"game directory is empty",
		)
	}

	if strings.TrimSpace(
		c.OllamaURL,
	) == "" {
		return fmt.Errorf(
			"Ollama URL is empty",
		)
	}

	if strings.TrimSpace(
		c.Model,
	) == "" {
		return fmt.Errorf(
			"Ollama model is empty",
		)
	}

	if strings.TrimSpace(
		c.PromptPath,
	) == "" {
		return fmt.Errorf(
			"prompt path is empty",
		)
	}

	if strings.TrimSpace(
		c.CachePath,
	) == "" {
		return fmt.Errorf(
			"translation cache path is empty",
		)
	}

	if c.WorkerCount <= 0 {
		return fmt.Errorf(
			"worker count must be greater than zero",
		)
	}

	return nil
}

func (c Config) Normalized() Config {
	c.GameDir = strings.TrimSpace(
		c.GameDir,
	)

	c.OllamaURL = strings.TrimSpace(
		c.OllamaURL,
	)

	c.Model = strings.TrimSpace(
		c.Model,
	)

	c.PromptPath = strings.TrimSpace(
		c.PromptPath,
	)

	c.PromptName = strings.TrimSpace(
		c.PromptName,
	)

	c.CachePath = strings.TrimSpace(
		c.CachePath,
	)

	return c
}
