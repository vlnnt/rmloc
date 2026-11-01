package troops

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

type TroopText struct {
	TroopID        int
	TroopIndex     int
	PageIndex      int
	CommandIndex   int
	CommandCode    int
	ParameterIndex int
	ChoiceIndex    int
	Field          string
	Text           string
}

type troopRecord struct {
	ID    int         `json:"id"`
	Pages []troopPage `json:"pages"`
}

type troopPage struct {
	List []eventCommand `json:"list"`
}

type eventCommand struct {
	Code       int               `json:"code"`
	Parameters []json.RawMessage `json:"parameters"`
}
