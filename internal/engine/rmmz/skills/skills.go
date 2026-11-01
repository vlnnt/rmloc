package skills

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]SkillText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Skills.json: %w", err)
	}

	var skills []*skillRecord
	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("decode Skills.json: %w", err)
	}

	texts := make([]SkillText, 0)
	for skillIndex, skill := range skills {
		if skill == nil {
			continue
		}

		texts = appendSkillText(texts, skill.ID, skillIndex, "name", skill.Name)
		texts = appendSkillText(texts, skill.ID, skillIndex, "description", skill.Description)
		texts = appendSkillText(texts, skill.ID, skillIndex, "message1", skill.Message1)
		texts = appendSkillText(texts, skill.ID, skillIndex, "message2", skill.Message2)
	}

	return texts, nil
}
