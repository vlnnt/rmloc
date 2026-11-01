package system

import (
	"sort"
	"strings"
)

func appendSystemText(
	texts []SystemText,
	text SystemText,
) []SystemText {
	if strings.TrimSpace(text.Text) == "" {
		return texts
	}

	return append(texts, text)
}

func appendSystemArrayTexts(
	texts []SystemText,
	field string,
	values []string,
) []SystemText {
	for index, value := range values {
		texts = appendSystemText(texts, SystemText{
			Field: field,
			Index: index,
			Text:  value,
		})
	}

	return texts
}

func appendSystemMessageTexts(
	texts []SystemText,
	messages map[string]string,
) []SystemText {
	keys := make([]string, 0, len(messages))
	for key := range messages {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	for _, key := range keys {
		texts = appendSystemText(texts, SystemText{
			Field: "terms.messages",
			Index: -1,
			Key:   key,
			Text:  messages[key],
		})
	}

	return texts
}
