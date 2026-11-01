package system

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]SystemText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read system file: %w", err)
	}

	var record systemRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode system file: %w", err)
	}

	texts := make([]SystemText, 0)

	texts = appendSystemArrayTexts(texts, "armorTypes", record.ArmorTypes)
	texts = appendSystemArrayTexts(texts, "elements", record.Elements)
	texts = appendSystemArrayTexts(texts, "equipTypes", record.EquipTypes)
	texts = appendSystemArrayTexts(texts, "skillTypes", record.SkillTypes)
	texts = appendSystemArrayTexts(texts, "weaponTypes", record.WeaponTypes)

	texts = appendSystemArrayTexts(texts, "terms.basic", record.Terms.Basic)
	texts = appendSystemArrayTexts(texts, "terms.commands", record.Terms.Commands)
	texts = appendSystemArrayTexts(texts, "terms.params", record.Terms.Params)

	texts = appendSystemMessageTexts(texts, record.Terms.Messages)

	return texts, nil
}
