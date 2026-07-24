package armors

import "strings"

func appendArmorText(
	texts []ArmorText,
	armorID, armorIndex int,
	field, text string,
) []ArmorText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, ArmorText{
		ArmorID:    armorID,
		ArmorIndex: armorIndex,
		Field:      field,
		Text:       text,
	})
}
