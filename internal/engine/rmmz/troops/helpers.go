package troops

import (
	"encoding/json"
	"fmt"
	"strings"
)

func extractCommandTexts(
	troopID, troopIndex, pageIndex, commandIndex int,
	command eventCommand,
) ([]TroopText, error) {
	switch command.Code {
	case showTextCode:
		if len(command.Parameters) <= 4 {
			return nil, nil
		}

		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 4, "speakerName")

	case messageLineCode:
		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 0, "message")

	case scrollingTextLineCode:
		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 0, "scrollingText")

	case changeNameCode:
		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 1, "actorName")

	case changeNicknameCode:
		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 1, "actorNickname")

	case changeProfileCode:
		return extractStringParameter(troopID, troopIndex, pageIndex, commandIndex, command, 1, "actorProfile")

	case showChoicesCode:
		return extractChoices(troopID, troopIndex, pageIndex, commandIndex, command)
	}

	return nil, nil
}

func extractStringParameter(
	troopID, troopIndex, pageIndex, commandIndex int,
	command eventCommand,
	parameterIndex int,
	field string,
) ([]TroopText, error) {
	if parameterIndex >= len(command.Parameters) {
		return nil, nil
	}

	var value string
	if err := json.Unmarshal(
		command.Parameters[parameterIndex],
		&value,
	); err != nil {
		return nil, fmt.Errorf(
			"decode troop %d page %d command %d parameter %d: %w",
			troopID, pageIndex, commandIndex, parameterIndex, err,
		)
	}

	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	return []TroopText{
		{
			TroopID:        troopID,
			TroopIndex:     troopIndex,
			PageIndex:      pageIndex,
			CommandIndex:   commandIndex,
			CommandCode:    command.Code,
			ParameterIndex: parameterIndex,
			ChoiceIndex:    -1,
			Field:          field,
			Text:           value,
		},
	}, nil
}

func extractChoices(
	troopID, troopIndex, pageIndex, commandIndex int,
	command eventCommand,
) ([]TroopText, error) {
	if len(command.Parameters) == 0 {
		return nil, nil
	}

	var choices []string
	if err := json.Unmarshal(command.Parameters[0], &choices); err != nil {
		return nil, fmt.Errorf(
			"decode troop %d page %d command %d choices: %w",
			troopID, pageIndex, commandIndex, err,
		)
	}

	texts := make([]TroopText, 0, len(choices))
	for choiceIndex, choice := range choices {
		if strings.TrimSpace(choice) == "" {
			continue
		}

		texts = append(texts, TroopText{
			TroopID:        troopID,
			TroopIndex:     troopIndex,
			PageIndex:      pageIndex,
			CommandIndex:   commandIndex,
			CommandCode:    command.Code,
			ParameterIndex: 0,
			ChoiceIndex:    choiceIndex,
			Field:          "choice",
			Text:           choice,
		})
	}

	return texts, nil
}
