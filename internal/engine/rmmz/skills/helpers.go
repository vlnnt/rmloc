package skills

import "strings"

func appendSkillText(
	texts []SkillText,
	skillID, skillIndex int,
	field, text string,
) []SkillText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, SkillText{
		SkillID:    skillID,
		SkillIndex: skillIndex,
		Field:      field,
		Text:       text,
	})
}
