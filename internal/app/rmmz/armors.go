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
	"time"

	"rmloc/internal/engine/rmmz/armors"
)

func (a *RmmzEngine) ProcessArmors(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	armorsPath, err := findArmorsFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(armorsPath)
	if err != nil {
		return fmt.Errorf(
			"read Armors.json: %w",
			err,
		)
	}

	fileInfo, err := os.Stat(armorsPath)
	if err != nil {
		return fmt.Errorf(
			"stat Armors.json: %w",
			err,
		)
	}

	texts, err := armors.Extract(armorsPath)
	if err != nil {
		return fmt.Errorf(
			"extract Armors.json: %w",
			err,
		)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(armorsPath, len(texts))
	reporter.FileStarted(armorsPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(armorsPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(armorsPath)

		default:
			reporter.FileFailed(armorsPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", armorsPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		armorsPath,
		texts,
		func(text *armors.ArmorText) string {
			return text.Text
		},
		func(text *armors.ArmorText, translation string) {
			text.Text = translation
		},
		func(text *armors.ArmorText) string {
			return fmt.Sprintf(
				"armor=%d field=%s",
				text.ArmorID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf(
			"translate Armors.json: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyArmorTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := armorsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Armors.json: %w",
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(armorsPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	backupName := fmt.Sprintf(
		"Armors_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(armorsPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original Armors.json: %w",
			err,
		)
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

	fmt.Printf("Translation saved: %s\n", armorsPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findArmorsFile(
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
			"Armors.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Armors.json",
		),
		filepath.Join(
			rootDir,
			"Armors.json",
		),
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
		return "", fmt.Errorf(
			"search Armors.json: %w",
			err,
		)
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
		return nil, fmt.Errorf(
			"decode Armors.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.ArmorIndex < 0 ||
			text.ArmorIndex >= len(records) {
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
		return nil, fmt.Errorf(
			"encode Armors.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
