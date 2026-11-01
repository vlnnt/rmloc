package actors

import (
	"encoding/json"
	"fmt"
	"os"
)

func Extract(
	filePath string,
) ([]ActorText, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read Actors.json: %w", err)
	}

	var actors []*actorRecord
	if err := json.Unmarshal(data, &actors); err != nil {
		return nil, fmt.Errorf("decode Actors.json: %w", err)
	}

	texts := make([]ActorText, 0)
	for actorIndex, actor := range actors {
		if actor == nil {
			continue
		}

		texts = appendActorText(texts, actor.ID, actorIndex, "name", actor.Name)
		texts = appendActorText(texts, actor.ID, actorIndex, "nickname", actor.Nickname)
		texts = appendActorText(texts, actor.ID, actorIndex, "profile", actor.Profile)
	}

	return texts, nil
}
