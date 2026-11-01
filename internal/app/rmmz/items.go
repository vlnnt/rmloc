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

	"rmloc/internal/engine/rmmz/items"
)

func (a *RmmzEngine) ProcessItems(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	itemsPath, err := findItemsFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(itemsPath)
	if err != nil {
		return fmt.Errorf(
			"read Items.json: %w",
			err,
		)
	}

	fileInfo, err := os.Stat(itemsPath)
	if err != nil {
		return fmt.Errorf(
			"stat Items.json: %w",
			err,
		)
	}

	texts, err := items.Extract(itemsPath)
	if err != nil {
		return fmt.Errorf(
			"extract Items.json: %w",
			err,
		)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(itemsPath, len(texts))
	reporter.FileStarted(itemsPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(itemsPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(itemsPath)

		default:
			reporter.FileFailed(itemsPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", itemsPath)

	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		itemsPath,
		texts,
		func(text *items.ItemText) string {
			return text.Text
		},
		func(text *items.ItemText, translation string,
		) {
			text.Text = translation
		},
		func(text *items.ItemText) string {
			return fmt.Sprintf(
				"item=%d field=%s",
				text.ItemID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf(
			"translate Items.json: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyItemTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := itemsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Items.json: %w",
			err,
		)
	}

	defer os.Remove(tempPath)
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(itemsPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	backupName := fmt.Sprintf(
		"Items_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(itemsPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original Items.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, itemsPath); err != nil {
		restoreErr := os.Rename(
			backupPath,
			itemsPath,
		)

		if restoreErr != nil {
			return fmt.Errorf(
				"replace Items.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace Items.json: %w", err)
	}

	fmt.Printf("Translation saved: %s\n", itemsPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findItemsFile(
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
			"Items.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Items.json",
		),
		filepath.Join(
			rootDir,
			"Items.json",
		),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var itemsPath string
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

			if entry.Name() != "Items.json" {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			itemsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Items.json: %w",
			err,
		)
	}

	if itemsPath == "" {
		return "", fmt.Errorf(
			"data/Items.json not found inside: %s",
			rootDir,
		)
	}

	return itemsPath, nil
}

func applyItemTranslations(
	sourceData []byte,
	texts []items.ItemText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf(
			"decode Items.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.ItemIndex < 0 ||
			text.ItemIndex >= len(records) {
			return nil, fmt.Errorf(
				"item index is out of range: %d",
				text.ItemIndex,
			)
		}

		record := records[text.ItemIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"item at index %d is null",
				text.ItemIndex,
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
			"encode Items.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
