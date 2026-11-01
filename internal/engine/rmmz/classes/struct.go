package classes

type ClassText struct {
	ClassID    int
	ClassIndex int
	Field      string
	Text       string
}

type classRecord struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
