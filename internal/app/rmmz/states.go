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

	"rmloc/internal/engine/rmmz/states"
)

func (a *RmmzEngine) ProcessStates(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	statesPath, err := findStatesFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(statesPath)
	if err != nil {
		return fmt.Errorf("read States.json: %w", err)
	}

	fileInfo, err := os.Stat(statesPath)
	if err != nil {
		return fmt.Errorf("stat States.json: %w", err)
	}

	texts, err := states.Extract(statesPath)
	if err != nil {
		return fmt.Errorf("extract States.json: %w", err)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(statesPath, len(texts))
	reporter.FileStarted(statesPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(statesPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(statesPath)

		default:
			reporter.FileFailed(statesPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", statesPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		statesPath,
		texts,
		func(text *states.StateText) string {
			return text.Text
		},
		func(text *states.StateText, translation string) {
			text.Text = translation
		},
		func(text *states.StateText) string {
			return fmt.Sprintf(
				"state=%d field=%s",
				text.StateID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf("translate States.json: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyStateTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := statesPath + ".tmp"
	if err := os.WriteFile(
		tempPath, translatedData, fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary States.json: %w",
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(statesPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"States_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(statesPath, backupPath); err != nil {
		return fmt.Errorf("move original States.json: %w", err)
	}

	if err := os.Rename(tempPath, statesPath); err != nil {
		restoreErr := os.Rename(backupPath, statesPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace States.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace States.json: %w", err)
	}

	fmt.Printf("Translation saved: %s\n", statesPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findStatesFile(
	gameDir string,
) (string, error) {
	rootDir, err := filepath.Abs(gameDir)
	if err != nil {
		return "", fmt.Errorf("resolve game directory: %w", err)
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		return "", fmt.Errorf("open game directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("game path is not a directory: %s", rootDir)
	}

	candidates := []string{
		filepath.Join(
			rootDir,
			"data",
			"States.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"States.json",
		),
		filepath.Join(
			rootDir,
			"States.json",
		),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var statesPath string
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

			if entry.Name() != "States.json" {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			statesPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf("search States.json: %w", err)
	}

	if statesPath == "" {
		return "", fmt.Errorf(
			"data/States.json not found inside: %s",
			rootDir,
		)
	}

	return statesPath, nil
}

func applyStateTranslations(
	sourceData []byte,
	texts []states.StateText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf("decode States.json: %w", err)
	}

	for _, text := range texts {
		if text.StateIndex < 0 ||
			text.StateIndex >= len(records) {
			return nil, fmt.Errorf(
				"state index is out of range: %d",
				text.StateIndex,
			)
		}

		record := records[text.StateIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"state at index %d is null",
				text.StateIndex,
			)
		}

		record[text.Field] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf("encode States.json: %w", err)
	}

	return output.Bytes(), nil
}
