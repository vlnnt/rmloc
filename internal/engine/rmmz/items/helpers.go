package items

import "strings"

func appendItemText(
	texts []ItemText,
	itemID, itemIndex int,
	field, text string,
) []ItemText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, ItemText{
		ItemID:    itemID,
		ItemIndex: itemIndex,
		Field:     field,
		Text:      text,
	})
}
