package states

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]StateText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read States.json: %w", err)
	}

	var states []*stateRecord
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, fmt.Errorf("decode States.json: %w", err)
	}

	texts := make([]StateText, 0)
	for stateIndex, state := range states {
		if state == nil {
			continue
		}

		texts = appendStateText(texts, state.ID, stateIndex, "name", state.Name)
		texts = appendStateText(texts, state.ID, stateIndex, "message1", state.Message1)
		texts = appendStateText(texts, state.ID, stateIndex, "message2", state.Message2)
		texts = appendStateText(texts, state.ID, stateIndex, "message3", state.Message3)
		texts = appendStateText(texts, state.ID, stateIndex, "message4", state.Message4)
	}

	return texts, nil
}
