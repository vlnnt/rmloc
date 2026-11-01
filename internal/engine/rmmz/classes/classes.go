package classes

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]ClassText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read classes file: %w", err)
	}

	var records []*classRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode classes file: %w", err)
	}

	texts := make([]ClassText, 0)
	for classIndex, record := range records {
		if record == nil {
			continue
		}

		texts = appendClassText(texts, ClassText{
			ClassID:    record.ID,
			ClassIndex: classIndex,
			Field:      "name",
			Text:       record.Name,
		})
	}

	return texts, nil
}
