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
	"rmloc/internal/engine/rmmz/commonevents"
	"strings"
	"time"
)

func (a *RmmzEngine) ProcessCommonEvents(
	ctx context.Context,
	gameDir string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	filePath, err := findCommonEventsFile(gameDir)
	if err != nil {
		return err
	}

	sourceData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read CommonEvents.json: %w", err)
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat CommonEvents.json: %w", err)
	}

	texts, err := commonevents.Extract(filePath)
	if err != nil {
		return fmt.Errorf("extract CommonEvents.json: %w", err)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(filePath, len(texts))
	reporter.FileStarted(filePath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(filePath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(filePath)

		default:
			reporter.FileFailed(filePath, resultErr)
		}
	}()

	for index := 0; index < len(texts); {
		if err := ctx.Err(); err != nil {
			return err
		}

		end := commonEventBlockEnd(texts, index)
		if end-index > 1 {
			lines := make([]string, end-index)
			for i := index; i < end; i++ {
				lines[i-index] = texts[i].Text
			}

			first := texts[index]
			last := texts[end-1]

			fmt.Printf(
				"[CommonEvents.json %d-%d/%d] event=%d commands=%d-%d field=%s\n",
				index+1,
				end,
				len(texts),
				first.EventID,
				first.CommandIndex,
				last.CommandIndex,
				first.Field,
			)

			translations, err := a.ollamaClient.TranslateLines(ctx, lines)
			if err != nil {
				return fmt.Errorf(
					"translate CommonEvents.json event %d commands %d-%d: %w",
					first.EventID,
					first.CommandIndex,
					last.CommandIndex,
					err,
				)
			}

			if len(translations) != len(lines) {
				return fmt.Errorf(
					"translation count mismatch in CommonEvents.json event %d: received %d, expected %d",
					first.EventID,
					len(translations),
					len(lines),
				)
			}

			for i := index; i < end; i++ {
				if err := ctx.Err(); err != nil {
					return err
				}

				translation := strings.TrimSpace(translations[i-index])
				if translation == "" {
					return fmt.Errorf(
						"empty translation in CommonEvents.json event %d command %d",
						texts[i].EventID,
						texts[i].CommandIndex,
					)
				}

				texts[i].Text = translation
			}

			reporter.FileAdvanced(filePath, len(lines))
			index = end
			continue
		}

		text := &texts[index]
		fmt.Printf(
			"[CommonEvents.json %d/%d] event=%d command=%d field=%s text=%q\n",
			index+1,
			len(texts),
			text.EventID,
			text.CommandIndex,
			text.Field,
			text.Text,
		)

		translated, err := a.ollamaClient.Translate(ctx, text.Text)
		if err != nil {
			return fmt.Errorf(
				"translate CommonEvents.json event %d command %d field %s: %w",
				text.EventID,
				text.CommandIndex,
				text.Field,
				err,
			)
		}

		translated = strings.TrimSpace(translated)
		if translated == "" {
			return fmt.Errorf(
				"empty translation in CommonEvents.json event %d command %d",
				text.EventID,
				text.CommandIndex,
			)
		}

		text.Text = translated
		reporter.FileAdvanced(filePath, 1)
		index++
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyCommonEventTranslations(sourceData, texts)
	if err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := filePath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary CommonEvents.json: %w",
			err,
		)
	}

	defer os.Remove(tempPath)
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(filePath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
	}

	backupName := fmt.Sprintf(
		"CommonEvents_%s.json",
		time.Now().Format("20060102_150405"),
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(filePath, backupPath); err != nil {
		return fmt.Errorf(
			"move original CommonEvents.json: %w",
			err,
		)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		restoreErr := os.Rename(backupPath, filePath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace CommonEvents.json: %w; restore original: %v",
				err,
				restoreErr,
			)
		}

		return fmt.Errorf("replace CommonEvents.json: %w", err)
	}

	fmt.Printf("Translation saved: %s\n", filePath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func commonEventBlockEnd(
	texts []commonevents.CommonEventText,
	start int,
) int {
	first := texts[start]
	end := start + 1

	switch first.Field {
	case "message", "scrollingText":
		for end < len(texts) {
			previous := texts[end-1]
			current := texts[end]

			if current.Field != first.Field ||
				current.EventIndex != first.EventIndex ||
				current.CommandIndex != previous.CommandIndex+1 {
				break
			}

			end++
		}

	case "choice":
		for end < len(texts) {
			current := texts[end]

			if current.Field != "choice" ||
				current.EventIndex != first.EventIndex ||
				current.CommandIndex != first.CommandIndex ||
				current.ParameterIndex != first.ParameterIndex {
				break
			}

			end++
		}
	}

	return end
}

func findCommonEventsFile(
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
		return "", fmt.Errorf(
			"game path is not a directory: %s",
			rootDir,
		)
	}

	candidates := []string{
		filepath.Join(rootDir, "data", "CommonEvents.json"),
		filepath.Join(rootDir, "www", "data", "CommonEvents.json"),
		filepath.Join(rootDir, "CommonEvents.json"),
	}

	for _, candidate := range candidates {
		if isFile(candidate) {
			return candidate, nil
		}
	}

	var filePath string
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

			if !strings.EqualFold(entry.Name(), "CommonEvents.json") {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
				return nil
			}

			filePath = path
			return fs.SkipAll
		},
	)
	if err != nil {
		return "", fmt.Errorf("search CommonEvents.json: %w", err)
	}

	if filePath == "" {
		return "", fmt.Errorf(
			"data/CommonEvents.json not found inside: %s",
			rootDir,
		)
	}

	return filePath, nil
}

func applyCommonEventTranslations(
	sourceData []byte,
	texts []commonevents.CommonEventText,
) ([]byte, error) {
	var events []any
	if err := json.Unmarshal(sourceData, &events); err != nil {
		return nil, fmt.Errorf(
			"decode CommonEvents.json: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.EventIndex < 0 ||
			text.EventIndex >= len(events) {
			return nil, fmt.Errorf(
				"event index is out of range: %d",
				text.EventIndex,
			)
		}

		event, ok := events[text.EventIndex].(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"event at index %d is invalid",
				text.EventIndex,
			)
		}

		commands, ok := event["list"].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"command list of event %d is invalid",
				text.EventID,
			)
		}

		if text.CommandIndex < 0 ||
			text.CommandIndex >= len(commands) {
			return nil, fmt.Errorf(
				"command index is out of range: %d",
				text.CommandIndex,
			)
		}

		command, ok := commands[text.CommandIndex].(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"command %d of event %d is invalid",
				text.CommandIndex,
				text.EventID,
			)
		}

		parameters, ok := command["parameters"].([]any)
		if !ok {
			return nil, fmt.Errorf(
				"parameters of command %d are invalid",
				text.CommandIndex,
			)
		}

		if text.ParameterIndex < 0 ||
			text.ParameterIndex >= len(parameters) {
			return nil, fmt.Errorf(
				"parameter index is out of range: %d",
				text.ParameterIndex,
			)
		}

		if text.ChoiceIndex >= 0 {
			choices, ok := parameters[text.ParameterIndex].([]any)
			if !ok {
				return nil, fmt.Errorf(
					"choices of command %d are invalid",
					text.CommandIndex,
				)
			}

			if text.ChoiceIndex >= len(choices) {
				return nil, fmt.Errorf(
					"choice index is out of range: %d",
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

	if err := encoder.Encode(events); err != nil {
		return nil, fmt.Errorf(
			"encode CommonEvents.json: %w",
			err,
		)
	}

	return output.Bytes(), nil
}
