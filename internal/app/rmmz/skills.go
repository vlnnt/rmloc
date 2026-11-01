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

	"rmloc/internal/engine/rmmz/skills"
)

func (a *RmmzEngine) ProcessSkills(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	skillsPath, err := findSkillsFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
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

	reporter := a.Reporter()
	reporter.FileDiscovered(skillsPath, len(texts))
	reporter.FileStarted(skillsPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(skillsPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(skillsPath)

		default:
			reporter.FileFailed(skillsPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", skillsPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		skillsPath,
		texts,
		func(text *skills.SkillText) string {
			return text.Text
		},
		func(text *skills.SkillText, translation string) {
			text.Text = translation
		},
		func(text *skills.SkillText) string {
			return fmt.Sprintf(
				"skill=%d field=%s",
				text.SkillID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf("translate Skills.json: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applySkillTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
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
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(skillsPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Skills_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(skillsPath, backupPath); err != nil {
		return fmt.Errorf("move original Skills.json: %w", err)
	}

	if err := os.Rename(tempPath, skillsPath); err != nil {
		restoreErr := os.Rename(backupPath, skillsPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Skills.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace Skills.json: %w", err)
	}

	fmt.Printf("Translation saved: %s\n", skillsPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

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
			"Skills.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Skills.json",
		),
		filepath.Join(
			rootDir,
			"Skills.json",
		),
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
		return nil, fmt.Errorf(
			"decode Skills.json: %w",
			err,
		)
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
		return nil, fmt.Errorf(
			"encode Skills.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
