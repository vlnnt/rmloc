package ollama

import (
	"regexp"
	"strings"
)

var (
	latinLetterPattern         = regexp.MustCompile(`[A-Za-z]`)
	rpgMakerControlCodePattern = regexp.MustCompile(`\\[A-Za-z]+\[[^\]]*]`)
	rpgMakerSimpleCodePattern  = regexp.MustCompile(`\\[.$|!><^{}]`)
	htmlTagPattern             = regexp.MustCompile(`<[^>]*>`)
	placeholderPattern         = regexp.MustCompile(`\{\{?[^{}]+\}?\}|\$\{[^{}]+\}|%[a-zA-Z]`)
)

func needsTranslation(
	text string,
) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	text = rpgMakerControlCodePattern.ReplaceAllString(text, "")
	text = rpgMakerSimpleCodePattern.ReplaceAllString(text, "")
	text = htmlTagPattern.ReplaceAllString(text, "")
	text = placeholderPattern.ReplaceAllString(text, "")
	return latinLetterPattern.MatchString(text)
}

func needsAnyTranslation(
	lines []string,
) bool {
	for _, line := range lines {
		if needsTranslation(line) {
			return true
		}
	}

	return false
}
