package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	GameDir     string `json:"gameDir"`
	OllamaURL   string `json:"ollamaUrl"`
	Model       string `json:"model"`
	PromptPath  string `json:"promptPath"`
	PromptName  string `json:"promptName"`
	CachePath   string `json:"cachePath"`
	WorkerCount int    `json:"workerCount"`
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.GameDir) == "" {
		return fmt.Errorf("game directory is empty")
	}

	if strings.TrimSpace(c.OllamaURL) == "" {
		return fmt.Errorf("ollama URL is empty")
	}

	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("ollama model is empty")
	}

	if strings.TrimSpace(c.PromptPath) == "" {
		return fmt.Errorf("prompt path is empty")
	}

	if strings.TrimSpace(c.CachePath) == "" {
		return fmt.Errorf("translation cache path is empty")
	}

	if c.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be greater than zero")
	}

	return nil
}

func (c Config) Normalized() Config {
	c.GameDir = strings.TrimSpace(c.GameDir)
	c.OllamaURL = strings.TrimSpace(c.OllamaURL)
	c.Model = strings.TrimSpace(c.Model)
	c.PromptPath = strings.TrimSpace(c.PromptPath)
	c.PromptName = strings.TrimSpace(c.PromptName)
	c.CachePath = strings.TrimSpace(c.CachePath)

	return c
}

func configFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf(
			"get user config directory: %w",
			err,
		)
	}

	return filepath.Join(
		configDir,
		"rmloc",
		"config.json",
	), nil
}

func LoadConfig() (Config, error) {
	path, err := configFilePath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}

	if err != nil {
		return Config{}, fmt.Errorf(
			"read config file: %w",
			err,
		)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf(
			"decode config file: %w",
			err,
		)
	}

	return config.Normalized(), nil
}

func SaveConfig(
	config Config,
) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf(
			"create config directory: %w",
			err,
		)
	}

	data, err := json.MarshalIndent(
		config.Normalized(),
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"encode config: %w",
			err,
		)
	}

	tempFile, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf(
			"create temporary config: %w",
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempFile.Name())
	}()

	if err := tempFile.Chmod(0o600); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf(
			"set config permissions: %w",
			err,
		)
	}

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf(
			"write temporary config: %w",
			err,
		)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf(
			"close temporary config: %w",
			err,
		)
	}

	if err := os.Rename(tempFile.Name(), path); err != nil {
		return fmt.Errorf(
			"replace config file: %w",
			err,
		)
	}

	return nil
}
