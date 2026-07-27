package ollama

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const translationCacheVersion = 1

const (
	cacheKindTranslation = "translation"
	cacheKindDialogue    = "dialogue"
)

func (c *Client) EnablePersistentCache(
	cachePath string,
) error {
	cachePath = strings.TrimSpace(cachePath)
	if cachePath == "" {
		return fmt.Errorf("cache path is empty")
	}

	absolutePath, err := filepath.Abs(cachePath)
	if err != nil {
		return fmt.Errorf(
			"resolve cache path: %w",
			err,
		)
	}

	if err := os.MkdirAll(
		filepath.Dir(absolutePath),
		0755,
	); err != nil {
		return fmt.Errorf(
			"create cache directory: %w",
			err,
		)
	}

	c.cacheFileMu.Lock()
	defer c.cacheFileMu.Unlock()

	c.cachePath = absolutePath
	file, err := os.Open(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf(
			"open translation cache: %w",
			err,
		)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(
		make([]byte, 64*1024),
		4*1024*1024,
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry persistentCacheEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Version != translationCacheVersion {
			continue
		}

		if entry.Model != c.model {
			continue
		}

		switch entry.Kind {
		case cacheKindTranslation:
			if entry.Key == "" ||
				strings.TrimSpace(entry.Translation) == "" {
				continue
			}

			c.translationCache.Store(
				entry.Key,
				entry.Translation,
			)

		case cacheKindDialogue:
			if entry.Key == "" ||
				len(entry.Translations) == 0 {
				continue
			}

			valid := true
			for _, translation := range entry.Translations {
				if strings.TrimSpace(translation) == "" {
					valid = false
					break
				}
			}

			if !valid {
				continue
			}

			c.dialogueCache.Store(
				entry.Key,
				copyStrings(entry.Translations),
			)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf(
			"read translation cache: %w",
			err,
		)
	}

	return nil
}

func (c *Client) persistTranslation(
	key string,
	translation string,
) error {
	return c.appendPersistentCacheEntry(
		persistentCacheEntry{
			Version:     translationCacheVersion,
			Model:       c.model,
			Kind:        cacheKindTranslation,
			Key:         key,
			Translation: translation,
		},
	)
}

func (c *Client) persistDialogue(
	key string,
	translations []string,
) error {
	return c.appendPersistentCacheEntry(
		persistentCacheEntry{
			Version:      translationCacheVersion,
			Model:        c.model,
			Kind:         cacheKindDialogue,
			Key:          key,
			Translations: copyStrings(translations),
		},
	)
}

func (c *Client) appendPersistentCacheEntry(
	entry persistentCacheEntry,
) error {
	c.cacheFileMu.Lock()
	defer c.cacheFileMu.Unlock()
	if c.cachePath == "" {
		return nil
	}

	file, err := os.OpenFile(
		c.cachePath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return fmt.Errorf(
			"open translation cache for writing: %w",
			err,
		)
	}

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(entry); err != nil {
		file.Close()

		return fmt.Errorf(
			"write translation cache: %w",
			err,
		)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf(
			"close translation cache: %w",
			err,
		)
	}

	return nil
}
