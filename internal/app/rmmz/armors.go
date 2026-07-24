package rmmz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rmloc/internal/engine/rmmz/armors"
)

func (a *RmmzEngine) ProcessArmors(
	gameDir string,
) error {
	armorsPath, err := findArmorsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(armorsPath)
	if err != nil {
		return fmt.Errorf("read Armors.json: %w", err)
	}

	fileInfo, err := os.Stat(armorsPath)
	if err != nil {
		return fmt.Errorf("stat Armors.json: %w", err)
	}

	texts, err := armors.Extract(armorsPath)
	if err != nil {
		return fmt.Errorf("extract Armors.json: %w", err)
	}

	fmt.Printf("Найден файл: %s\n", armorsPath)
	fmt.Printf("Найдено строк: %d\n", len(texts))

	cache := make(map[string]string)
	for index := range texts {
		text := &texts[index]
		fmt.Printf(
			"[%d/%d] armor=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.ArmorID,
			text.Field,
			text.Text,
		)

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate armor %d field %s: %w", text.ArmorID, text.Field, err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation for armor %d field %s",
					text.ArmorID,
					text.Field,
				)
			}
			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applyArmorTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	tempPath := armorsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf("write temporary Armors.json: %w", err)
	}
	defer os.Remove(tempPath)

	oldDir := filepath.Join(filepath.Dir(armorsPath), "old")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Armors_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := os.Rename(armorsPath, backupPath); err != nil {
		return fmt.Errorf("move original Armors.json: %w", err)
	}

	if err := os.Rename(tempPath, armorsPath); err != nil {
		restoreErr := os.Rename(backupPath, armorsPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Armors.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}
		return fmt.Errorf("replace Armors.json: %w", err)
	}

	fmt.Printf("Перевод сохранён: %s\n", armorsPath)
	fmt.Printf("Оригинал сохранён: %s\n", backupPath)

	return nil
}

func findArmorsFile(
	gameDir string,
) (string, error) {
	rootDir, err := filepath.Abs(gameDir)
	if err != nil {
		return "", fmt.Errorf("resolve game directory: %w", err)
	}

	candidates := []string{
		filepath.Join(rootDir, "data", "Armors.json"),
		filepath.Join(rootDir, "www", "data", "Armors.json"),
		filepath.Join(rootDir, "Armors.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var armorsPath string
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

			if entry.Name() != "Armors.json" {
				return nil
			}

			if filepath.Base(filepath.Dir(path)) != "data" {
				return nil
			}

			armorsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf("search Armors.json: %w", err)
	}

	if armorsPath == "" {
		return "", fmt.Errorf(
			"data/Armors.json not found inside: %s",
			rootDir,
		)
	}

	return armorsPath, nil
}

func applyArmorTranslations(
	sourceData []byte,
	texts []armors.ArmorText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf("decode Armors.json: %w", err)
	}

	for _, text := range texts {
		if text.ArmorIndex < 0 || text.ArmorIndex >= len(records) {
			return nil, fmt.Errorf(
				"armor index is out of range: %d",
				text.ArmorIndex,
			)
		}

		record := records[text.ArmorIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"armor at index %d is null",
				text.ArmorIndex,
			)
		}
		record[text.Field] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf("encode Armors.json: %w", err)
	}

	return output.Bytes(), nil
}
