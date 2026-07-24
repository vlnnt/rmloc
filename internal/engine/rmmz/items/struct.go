package items

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]ItemText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Items.json: %w", err)
	}

	var items []*itemRecord
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("decode Items.json: %w", err)
	}

	texts := make([]ItemText, 0)
	for itemIndex, item := range items {
		if item == nil {
			continue
		}

		texts = appendItemText(
			texts, item.ID, itemIndex, "name", item.Name,
		)

		texts = appendItemText(
			texts, item.ID, itemIndex, "description", item.Description,
		)
	}

	return texts, nil
}
