package maps

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]MapText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read map file: %w", err)
	}

	var record mapRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode map file: %w", err)
	}

	texts := make([]MapText, 0)
	texts = appendMapText(texts, MapText{
		EventID:        -1,
		EventIndex:     -1,
		PageIndex:      -1,
		CommandIndex:   -1,
		CommandCode:    0,
		ParameterIndex: -1,
		ChoiceIndex:    -1,
		Field:          "displayName",
		Text:           record.DisplayName,
	})

	for eventIndex, event := range record.Events {
		if event == nil {
			continue
		}

		for pageIndex, page := range event.Pages {
			for commandIndex, command := range page.List {
				commandTexts, err := extractCommandTexts(
					event.ID,
					eventIndex,
					pageIndex,
					commandIndex,
					command,
				)
				if err != nil {
					return nil, err
				}

				texts = append(texts, commandTexts...)
			}
		}
	}

	return texts, nil
}
