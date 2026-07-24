package rmmz

import (
	"bytes"
	"encoding/json"
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

var mapFilePattern = regexp.MustCompile(`(?i)^Map\d{3,}\.json$`)

func (a *RmmzEngine) ProcessMap(
	mapPath string,
) error {
	sourceData, err := os.ReadFile(mapPath)
	if err != nil {
		return fmt.Errorf(
			"read %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	fileInfo, err := os.Stat(mapPath)
	if err != nil {
		return fmt.Errorf(
			"stat %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	texts, err := rmmaps.Extract(mapPath)
	if err != nil {
		return fmt.Errorf(
			"extract %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	fmt.Printf("Найден файл: %s\n", mapPath)
	fmt.Printf(
		"Найдено строк в %s: %d\n",
		filepath.Base(mapPath),
		len(texts),
	)

	cache := make(map[string]string)
	for index := range texts {
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

		translated, exists := cache[text.Text]
		if !exists {
			translated, err = a.ollamaClient.Translate(text.Text)
			if err != nil {
				return fmt.Errorf(
					"translate %s event %d page %d command %d: %w",
					filepath.Base(mapPath),
					text.EventID,
					text.PageIndex,
					text.CommandIndex,
					err,
				)
			}

			if strings.TrimSpace(translated) == "" {
				return fmt.Errorf(
					"empty translation in %s event %d page %d command %d",
					filepath.Base(mapPath),
					text.EventID,
					text.PageIndex,
					text.CommandIndex,
				)
			}
			cache[text.Text] = translated
		}
		text.Text = translated
	}

	translatedData, err := applyMapTranslations(
		sourceData,
		texts,
	)
	if err != nil {
		return fmt.Errorf(
			"apply translations to %s: %w",
			filepath.Base(mapPath),
			err,
		)
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
	oldDir := filepath.Join(
		filepath.Dir(mapPath),
		"old",
	)

	if err := os.MkdirAll(oldDir, 0755); err != nil {
		return fmt.Errorf("create old directory: %w", err)
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

	backupPath := filepath.Join(
		oldDir,
		backupName,
	)

	if err := os.Rename(
		mapPath,
		backupPath,
	); err != nil {
		return fmt.Errorf(
			"move original %s: %w",
			filepath.Base(mapPath),
			err,
		)
	}

	if err := os.Rename(
		tempPath,
		mapPath,
	); err != nil {
		restoreErr := os.Rename(
			backupPath,
			mapPath,
		)

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

	fmt.Printf("Перевод сохранён: %s\n", mapPath)
	fmt.Printf("Оригинал сохранён: %s\n\n", backupPath)

	return nil
}

func (a *RmmzEngine) FindMapFiles(
	gameDir string,
) ([]string, error) {
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
		filepath.Join(rootDir, "data"),
		filepath.Join(rootDir, "www", "data"),
		rootDir,
	}

	for _, candidate := range candidates {
		files, err := readMapFiles(candidate)
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

			if !mapFilePattern.MatchString(entry.Name()) {
				return nil
			}

			if !strings.EqualFold(
				filepath.Base(filepath.Dir(path)),
				"data",
			) {
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
	fmt.Printf("Найдено карт: %d\n", len(mapFiles))
	return mapFiles, nil
}

func applyMapTranslations(
	sourceData []byte,
	texts []rmmaps.MapText,
) ([]byte, error) {
	var record map[string]any
	if err := json.Unmarshal(sourceData, &record); err != nil {
		return nil, fmt.Errorf("decode map JSON: %w", err)
	}

	for _, text := range texts {
		if text.Field == "displayName" {
			record["displayName"] = text.Text
			continue
		}

		if err := applyMapCommandTranslation(
			record,
			text,
		); err != nil {
			return nil, err
		}
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(record); err != nil {
		return nil, fmt.Errorf("encode map JSON: %w", err)
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
		if entry.IsDir() {
			continue
		}

		if !mapFilePattern.MatchString(entry.Name()) {
			continue
		}

		mapFiles = append(
			mapFiles,
			filepath.Join(dataDir, entry.Name()),
		)
	}

	sort.Strings(mapFiles)
	if len(mapFiles) > 0 {
		fmt.Printf("Найдено карт: %d\n", len(mapFiles))
	}

	return mapFiles, nil
}
