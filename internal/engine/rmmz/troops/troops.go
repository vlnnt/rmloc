package troops

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]TroopText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read troops file: %w", err)
	}

	var records []*troopRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode troops file: %w", err)
	}

	texts := make([]TroopText, 0)
	for troopIndex, record := range records {
		if record == nil {
			continue
		}

		for pageIndex, page := range record.Pages {
			for commandIndex, command := range page.List {
				commandTexts, err := extractCommandTexts(
					record.ID,
					troopIndex,
					pageIndex,
					commandIndex,
					command,
				)
				if err != nil {
					return nil, fmt.Errorf(
						"extract troop %d page %d command %d: %w",
						record.ID,
						pageIndex,
						commandIndex,
						err,
					)
				}

				texts = append(texts, commandTexts...)
			}
		}
	}

	return texts, nil
}
