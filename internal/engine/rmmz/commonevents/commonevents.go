package commonevents

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]CommonEventText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read common events file: %w", err)
	}

	var events []*commonEvent
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("decode common events file: %w", err)
	}

	texts := make([]CommonEventText, 0)
	for eventIndex, event := range events {
		if event == nil {
			continue
		}

		for commandIndex, command := range event.List {
			commandTexts, err := extractCommandTexts(
				event.ID,
				eventIndex,
				commandIndex,
				command,
			)
			if err != nil {
				return nil, err
			}

			texts = append(texts, commandTexts...)
		}
	}

	return texts, nil
}
