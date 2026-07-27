package enemies

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]EnemyText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Enemies.json: %w", err)
	}

	var enemies []*enemyRecord
	if err := json.Unmarshal(data, &enemies); err != nil {
		return nil, fmt.Errorf("decode Enemies.json: %w", err)
	}

	texts := make([]EnemyText, 0)
	for enemyIndex, enemy := range enemies {
		if enemy == nil {
			continue
		}

		texts = appendEnemyText(
			texts, enemy.ID, enemyIndex, "name", enemy.Name,
		)
	}

	return texts, nil
}
