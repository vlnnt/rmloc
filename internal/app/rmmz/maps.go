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
	"regexp"
	"sort"
	"strings"
	"time"

	rmmaps "rmloc/internal/engine/rmmz/maps"
)

var mapFilePattern = regexp.MustCompile(
	`(?i)^Map\d{3,}\.json$`,
)

func (a *RmmzEngine) ProcessMap(
	ctx context.Context,
	mapPath string,
) (resultErr error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	sourceData, err := os.ReadFile(mapPath)
	if err != nil {
		return fmt.Errorf(
			"read %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	fileInfo, err := os.Stat(mapPath)
	if err != nil {
		return fmt.Errorf(
			"stat %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	texts, err := rmmaps.Extract(mapPath)
	if err != nil {
		return fmt.Errorf(
			"extract %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	reporter := a.Reporter()
	reporter.FileDiscovered(mapPath, len(texts))
	reporter.FileStarted(mapPath)

	defer func() {
		switch {
		case resultErr == nil:
			reporter.FileCompleted(mapPath)

		case errors.Is(resultErr, context.Canceled) ||
			errors.Is(resultErr, context.DeadlineExceeded):
			reporter.FileCanceled(mapPath)

		default:
			reporter.FileFailed(mapPath, resultErr)
		}
	}()

	fmt.Printf("Found file: %s\n", mapPath)
	fmt.Printf(
		"Found text entries in %s: %d\n",
		filepath.Base(mapPath),
		len(texts),
	)

	for index := 0; index < len(texts); {
		if err := ctx.Err(); err != nil {
			return err
		}

		blockEnd := relatedTextBlockEnd(texts, index)
		if blockEnd-index > 1 {
			if err := a.translateMapTextBlock(
				ctx,
				reporter,
				mapPath,
				texts,
				index,
				blockEnd,
			); err != nil {
				return err
			}

			index = blockEnd
			continue
		}

		text := &texts[index]
		fmt.Printf(
			"[%s %d/%d] event=%d page=%d command=%d field=%s text=%q\n",
			filepath.Base(mapPath),
			index+1,
			len(texts),
			text.EventID,
			text.PageIndex,
			text.CommandIndex,
			text.Field,
			text.Text,
		)

		translated, err := a.ollamaClient.Translate(ctx, text.Text)
		if err != nil {
			return fmt.Errorf(
				"translate %s event %d page %d command %d field %s: %w",
				filepath.Base(mapPath),
				text.EventID,
				text.PageIndex,
				text.CommandIndex,
				text.Field,
				err,
			)
		}

		translated = strings.TrimSpace(translated)
		if translated == "" {
			return fmt.Errorf(
				"empty translation in %s event %d page %d command %d field %s",
				filepath.Base(mapPath),
				text.EventID,
				text.PageIndex,
				text.CommandIndex,
				text.Field,
			)
		}

		text.Text = translated
		reporter.FileAdvanced(mapPath, 1)
		index++
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	translatedData, err := applyMapTranslations(sourceData, texts)
	if err != nil {
		return fmt.Errorf(
			"apply translations to %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	tempPath := mapPath + ".tmp"
	if err := os.WriteFile(
		tempPath,
		translatedData,
		fileInfo.Mode().Perm(),
	); err != nil {
		return fmt.Errorf(
			"write temporary %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	defer os.Remove(tempPath)
	if err := ctx.Err(); err != nil {
		return err
	}

	oldDir := filepath.Join(filepath.Dir(mapPath), "old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		return fmt.Errorf(
			"create old directory: %w",
			err,
		)
	}

	extension := filepath.Ext(mapPath)
	baseName := strings.TrimSuffix(
		filepath.Base(mapPath),
		extension,
	)

	backupName := fmt.Sprintf(
		"%s_%s%s",
		baseName,
		time.Now().Format("20060102_150405"),
		extension,
	)

	backupPath := filepath.Join(oldDir, backupName)
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(mapPath, backupPath); err != nil {
		return fmt.Errorf(
			"move original %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	if err := os.Rename(tempPath, mapPath); err != nil {
		restoreErr := os.Rename(backupPath, mapPath)
		if restoreErr != nil {
			return fmt.Errorf(
				"replace %s: %w; restore original: %v",
				filepath.Base(mapPath),
				err,
				restoreErr,
			)
		}

		return fmt.Errorf(
			"replace %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	fmt.Printf("Translation saved: %s\n", mapPath)
	fmt.Printf("Original file backed up: %s\n\n", backupPath)

	return nil
}

func (a *RmmzEngine) translateMapTextBlock(
	ctx context.Context,
	reporter interface {
		FileAdvanced(path string, count int)
	},
	mapPath string,
	texts []rmmaps.MapText,
	start int,
	end int,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	lines := make([]string, end-start)
	for index := start; index < end; index++ {
		lines[index-start] = texts[index].Text
	}

	first := texts[start]
	last := texts[end-1]

	fmt.Printf(
		"[%s %d-%d/%d] event=%d page=%d commands=%d-%d field=%s elements=%d\n",
		filepath.Base(mapPath),
		start+1,
		end,
		len(texts),
		first.EventID,
		first.PageIndex,
		first.CommandIndex,
		last.CommandIndex,
		first.Field,
		len(lines),
	)

	translatedLines, err := a.ollamaClient.TranslateLines(ctx, lines)
	if err != nil {
		return fmt.Errorf(
			"translate %s block event %d page %d commands %d-%d field %s: %w",
			filepath.Base(mapPath),
			first.EventID,
			first.PageIndex,
			first.CommandIndex,
			last.CommandIndex,
			first.Field,
			err,
		)
	}

	if len(translatedLines) != len(lines) {
		return fmt.Errorf(
			"translation count mismatch in %s event %d page %d commands %d-%d: received %d, expected %d",
			filepath.Base(mapPath),
			first.EventID,
			first.PageIndex,
			first.CommandIndex,
			last.CommandIndex,
			len(translatedLines),
			len(lines),
		)
	}

	for index := start; index < end; index++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		translation := strings.TrimSpace(translatedLines[index-start])
		if translation == "" {
			return fmt.Errorf(
				"empty block translation in %s event %d page %d command %d element %d",
				filepath.Base(mapPath),
				texts[index].EventID,
				texts[index].PageIndex,
				texts[index].CommandIndex,
				index-start,
			)
		}

		texts[index].Text = translation
	}

	reporter.FileAdvanced(mapPath, len(lines))
	return nil
}

func relatedTextBlockEnd(
	texts []rmmaps.MapText,
	start int,
) int {
	if start < 0 ||
		start >= len(texts) {
		return start
	}

	switch texts[start].Field {
	case "message":
		return sequentialCommandBlockEnd(
			texts,
			start,
			"message",
		)

	case "scrollingText":
		return sequentialCommandBlockEnd(
			texts,
			start,
			"scrollingText",
		)

	case "choice":
		return choiceBlockEnd(
			texts,
			start,
		)

	default:
		return start + 1
	}
}

func sequentialCommandBlockEnd(
	texts []rmmaps.MapText,
	start int,
	field string,
) int {
	first := texts[start]
	end := start + 1
	for end < len(texts) {
		previous := texts[end-1]
		current := texts[end]
		if current.Field != field {
			break
		}

		if current.EventIndex != first.EventIndex ||
			current.PageIndex != first.PageIndex {
			break
		}

		if current.CommandIndex !=
			previous.CommandIndex+1 {
			break
		}

		end++
	}

	return end
}

func choiceBlockEnd(
	texts []rmmaps.MapText,
	start int,
) int {
	first := texts[start]
	end := start + 1
	for end < len(texts) {
		current := texts[end]
		if current.Field != "choice" {
			break
		}

		if current.EventIndex != first.EventIndex ||
			current.PageIndex != first.PageIndex ||
			current.CommandIndex != first.CommandIndex ||
			current.ParameterIndex != first.ParameterIndex {
			break
		}

		end++
	}

	return end
}

func (a *RmmzEngine) FindMapFiles(
	gameDir string,
) ([]string, error) {
	return a.FindMapFilesContext(context.Background(), gameDir)
}

func (a *RmmzEngine) FindMapFilesContext(
	ctx context.Context,
	gameDir string,
) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rootDir, err := filepath.Abs(gameDir)
	if err != nil {
		return nil, fmt.Errorf(
			"resolve game directory: %w",
			err,
		)
	}

	info, err := os.Stat(rootDir)
	if err != nil {
		return nil, fmt.Errorf(
			"open game directory: %w",
			err,
		)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf(
			"game path is not a directory: %s",
			rootDir,
		)
	}

	candidates := []string{
		filepath.Join(
			rootDir,
			"data",
		),
		filepath.Join(
			rootDir,
			"www",
			"data",
		),
		rootDir,
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		files, err := readMapFilesContext(ctx, candidate)
		if err != nil {
			return nil, err
		}

		if len(files) > 0 {
			return files, nil
		}
	}

	mapFiles := make([]string, 0)

	err = filepath.WalkDir(
		rootDir,
		func(
			path string,
			entry fs.DirEntry,
			walkErr error,
		) error {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

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

			if !mapFilePattern.MatchString(entry.Name()) {
				return nil
			}

			if !strings.EqualFold(filepath.Base(filepath.Dir(path)), "data") {
				return nil
			}

			mapFiles = append(mapFiles, path)
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"search map files: %w",
			err,
		)
	}

	if len(mapFiles) == 0 {
		return nil, fmt.Errorf(
			"MapXXX.json files not found inside: %s",
			rootDir,
		)
	}

	sort.Strings(mapFiles)
	fmt.Printf("Found maps: %d\n", len(mapFiles))

	return mapFiles, nil
}

func applyMapTranslations(
	sourceData []byte,
	texts []rmmaps.MapText,
) ([]byte, error) {
	var record map[string]any
	if err := json.Unmarshal(sourceData, &record); err != nil {
		return nil, fmt.Errorf(
			"decode map JSON: %w",
			err,
		)
	}

	for _, text := range texts {
		if text.Field == "displayName" {
			record["displayName"] = text.Text
			continue
		}

		if err := applyMapCommandTranslation(record, text); err != nil {
			return nil, err
		}
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(record); err != nil {
		return nil, fmt.Errorf(
			"encode map JSON: %w",
			err,
		)
	}

	return output.Bytes(), nil
}

func applyMapCommandTranslation(
	record map[string]any,
	text rmmaps.MapText,
) error {
	events, ok := record["events"].([]any)
	if !ok {
		return fmt.Errorf("events is not an array")
	}

	if text.EventIndex < 0 ||
		text.EventIndex >= len(events) {
		return fmt.Errorf(
			"event index is out of range: %d",
			text.EventIndex,
		)
	}

	event, ok := events[text.EventIndex].(map[string]any)
	if !ok {
		return fmt.Errorf(
			"event at index %d is invalid",
			text.EventIndex,
		)
	}

	pages, ok := event["pages"].([]any)
	if !ok {
		return fmt.Errorf(
			"pages of event %d is not an array",
			text.EventID,
		)
	}

	if text.PageIndex < 0 ||
		text.PageIndex >= len(pages) {
		return fmt.Errorf(
			"page index is out of range: %d",
			text.PageIndex,
		)
	}

	page, ok := pages[text.PageIndex].(map[string]any)
	if !ok {
		return fmt.Errorf(
			"page %d of event %d is invalid",
			text.PageIndex,
			text.EventID,
		)
	}

	commands, ok := page["list"].([]any)
	if !ok {
		return fmt.Errorf(
			"command list of event %d page %d is invalid",
			text.EventID,
			text.PageIndex,
		)
	}

	if text.CommandIndex < 0 ||
		text.CommandIndex >= len(commands) {
		return fmt.Errorf(
			"command index is out of range: %d",
			text.CommandIndex,
		)
	}

	command, ok := commands[text.CommandIndex].(map[string]any)
	if !ok {
		return fmt.Errorf(
			"command %d of event %d is invalid",
			text.CommandIndex,
			text.EventID,
		)
	}

	parameters, ok := command["parameters"].([]any)
	if !ok {
		return fmt.Errorf(
			"parameters of command %d are invalid",
			text.CommandIndex,
		)
	}

	if text.ParameterIndex < 0 ||
		text.ParameterIndex >= len(parameters) {
		return fmt.Errorf(
			"parameter index is out of range: %d",
			text.ParameterIndex,
		)
	}

	if text.ChoiceIndex >= 0 {
		choices, ok := parameters[text.ParameterIndex].([]any)
		if !ok {
			return fmt.Errorf(
				"choices of command %d are invalid",
				text.CommandIndex,
			)
		}

		if text.ChoiceIndex >= len(choices) {
			return fmt.Errorf(
				"choice index is out of range: %d",
				text.ChoiceIndex,
			)
		}

		choices[text.ChoiceIndex] = text.Text
		return nil
	}

	parameters[text.ParameterIndex] = text.Text
	return nil
}

func readMapFiles(
	dataDir string,
) ([]string, error) {
	return readMapFilesContext(context.Background(), dataDir)
}

func readMapFilesContext(
	ctx context.Context,
	dataDir string,
) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := os.Stat(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf(
			"open data directory %s: %w",
			dataDir,
			err,
		)
	}

	if !info.IsDir() {
		return nil, nil
	}

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, fmt.Errorf(
			"read data directory %s: %w",
			dataDir,
			err,
		)
	}

	mapFiles := make([]string, 0)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if entry.IsDir() {
			continue
		}

		if !mapFilePattern.MatchString(entry.Name()) {
			continue
		}

		mapFiles = append(mapFiles, filepath.Join(dataDir, entry.Name()))
	}

	sort.Strings(mapFiles)
	if len(mapFiles) > 0 {
		fmt.Printf("Found maps: %d\n", len(mapFiles))
	}

	return mapFiles, nil
}
