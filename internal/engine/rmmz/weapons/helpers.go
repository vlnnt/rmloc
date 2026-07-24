package weapons

import "strings"

func appendWeaponText(
	texts []WeaponText,
	weaponID, weaponIndex int,
	field, text string,
) []WeaponText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, WeaponText{
		WeaponID:    weaponID,
		WeaponIndex: weaponIndex,
		Field:       field,
		Text:        text,
	})
}
