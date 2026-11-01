package commonevents

import "encoding/json"

const (
	showTextCode          = 101
	showChoicesCode       = 102
	changeNameCode        = 320
	changeNicknameCode    = 324
	changeProfileCode     = 325
	messageLineCode       = 401
	scrollingTextLineCode = 405
)

type CommonEventText struct {
	EventID        int
	EventIndex     int
	CommandIndex   int
	CommandCode    int
	ParameterIndex int
	ChoiceIndex    int
	Field          string
	Text           string
}

type commonEvent struct {
	ID   int            `json:"id"`
	List []eventCommand `json:"list"`
}

type eventCommand struct {
	Code       int               `json:"code"`
	Parameters []json.RawMessage `json:"parameters"`
}
