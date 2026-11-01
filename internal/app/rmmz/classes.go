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
	"rmloc/internal/engine/rmmz/classes"
	"time"
)

func (a *RmmzEngine) ProcessClasses(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	classesPath, err := findClassesFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(classesPath)
	if err != nil {
		return fmt.Errorf(
			"read Classes.json: %w",
			err,
		)
	}

	fileInfo, err := os.Stat(classesPath)
	if err != nil {
		return fmt.Errorf(
			"stat Classes.json: %w",
			err,
		)
	}

	texts, err := classes.Extract(classesPath)
	if err != nil {
		return fmt.Errorf(
			"extract Classes.json: %w",
			err,
		)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(classesPath, len(texts))
	reporter.FileStarted(classesPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(classesPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(classesPath)

		default:
			reporter.FileFailed(classesPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", classesPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		classesPath,
		texts,
		func(text *classes.ClassText) string {
			return text.Text
		},
		func(text *classes.ClassText, translation string) {
			text.Text = translation
		},
		func(text *classes.ClassText) string {
			return fmt.Sprintf(
				"class=%d field=%s",
				text.ClassID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf(
			"translate Classes.json: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyClassTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := classesPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Classes.json: %w",
			err,
		)
	}

	defer os.Remove(tempPath)
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(classesPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	backupName := fmt.Sprintf(
		"Classes_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(classesPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original Classes.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, classesPath); err != nil {
		restoreErr := os.Rename(backupPath, classesPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Classes.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace Classes.json: %w",
			err,
		)
	}

	fmt.Printf("Translation saved: %s\n", classesPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findClassesFile(
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
		filepath.Join(rootDir, "data", "Classes.json"),
		filepath.Join(rootDir, "www", "data", "Classes.json"),
		filepath.Join(rootDir, "Classes.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var classesPath string
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

			if entry.Name() != "Classes.json" {
				return nil
			}

			if filepath.Base(filepath.Dir(path)) != "data" {
				return nil
			}

			classesPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Classes.json: %w",
			err,
		)
	}

	if classesPath == "" {
		return "", fmt.Errorf(
			"data/Classes.json not found inside: %s",
			rootDir,
		)
	}

	return classesPath, nil
}

func applyClassTranslations(
	sourceData []byte,
	texts []classes.ClassText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf(
			"decode Classes.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.ClassIndex < 0 ||
			text.ClassIndex >= len(records) {
			return nil, fmt.Errorf(
				"class index is out of range: %d",
				text.ClassIndex,
			)
		}

		record := records[text.ClassIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"class at index %d is null",
				text.ClassIndex,
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
			"encode Classes.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
