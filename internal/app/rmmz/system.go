package rmmz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"rmloc/internal/engine/rmmz/system"
	"strings"
	"time"
)

func (a *RmmzEngine) ProcessSystem(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	systemPath, err := findSystemFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(systemPath)
	if err != nil {
		return fmt.Errorf("read System.json: %w", err)
	}

	fileInfo, err := os.Stat(systemPath)
	if err != nil {
		return fmt.Errorf("stat System.json: %w", err)
	}

	texts, err := system.Extract(systemPath)
	if err != nil {
		return fmt.Errorf("extract System.json: %w", err)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(systemPath, len(texts))
	reporter.FileStarted(systemPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(systemPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(systemPath)

		default:
			reporter.FileFailed(systemPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", systemPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		systemPath,
		texts,
		func(text *system.SystemText) string {
			return text.Text
		},
		func(text *system.SystemText, translation string) {
			text.Text = translation
		},
		func(text *system.SystemText) string {
			return fmt.Sprintf(
				"field=%s index=%d key=%s",
				text.Field,
				text.Index,
				text.Key,
			)
		},
	); err != nil {
		return fmt.Errorf("translate System.json: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applySystemTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := systemPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary System.json: %w",
			err,
		)
	}

	defer os.Remove(tempPath)

	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(systemPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"System_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(systemPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original System.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, systemPath); err != nil {
		restoreErr := os.Rename(backupPath, systemPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace System.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace System.json: %w",
			err,
		)
	}

	fmt.Printf("Translation saved: %s\n", systemPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findSystemFile(
	gameDir string,
) (string, error) {
	rootDir, err := filepath.Abs(gameDir)
	if err != nil {
		return "", fmt.Errorf(
			"resolve game directory: %w",
			err,
		)
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		return "", fmt.Errorf(
			"open game directory: %w",
			err,
		)
	}

	if !info.IsDir() {
		return "", fmt.Errorf(
			"game path is not a directory: %s",
			rootDir,
		)
	}

	candidates := []string{
		filepath.Join(rootDir, "data", "System.json"),
		filepath.Join(rootDir, "www", "data", "System.json"),
		filepath.Join(rootDir, "System.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var systemPath string
	err = filepath.WalkDir(
		rootDir,
		func(
			path string,
			entry fs.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				switch entry.Name() {
				case ".git", "node_modules", "old", ".rmloc":
					return filepath.SkipDir
				}

				return nil
			}

			if entry.Name() != "System.json" {
				return nil
			}

			if filepath.Base(filepath.Dir(path)) != "data" {
				return nil
			}

			systemPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search System.json: %w",
			err,
		)
	}

	if systemPath == "" {
		return "", fmt.Errorf(
			"data/System.json not found inside: %s",
			rootDir,
		)
	}

	return systemPath, nil
}

func applySystemTranslations(
	sourceData []byte,
	texts []system.SystemText,
) ([]byte, error) {
	var record map[string]any
	if err := json.Unmarshal(sourceData, &record); err != nil {
		return nil, fmt.Errorf(
			"decode System.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.Field == "gameTitle" {
			record["gameTitle"] = text.Text
			continue
		}

		if text.Field == "terms.messages" {
			terms, ok := record["terms"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("terms is not an object")
			}

			messages, ok := terms["messages"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf(
					"terms.messages is not an object",
				)
			}

			if _, exists := messages[text.Key]; !exists {
				return nil, fmt.Errorf(
					"message key not found: %s",
					text.Key,
				)
			}

			messages[text.Key] = text.Text
			continue
		}

		container := record
		field := text.Field

		if strings.HasPrefix(field, "terms.") {
			terms, ok := record["terms"].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("terms is not an object")
			}

			container = terms
			field = strings.TrimPrefix(field, "terms.")
		}

		values, ok := container[field].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"%s is not an array",
				text.Field,
			)
		}

		if text.Index < 0 || text.Index >= len(values) {
			return nil, fmt.Errorf(
				"index %d is out of range for %s",
				text.Index,
				text.Field,
			)
		}

		values[text.Index] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(record); err != nil {
		return nil, fmt.Errorf(
			"encode System.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
