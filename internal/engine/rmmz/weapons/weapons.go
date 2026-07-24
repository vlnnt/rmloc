package weapons

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]WeaponText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Weapons.json: %w", err)
	}

	var weapons []*weaponRecord
	if err := json.Unmarshal(data, &weapons); err != nil {
		return nil, fmt.Errorf("decode Weapons.json: %w", err)
	}

	texts := make([]WeaponText, 0)
	for weaponIndex, weapon := range weapons {
		if weapon == nil {
			continue
		}

		texts = appendWeaponText(
			texts, weapon.ID, weaponIndex, "name", weapon.Name,
		)

		texts = appendWeaponText(
			texts, weapon.ID, weaponIndex, "description", weapon.Description,
		)
	}

	return texts, nil
}
