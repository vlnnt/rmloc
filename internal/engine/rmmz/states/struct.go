package states

type StateText struct {
	StateID    int
	StateIndex int
	Field      string
	Text       string
}

type stateRecord struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Message1 string `json:"message1"`
	Message2 string `json:"message2"`
	Message3 string `json:"message3"`
	Message4 string `json:"message4"`
}
