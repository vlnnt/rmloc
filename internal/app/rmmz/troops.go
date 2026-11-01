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
	"rmloc/internal/engine/rmmz/troops"
	"strings"
	"time"
)

type troopTranslationGroup struct {
	Text             string
	TextIndexes      []int
	TroopID          int
	TroopIndex       int
	PageIndex        int
	LastCommandIndex int
	CommandCode      int
	Field            string
}

func (a *RmmzEngine) ProcessTroops(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	troopsPath, err := findTroopsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(troopsPath)
	if err != nil {
		return fmt.Errorf("read Troops.json: %w", err)
	}

	fileInfo, err := os.Stat(troopsPath)
	if err != nil {
		return fmt.Errorf("stat Troops.json: %w", err)
	}

	texts, err := troops.Extract(troopsPath)
	if err != nil {
		return fmt.Errorf("extract Troops.json: %w", err)
	}

	groups := buildTroopTranslationGroups(texts)
	reporter := a.Reporter()
	reporter.FileDiscovered(troopsPath, len(groups))
	reporter.FileStarted(troopsPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(troopsPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(troopsPath)

		default:
			reporter.FileFailed(troopsPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", troopsPath)
	fmt.Printf("Found text entries: %d\n", len(texts))
	fmt.Printf("Translation groups: %d\n", len(groups))

	if err := translateTextEntries(
		ctx,
		a.ollamaClient,
		reporter,
		troopsPath,
		groups,
		func(group *troopTranslationGroup) string {
			return group.Text
		},
		func(group *troopTranslationGroup, translation string) {
			group.Text = translation
		},
		func(group *troopTranslationGroup) string {
			return fmt.Sprintf(
				"troop=%d page=%d command=%d field=%s",
				group.TroopID,
				group.PageIndex,
				group.LastCommandIndex,
				group.Field,
			)
		},
	); err != nil {
		return fmt.Errorf("translate Troops.json: %w", err)
	}

	if err := distributeTroopTranslations(texts, groups); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyTroopTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := troopsPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary Troops.json: %w",
			err,
		)
	}

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(troopsPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"Troops_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(troopsPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original Troops.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, troopsPath); err != nil {
		restoreErr := os.Rename(backupPath, troopsPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace Troops.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace Troops.json: %w",
			err,
		)
	}

	fmt.Printf("Translation saved: %s\n", troopsPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func findTroopsFile(
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
		filepath.Join(rootDir, "data", "Troops.json"),
		filepath.Join(rootDir, "www", "data", "Troops.json"),
		filepath.Join(rootDir, "Troops.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var troopsPath string
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

			if entry.Name() != "Troops.json" {
				return nil
			}

			if filepath.Base(filepath.Dir(path)) != "data" {
				return nil
			}

			troopsPath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf(
			"search Troops.json: %w",
			err,
		)
	}

	if troopsPath == "" {
		return "", fmt.Errorf(
			"data/Troops.json not found inside: %s",
			rootDir,
		)
	}

	return troopsPath, nil
}

func buildTroopTranslationGroups(
	texts []troops.TroopText,
) []troopTranslationGroup {
	groups := make([]troopTranslationGroup, 0, len(texts))
	for index, text := range texts {
		isMessageLine := text.CommandCode == 401 ||
			text.CommandCode == 405

		if len(groups) > 0 && isMessageLine {
			last := &groups[len(groups)-1]

			if last.TroopIndex == text.TroopIndex &&
				last.PageIndex == text.PageIndex &&
				last.CommandCode == text.CommandCode &&
				last.LastCommandIndex+1 == text.CommandIndex &&
				!strings.Contains(last.Text, "\n") &&
				!strings.Contains(text.Text, "\n") {
				last.Text += "\n" + text.Text
				last.TextIndexes = append(last.TextIndexes, index)
				last.LastCommandIndex = text.CommandIndex
				continue
			}
		}

		groups = append(groups, troopTranslationGroup{
			Text:             text.Text,
			TextIndexes:      []int{index},
			TroopID:          text.TroopID,
			TroopIndex:       text.TroopIndex,
			PageIndex:        text.PageIndex,
			LastCommandIndex: text.CommandIndex,
			CommandCode:      text.CommandCode,
			Field:            text.Field,
		})
	}

	return groups
}

func distributeTroopTranslations(
	texts []troops.TroopText,
	groups []troopTranslationGroup,
) error {
	for _, group := range groups {
		if len(group.TextIndexes) == 1 {
			texts[group.TextIndexes[0]].Text = group.Text
			continue
		}

		translation := strings.ReplaceAll(group.Text, "\r\n", "\n")
		lines := strings.Split(translation, "\n")

		if len(lines) != len(group.TextIndexes) {
			return fmt.Errorf(
				"troop %d page %d: expected %d translated lines, got %d",
				group.TroopID,
				group.PageIndex,
				len(group.TextIndexes),
				len(lines),
			)
		}

		for index, textIndex := range group.TextIndexes {
			texts[textIndex].Text = lines[index]
		}
	}

	return nil
}

func applyTroopTranslations(
	sourceData []byte,
	texts []troops.TroopText,
) ([]byte, error) {
	var records []map[string]any
	if err := json.Unmarshal(sourceData, &records); err != nil {
		return nil, fmt.Errorf(
			"decode Troops.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.TroopIndex < 0 || text.TroopIndex >= len(records) {
			return nil, fmt.Errorf(
				"troop index out of range: %d",
				text.TroopIndex,
			)
		}

		record := records[text.TroopIndex]
		if record == nil {
			return nil, fmt.Errorf(
				"troop at index %d is null",
				text.TroopIndex,
			)
		}

		pages, ok := record["pages"].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"troop %d: pages is not an array",
				text.TroopID,
			)
		}

		if text.PageIndex < 0 || text.PageIndex >= len(pages) {
			return nil, fmt.Errorf(
				"troop %d: page index out of range: %d",
				text.TroopID,
				text.PageIndex,
			)
		}

		page, ok := pages[text.PageIndex].(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"troop %d: page is not an object",
				text.TroopID,
			)
		}

		commands, ok := page["list"].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"troop %d: command list is not an array",
				text.TroopID,
			)
		}

		if text.CommandIndex < 0 ||
			text.CommandIndex >= len(commands) {
			return nil, fmt.Errorf(
				"troop %d: command index out of range: %d",
				text.TroopID,
				text.CommandIndex,
			)
		}

		command, ok := commands[text.CommandIndex].(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"troop %d: command is not an object",
				text.TroopID,
			)
		}

		parameters, ok := command["parameters"].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"troop %d: parameters is not an array",
				text.TroopID,
			)
		}

		if text.ParameterIndex < 0 ||
			text.ParameterIndex >= len(parameters) {
			return nil, fmt.Errorf(
				"troop %d: parameter index out of range: %d",
				text.TroopID,
				text.ParameterIndex,
			)
		}

		if text.Field == "choice" {
			choices, ok := parameters[text.ParameterIndex].([]any)
			if !ok {
				return nil, fmt.Errorf(
					"troop %d: choices is not an array",
					text.TroopID,
				)
			}

			if text.ChoiceIndex < 0 ||
				text.ChoiceIndex >= len(choices) {
				return nil, fmt.Errorf(
					"troop %d: choice index out of range: %d",
					text.TroopID,
					text.ChoiceIndex,
				)
			}

			choices[text.ChoiceIndex] = text.Text
			continue
		}

		parameters[text.ParameterIndex] = text.Text
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(records); err != nil {
		return nil, fmt.Errorf(
			"encode Troops.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
