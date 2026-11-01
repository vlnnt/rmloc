package ollama

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	cacheKindTranslation = "translation"
	cacheKindDialogue    = "dialogue"
)

func (c *Client) EnablePersistentCache(
	cachePath string,
) error {
	if strings.TrimSpace(cachePath) == "" {
		return fmt.Errorf("cache path is empty")
	}

	path, err := filepath.Abs(cachePath)
	if err != nil {
		return fmt.Errorf("resolve cache path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}

	c.cacheFileMu.Lock()
	defer c.cacheFileMu.Unlock()

	file, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("open translation cache: %w", err)
	}

	if err == nil {
		defer func() {
			_ = file.Close()
		}()

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

		for scanner.Scan() {
			var entry persistentCacheEntry
			if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
				continue
			}

			if entry.Model != c.model ||
				entry.PromptHash != c.promptHash {
				continue
			}

			switch entry.Kind {
			case cacheKindTranslation:
				if entry.Key == "" || strings.TrimSpace(entry.Translation) == "" {
					continue
				}

				c.storeTranslationCacheEntry(entry.Key, entry.Translation)

			case cacheKindDialogue:
				if entry.Key == "" || len(entry.Translations) == 0 {
					continue
				}

				valid := true
				for _, translation := range entry.Translations {
					if strings.TrimSpace(translation) == "" {
						valid = false
						break
					}
				}

				if valid {
					c.storeDialogueCacheEntry(
						entry.Key,
						entry.Translations,
					)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read translation cache: %w", err)
		}
	}

	c.cachePath = path
	return nil
}

func (c *Client) persistTranslation(
	key, translation string,
) error {
	return c.appendPersistentCacheEntry(persistentCacheEntry{
		Model:       c.model,
		PromptHash:  c.promptHash,
		Kind:        cacheKindTranslation,
		Key:         key,
		Translation: translation,
	})
}

func (c *Client) persistDialogue(
	key string,
	translations []string,
) error {
	return c.appendPersistentCacheEntry(persistentCacheEntry{
		Model:        c.model,
		PromptHash:   c.promptHash,
		Kind:         cacheKindDialogue,
		Key:          key,
		Translations: copyStrings(translations),
	})
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
		0o644,
	)
	if err != nil {
		return fmt.Errorf("open translation cache for writing: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(entry); err != nil {
		_ = file.Close()
		return fmt.Errorf("write cache entry: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close translation cache: %w", err)
	}

	return nil
}

func (c *Client) storeTranslationCacheEntry(
	key, translation string,
) {
	c.translationCache.Store(key, translation)
}

func (c *Client) storeDialogueCacheEntry(
	key string,
	translations []string,
) {
	c.dialogueCache.Store(key, copyStrings(translations))
}
