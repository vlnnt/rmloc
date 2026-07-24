package maps

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

type MapText struct {
	EventID        int
	EventIndex     int
	PageIndex      int
	CommandIndex   int
	CommandCode    int
	ParameterIndex int
	ChoiceIndex    int
	Field          string
	Text           string
}

type mapRecord struct {
	DisplayName string      `json:"displayName"`
	Events      []*mapEvent `json:"events"`
}

type mapEvent struct {
	ID    int       `json:"id"`
	Pages []mapPage `json:"pages"`
}

type mapPage struct {
	List []eventCommand `json:"list"`
}

type eventCommand struct {
	Code       int               `json:"code"`
	Parameters []json.RawMessage `json:"parameters"`
}
