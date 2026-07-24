package rmmz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"rmloc/internal/engine/rmmz/weapons"
	"strings"
	"time"
)

func (a *RmmzEngine) ProcessWeapons(
	gameDir string,
) error {
	weaponsPath, err := findWeaponsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(weaponsPath)
	if err != nil {
		return fmt.Errorf("read Weapons.json: %w", err)
	}

	fileInfo, err := os.Stat(weaponsPath)
	if err != nil {
		return fmt.Errorf("stat Weapons.json: %w", err)
	}

	texts, err := weapons.Extract(weaponsPath)
	if err != nil {
		return fmt.Errorf("extract Weapons.json: %w", err)
	}

	fmt.Printf("Найден файл: %s\n", weaponsPath)
	fmt.Printf("Найдено строк: %d\n", len(texts))

	cache := make(map[string]string)
	for index := range texts {
		text := &texts[index]

		fmt.Printf(
			"[%d/%d] weapon=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.WeaponID,
			text.Field,
			text.Text,
		)

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate weapon %d field %s: %w",
					text.WeaponID,
					text.Field,
					err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation for weapon %d field %s",
					text.WeaponID,
					text.Field,
				)
			}
			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applyWeaponTranslations(
		sourceData,
		texts,
	)
	if err != nil {
		return err
	}

	tempPath := weaponsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf("write temporary Weapons.json: %w", err)
	}

	defer os.Remove(tempPath)

	oldDir := filepath.Join(
		filepath.Dir(weaponsPath),
		"old",
	)

	if err := os.MkdirAll(oldDir, 0755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Weapons_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(
		oldDir,
		backupName,
	)

	if err := os.Rename(
		weaponsPath,
		backupPath,
	); err != nil {
		return fmt.Errorf("move original Weapons.json: %w", err)
	}

	if err := os.Rename(
		tempPath,
		weaponsPath,
	); err != nil {
		restoreErr := os.Rename(
			backupPath,
			weaponsPath,
		)

		if restoreErr != nil {
			return fmt.Errorf(
				"replace Weapons.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace Weapons.json: %w", err)
	}

	fmt.Printf("Перевод сохранён: %s\n", weaponsPath)
	fmt.Printf("Оригинал сохранён: %s\n\n", backupPath)

	return nil
}

func findWeaponsFile(
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
		filepath.Join(rootDir, "data", "Weapons.json"),
		filepath.Join(rootDir, "www", "data", "Weapons.json"),
		filepath.Join(rootDir, "Weapons.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var weaponsPath string
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

			if entry.Name() != "Weapons.json" {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			weaponsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Weapons.json: %w",
			err,
		)
	}

	if weaponsPath == "" {
		return "", fmt.Errorf(
			"data/Weapons.json not found inside: %s",
			rootDir,
		)
	}

	return weaponsPath, nil
}

func applyWeaponTranslations(
	sourceData []byte,
	texts []weapons.WeaponText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf("decode Weapons.json: %w", err)
	}

	for _, text := range texts {
		if text.WeaponIndex < 0 ||
			text.WeaponIndex >= len(records) {
			return nil, fmt.Errorf(
				"weapon index is out of range: %d",
				text.WeaponIndex,
			)
		}

		record := records[text.WeaponIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"weapon at index %d is null",
				text.WeaponIndex,
			)
		}
		record[text.Field] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf("encode Weapons.json: %w", err)
	}

	return output.Bytes(), nil
}
