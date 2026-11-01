package classes

import "strings"

func appendClassText(
	texts []ClassText,
	text ClassText,
) []ClassText {
	if strings.TrimSpace(text.Text) == "" {
		return texts
	}

	return append(texts, text)
}
