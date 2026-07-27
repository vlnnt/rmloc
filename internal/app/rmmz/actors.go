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

	"rmloc/internal/engine/rmmz/actors"
)

func (a *RmmzEngine) ProcessActors(
	gameDir string,
) error {
	actorsPath, err := findActorsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(actorsPath)
	if err != nil {
		return fmt.Errorf("read Actors.json: %w", err)
	}

	fileInfo, err := os.Stat(actorsPath)
	if err != nil {
		return fmt.Errorf("stat Actors.json: %w", err)
	}

	texts, err := actors.Extract(actorsPath)
	if err != nil {
		return fmt.Errorf("extract Actors.json: %w", err)
	}

	fmt.Printf("Found file: %s\n", actorsPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	cache := make(map[string]string)
	for index := range texts {
		text := &texts[index]
		fmt.Printf(
			"[%d/%d] actor=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.ActorID,
			text.Field,
			text.Text,
		)

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate actor %d field %s: %w",
					text.ActorID,
					text.Field,
					err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation for actor %d field %s",
					text.ActorID,
					text.Field,
				)
			}

			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applyActorTranslations(
		sourceData,
		texts,
	)
	if err != nil {
		return err
	}

	tempPath := actorsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Actors.json: %w",
			err,
		)
	}

	defer os.Remove(tempPath)
	oldDir := filepath.Join(
		filepath.Dir(actorsPath),
		"old",
	)

	if err := os.MkdirAll(
		oldDir,
		0755,
	); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	backupName := fmt.Sprintf(
		"Actors_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(
		oldDir,
		backupName,
	)

	if err := os.Rename(
		actorsPath,
		backupPath,
	); err != nil {
		return fmt.Errorf(
			"move original Actors.json: %w",
			err,
		)
	}

	if err := os.Rename(
		tempPath,
		actorsPath,
	); err != nil {
		restoreErr := os.Rename(
			backupPath,
			actorsPath,
		)

		if restoreErr != nil {
			return fmt.Errorf(
				"replace Actors.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace Actors.json: %w",
			err,
		)
	}

	fmt.Printf(
		"Translation saved: %s\n",
		actorsPath,
	)

	fmt.Printf(
		"Original file backed up: %s\n\n",
		backupPath,
	)

	return nil
}

func findActorsFile(
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
			"Actors.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Actors.json",
		),
		filepath.Join(
			rootDir,
			"Actors.json",
		),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var actorsPath string
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

			if entry.Name() != "Actors.json" {
				return nil
			}

			if filepath.Base(
				filepath.Dir(path),
			) != "data" {
				return nil
			}

			actorsPath = path

			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Actors.json: %w",
			err,
		)
	}

	if actorsPath == "" {
		return "", fmt.Errorf(
			"data/Actors.json not found inside: %s",
			rootDir,
		)
	}

	return actorsPath, nil
}

func applyActorTranslations(
	sourceData []byte,
	texts []actors.ActorText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(
		sourceData,
		&records,
	); err != nil {
		return nil, fmt.Errorf(
			"decode Actors.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.ActorIndex < 0 ||
			text.ActorIndex >= len(records) {
			return nil, fmt.Errorf(
				"actor index is out of range: %d",
				text.ActorIndex,
			)
		}

		record := records[text.ActorIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"actor at index %d is null",
				text.ActorIndex,
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
			"encode Actors.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}

func isFile(
	path string,
) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
