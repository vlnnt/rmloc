package armors

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]ArmorText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Armors.json: %w", err)
	}

	var armors []*armorRecord
	if err := json.Unmarshal(data, &armors); err != nil {
		return nil, fmt.Errorf("decode Armors.json: %w", err)
	}

	texts := make([]ArmorText, 0)
	for armorIndex, armor := range armors {
		if armor == nil {
			continue
		}

		texts = appendArmorText(texts, armor.ID, armorIndex, "name", armor.Name)
		texts = appendArmorText(texts, armor.ID, armorIndex, "description", armor.Description)
	}

	return texts, nil
}
