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

	"rmloc/internal/engine/rmmz/weapons"
)

func (a *RmmzEngine) ProcessWeapons(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	weaponsPath, err := findWeaponsFile(gameDir)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
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

	reporter := a.Reporter()
	reporter.FileDiscovered(weaponsPath, len(texts))
	reporter.FileStarted(weaponsPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(weaponsPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(weaponsPath)

		default:
			reporter.FileFailed(weaponsPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", weaponsPath)
	fmt.Printf("Found text entries: %d\n", len(texts))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		weaponsPath,
		texts,
		func(text *weapons.WeaponText) string {
			return text.Text
		},
		func(text *weapons.WeaponText, translation string) {
			text.Text = translation
		},
		func(text *weapons.WeaponText) string {
			return fmt.Sprintf(
				"weapon=%d field=%s",
				text.WeaponID,
				text.Field,
			)
		},
	); err != nil {
		return fmt.Errorf("translate Weapons.json: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyWeaponTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
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
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(weaponsPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Weapons_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(weaponsPath, backupPath); err != nil {
		return fmt.Errorf("move original Weapons.json: %w", err)
	}

	if err := os.Rename(tempPath, weaponsPath); err != nil {
		restoreErr := os.Rename(backupPath, weaponsPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Weapons.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace Weapons.json: %w", err)
	}

	fmt.Printf("Translation saved: %s\n", weaponsPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findWeaponsFile(
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
			"Weapons.json",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
			"Weapons.json",
		),
		filepath.Join(
			rootDir,
			"Weapons.json",
		),
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

			if entry.Name() != "Weapons.json" {
				return nil
			}

			if !strings.EqualFold(filepath.Base(filepath.Dir(path)), "data") {
				return nil
			}

			weaponsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf("search Weapons.json: %w", err)
	}

	if weaponsPath == "" {
		return "", fmt.Errorf("data/Weapons.json not found inside: %s", rootDir)
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
