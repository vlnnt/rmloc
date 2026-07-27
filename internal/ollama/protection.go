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
	for strings.Contains(
		text,
		"⟦"+namespace+"_",
	) {
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

			fragments = append(
				fragments,
				protectedFragment{
					Marker:   marker,
					Original: original,
				},
			)

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
	if len(p.fragments) == 0 {
		return translated, nil
	}

	expectedMarkers := make(
		map[string]struct{},
		len(p.fragments),
	)

	for _, fragment := range p.fragments {
		expectedMarkers[fragment.Marker] = struct{}{}
		count := strings.Count(
			translated,
			fragment.Marker,
		)

		if count != 1 {
			return "", fmt.Errorf(
				"protected marker %q occurs %d times, expected exactly once",
				fragment.Marker,
				count,
			)
		}
	}

	markerPattern := regexp.MustCompile(
		regexp.QuoteMeta(
			"⟦"+p.namespace+"_",
		) + `[0-9]+⟧`,
	)

	for _, marker := range markerPattern.FindAllString(
		translated,
		-1,
	) {
		if _, exists := expectedMarkers[marker]; !exists {
			return "", fmt.Errorf(
				"unexpected protected marker %q",
				marker,
			)
		}
	}

	result := translated
	for _, fragment := range p.fragments {
		result = strings.ReplaceAll(
			result,
			fragment.Marker,
			fragment.Original,
		)
	}

	return result, nil
}

func protectLines(
	lines []string,
) ([]string, []protectedText) {
	protectedLines := make(
		[]string,
		len(lines),
	)

	protectedValues := make(
		[]protectedText,
		len(lines),
	)

	for index, line := range lines {
		protected := protectText(line)
		protectedLines[index] = protected.Text
		protectedValues[index] = protected
	}

	return protectedLines, protectedValues
}

func restoreLines(
	translatedLines []string,
	protectedValues []protectedText,
) ([]string, error) {
	if len(translatedLines) != len(protectedValues) {
		return nil, fmt.Errorf(
			"translated lines count %d does not match protected lines count %d",
			len(translatedLines),
			len(protectedValues),
		)
	}

	result := make([]string, len(translatedLines))
	for index, translated := range translatedLines {
		restored, err := protectedValues[index].restore(translated)
		if err != nil {
			return nil, fmt.Errorf(
				"restore protected values in line %d: %w",
				index,
				err,
			)
		}

		result[index] = restored
	}

	return result, nil
}
