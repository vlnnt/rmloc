package actors

type ActorText struct {
	ActorID    int
	ActorIndex int
	Field      string
	Text       string
}

type actorRecord struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Nickname string `json:"nickname"`
	Profile  string `json:"profile"`
}
