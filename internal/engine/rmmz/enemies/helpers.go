package enemies

import "strings"

func appendEnemyText(
	texts []EnemyText,
	enemyID, enemyIndex int,
	field, text string,
) []EnemyText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, EnemyText{
		EnemyID:    enemyID,
		EnemyIndex: enemyIndex,
		Field:      field,
		Text:       text,
	})
}
