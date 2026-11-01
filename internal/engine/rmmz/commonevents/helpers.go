package commonevents

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractCommandTexts(
	eventID, eventIndex, commandIndex int,
	command eventCommand,
) ([]CommonEventText, error) {
	switch command.Code {
	case showTextCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 4, "speakerName")

	case messageLineCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 0, "message")

	case scrollingTextLineCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 0, "scrollingText")

	case changeNameCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 1, "actorName")

	case changeNicknameCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 1, "actorNickname")

	case changeProfileCode:
		return extractStringParameter(eventID, eventIndex, commandIndex, command, 1, "actorProfile")

	case showChoicesCode:
		return extractChoices(eventID, eventIndex, commandIndex, command)

	default:
		return nil, nil
	}
}

func extractStringParameter(
	eventID, eventIndex, commandIndex int,
	command eventCommand, parameterIndex int, field string,
) ([]CommonEventText, error) {
	if parameterIndex >= len(command.Parameters) {
		return nil, nil
	}

	var text string
	if err := json.Unmarshal(
		command.Parameters[parameterIndex], &text); err != nil {
		return nil, fmt.Errorf(
			"decode event %d command %d parameter %d: %w",
			eventID,
			commandIndex,
			parameterIndex,
			err,
		)
	}

	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	return []CommonEventText{
		{
			EventID:        eventID,
			EventIndex:     eventIndex,
			CommandIndex:   commandIndex,
			CommandCode:    command.Code,
			ParameterIndex: parameterIndex,
			ChoiceIndex:    -1,
			Field:          field,
			Text:           text,
		},
	}, nil
}

func extractChoices(
	eventID, eventIndex, commandIndex int,
	command eventCommand,
) ([]CommonEventText, error) {
	const parameterIndex = 0
	if len(command.Parameters) == 0 {
		return nil, nil
	}

	var choices []string
	if err := json.Unmarshal(
		command.Parameters[parameterIndex], &choices); err != nil {
		return nil, fmt.Errorf(
			"decode choices in event %d command %d: %w",
			eventID,
			commandIndex,
			err,
		)
	}

	texts := make([]CommonEventText, 0, len(choices))
	for choiceIndex, choice := range choices {
		if strings.TrimSpace(choice) == "" {
			continue
		}

		texts = append(texts, CommonEventText{
			EventID:        eventID,
			EventIndex:     eventIndex,
			CommandIndex:   commandIndex,
			CommandCode:    command.Code,
			ParameterIndex: parameterIndex,
			ChoiceIndex:    choiceIndex,
			Field:          "choice",
			Text:           choice,
		})
	}

	return texts, nil
}
