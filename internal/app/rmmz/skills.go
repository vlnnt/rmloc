package rmmz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"rmloc/internal/engine/rmmz/skills"
	"strings"
	"time"
)

func (a *RmmzEngine) ProcessSkills(
	gameDir string,
) error {
	skillsPath, err := findSkillsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(skillsPath)
	if err != nil {
		return fmt.Errorf("read Skills.json: %w", err)
	}

	fileInfo, err := os.Stat(skillsPath)
	if err != nil {
		return fmt.Errorf("stat Skills.json: %w", err)
	}

	texts, err := skills.Extract(skillsPath)
	if err != nil {
		return fmt.Errorf("extract Skills.json: %w", err)
	}

	fmt.Printf("Найден файл: %s\n", skillsPath)
	fmt.Printf("Найдено строк: %d\n", len(texts))

	cache := make(map[string]string)
	for index := range texts {
		text := &texts[index]
		fmt.Printf(
			"[%d/%d] skill=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.SkillID,
			text.Field,
			text.Text,
		)

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate skill %d field %s: %w",
					text.SkillID,
					text.Field,
					err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation for skill %d field %s",
					text.SkillID,
					text.Field,
				)
			}
			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applySkillTranslations(
		sourceData,
		texts,
	)
	if err != nil {
		return err
	}

	tempPath := skillsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf("write temporary Skills.json: %w", err)
	}

	defer os.Remove(tempPath)

	oldDir := filepath.Join(
		filepath.Dir(skillsPath),
		"old",
	)

	if err := os.MkdirAll(oldDir, 0755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Skills_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(
		oldDir,
		backupName,
	)

	if err := os.Rename(
		skillsPath,
		backupPath,
	); err != nil {
		return fmt.Errorf("move original Skills.json: %w", err)
	}

	if err := os.Rename(
		tempPath,
		skillsPath,
	); err != nil {
		restoreErr := os.Rename(
			backupPath,
			skillsPath,
		)

		if restoreErr != nil {
			return fmt.Errorf(
				"replace Skills.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace Skills.json: %w", err)
	}

	fmt.Printf("Перевод сохранён: %s\n", skillsPath)
	fmt.Printf("Оригинал сохранён: %s\n\n", backupPath)

	return nil
}

func findSkillsFile(
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
		filepath.Join(rootDir, "data", "Skills.json"),
		filepath.Join(rootDir, "www", "data", "Skills.json"),
		filepath.Join(rootDir, "Skills.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var skillsPath string
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

			if entry.Name() != "Skills.json" {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			skillsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Skills.json: %w",
			err,
		)
	}

	if skillsPath == "" {
		return "", fmt.Errorf(
			"data/Skills.json not found inside: %s",
			rootDir,
		)
	}

	return skillsPath, nil
}

func applySkillTranslations(
	sourceData []byte,
	texts []skills.SkillText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf("decode Skills.json: %w", err)
	}

	for _, text := range texts {
		if text.SkillIndex < 0 ||
			text.SkillIndex >= len(records) {
			return nil, fmt.Errorf(
				"skill index is out of range: %d",
				text.SkillIndex,
			)
		}

		record := records[text.SkillIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"skill at index %d is null",
				text.SkillIndex,
			)
		}
		record[text.Field] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf("encode Skills.json: %w", err)
	}

	return output.Bytes(), nil
}
