package states

import "strings"

func appendStateText(
	texts []StateText,
	stateID, stateIndex int,
	field, text string,
) []StateText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, StateText{
		StateID:    stateID,
		StateIndex: stateIndex,
		Field:      field,
		Text:       text,
	})
}
