package ollama

import (
	"fmt"
	"regexp"
	"strings"
)

var protectedFragmentPattern = regexp.MustCompile(
	`\\\\|` +
		`\\[A-Za-z]+\[[^]]*\]|` +
		`\\[A-Za-z]+|` +
		`\\[.$|!><^{}]|` +
		`<[^>\r\n]*>|` +
		`\$\{[^{}\r\n]+\}|` +
		`\{\{[^{}\r\n]+\}\}|` +
		`\{[^{}\r\n]+\}|` +
		`%([0-9]+\$)?[-+#0 ']*[0-9]*(\.[0-9]+)?[bcdeEfFgGopqstvxX]|` +
		`%[0-9]+`,
)

type protectedFragment struct {
	Marker   string
	Original string
}

type protectedText struct {
	Text      string
	namespace string
	fragments []protectedFragment
}

func protectText(
	text string,
) protectedText {
	namespace := "RMLOC"
	for strings.Contains(text, "⟦"+namespace+"_") {
		namespace += "X"
	}

	fragments := make([]protectedFragment, 0)
	protected := protectedFragmentPattern.ReplaceAllStringFunc(
		text,
		func(original string) string {
			marker := fmt.Sprintf(
				"⟦%s_%04d⟧",
				namespace,
				len(fragments),
			)

			fragments = append(fragments, protectedFragment{
				Marker:   marker,
				Original: original,
			})

			return marker
		},
	)

	return protectedText{
		Text:      protected,
		namespace: namespace,
		fragments: fragments,
	}
}

func (p protectedText) restore(
	translated string,
) (string, error) {
	for _, fragment := range p.fragments {
		count := strings.Count(translated, fragment.Marker)
		if count != 1 {
			return "", fmt.Errorf(
				"protected marker %q occurs %d times, expected 1",
				fragment.Marker,
				count,
			)
		}
	}

	for _, fragment := range p.fragments {
		translated = strings.Replace(
			translated,
			fragment.Marker,
			fragment.Original,
			1,
		)
	}

	if len(p.fragments) > 0 &&
		strings.Contains(translated, "⟦"+p.namespace+"_") {
		return "", fmt.Errorf("unexpected protected marker in translation")
	}

	return translated, nil
}

func protectLines(
	lines []string,
) ([]string, []protectedText) {
	texts := make([]string, len(lines))
	protected := make([]protectedText, len(lines))

	for i, line := range lines {
		protected[i] = protectText(line)
		texts[i] = protected[i].Text
	}

	return texts, protected
}
