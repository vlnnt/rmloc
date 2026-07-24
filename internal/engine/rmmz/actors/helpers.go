package actors

import "strings"

func appendActorText(
	texts []ActorText,
	actorID, actorIndex int,
	field, text string,
) []ActorText {
	if strings.TrimSpace(text) == "" {
		return texts
	}

	return append(texts, ActorText{
		ActorID:    actorID,
		ActorIndex: actorIndex,
		Field:      field,
		Text:       text,
	})
}
