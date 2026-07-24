package rmmz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"rmloc/internal/engine/rmmz/items"
	"strings"
	"time"
)

func (a *RmmzEngine) ProcessItems(
	gameDir string,
) error {
	itemsPath, err := findItemsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(itemsPath)
	if err != nil {
		return fmt.Errorf("read Items.json: %w", err)
	}

	fileInfo, err := os.Stat(itemsPath)
	if err != nil {
		return fmt.Errorf("stat Items.json: %w", err)
	}

	texts, err := items.Extract(itemsPath)
	if err != nil {
		return fmt.Errorf("extract Items.json: %w", err)
	}

	fmt.Printf("Найден файл: %s\n", itemsPath)
	fmt.Printf("Найдено строк: %d\n", len(texts))

	cache := make(map[string]string)
	for index := range texts {
		text := &texts[index]
		fmt.Printf(
			"[%d/%d] item=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.ItemID,
			text.Field,
			text.Text,
		)

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate item %d field %s: %w",
					text.ItemID,
					text.Field,
					err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation for item %d field %s",
					text.ItemID,
					text.Field,
				)
			}
			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applyItemTranslations(
		sourceData,
		texts,
	)
	if err != nil {
		return err
	}

	tempPath := itemsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf("write temporary Items.json: %w", err)
	}

	defer os.Remove(tempPath)
	oldDir := filepath.Join(
		filepath.Dir(itemsPath),
		"old",
	)

	if err := os.MkdirAll(oldDir, 0755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Items_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(
		oldDir,
		backupName,
	)

	if err := os.Rename(
		itemsPath,
		backupPath,
	); err != nil {
		return fmt.Errorf("move original Items.json: %w", err)
	}

	if err := os.Rename(
		tempPath,
		itemsPath,
	); err != nil {
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

	fmt.Printf("Перевод сохранён: %s\n", itemsPath)
	fmt.Printf("Оригинал сохранён: %s\n\n", backupPath)

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

	candidates := []string{
		filepath.Join(rootDir, "data", "Items.json"),
		filepath.Join(rootDir, "www", "data", "Items.json"),
		filepath.Join(rootDir, "Items.json"),
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
			path string, entry fs.DirEntry, walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.IsDir() {
				switch entry.Name() {
				case ".git", "node_modules", "old":
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
		return nil, fmt.Errorf("decode Items.json: %w", err)
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
		return nil, fmt.Errorf("encode Items.json: %w", err)
	}

	return output.Bytes(), nil
}
