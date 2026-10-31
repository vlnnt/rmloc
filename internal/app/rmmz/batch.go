package rmmz

import (
	"context"
	"fmt"
	"strings"

	"rmloc/internal/ollama"
	"rmloc/internal/progress"
)

const databaseTranslationBatchSize = 8

func translateTextEntries[T any](
	ctx context.Context,
	client *ollama.Client,
	reporter progress.Reporter,
	filePath string,
	entries []T,
	getText func(*T) string,
	setText func(*T, string),
	describe func(*T) string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if reporter == nil {
		reporter = progress.NewNoopReporter()
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	for start := 0; start < len(entries); start += databaseTranslationBatchSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		end := start + databaseTranslationBatchSize
		if end > len(entries) {
			end = len(entries)
		}

		sourceTexts := make([]string, end-start)
		for index := start; index < end; index++ {
			if err := ctx.Err(); err != nil {
				return err
			}

			sourceTexts[index-start] = getText(&entries[index])
		}

		translations, err := client.TranslateBatch(ctx, sourceTexts)
		if err != nil {
			return fmt.Errorf(
				"translate text entries %d-%d of %d: %w",
				start+1,
				end,
				len(entries),
				err,
			)
		}

		if len(translations) != len(sourceTexts) {
			return fmt.Errorf(
				"translation count mismatch for entries %d-%d: received %d, expected %d",
				start+1,
				end,
				len(translations),
				len(sourceTexts),
			)
		}

		for offset, translation := range translations {
			if err := ctx.Err(); err != nil {
				return err
			}

			entryIndex := start + offset
			translation = strings.TrimSpace(translation)
			if translation == "" {
				return fmt.Errorf(
					"empty translation for entry %d: %s",
					entryIndex,
					describe(&entries[entryIndex]),
				)
			}

			setText(&entries[entryIndex], translation)
		}

		reporter.FileAdvanced(filePath, end-start)
	}

	return nil
}
