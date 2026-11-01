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
	"strings"
	"time"

	"rmloc/internal/engine/rmmz/enemies"
)

func (a *RmmzEngine) ProcessEnemies(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	enemiesPath, err := findEnemiesFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(enemiesPath)
	if err != nil {
		return fmt.Errorf(
			"read Enemies.json: %w",
			err,
		)
	}

	fileInfo, err := os.Stat(enemiesPath)
	if err != nil {
		return fmt.Errorf(
			"stat Enemies.json: %w",
			err,
		)
	}

	texts, err := enemies.Extract(enemiesPath)
	if err != nil {
		return fmt.Errorf(
			"extract Enemies.json: %w",
			err,
		)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(enemiesPath, len(texts))
	reporter.FileStarted(enemiesPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(enemiesPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(enemiesPath)

		default:
			reporter.FileFailed(enemiesPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", enemiesPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		enemiesPath,
		texts,
		func(text *enemies.EnemyText) string {
			return text.Text
		},
		func(text *enemies.EnemyText, translation string) {
			text.Text = translation
		},
		func(text *enemies.EnemyText) string {
			return fmt.Sprintf(
				"enemy=%d field=%s",
				text.EnemyID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf("translate Enemies.json: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyEnemyTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := enemiesPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Enemies.json: %w",
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(enemiesPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	backupName := fmt.Sprintf(
		"Enemies_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(enemiesPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original Enemies.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, enemiesPath); err != nil {
		restoreErr := os.Rename(backupPath, enemiesPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Enemies.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace Enemies.json: %w",
			err,
		)
	}

	fmt.Printf("Translation saved: %s\n", enemiesPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findEnemiesFile(
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
		filepath.Join(
			rootDir,
			"data",
			"Enemies.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Enemies.json",
		),
		filepath.Join(
			rootDir,
			"Enemies.json",
		),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var enemiesPath string
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
				case ".git",
					"node_modules",
					"old",
					".rmloc":
					return filepath.SkipDir
				}

				return nil
			}

			if entry.Name() != "Enemies.json" {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			enemiesPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Enemies.json: %w",
			err,
		)
	}

	if enemiesPath == "" {
		return "", fmt.Errorf(
			"data/Enemies.json not found inside: %s",
			rootDir,
		)
	}

	return enemiesPath, nil
}

func applyEnemyTranslations(
	sourceData []byte,
	texts []enemies.EnemyText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf(
			"decode Enemies.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.EnemyIndex < 0 ||
			text.EnemyIndex >= len(records) {
			return nil, fmt.Errorf(
				"enemy index is out of range: %d",
				text.EnemyIndex,
			)
		}

		record := records[text.EnemyIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"enemy at index %d is null",
				text.EnemyIndex,
			)
		}

		record[text.Field] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf(
			"encode Enemies.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
