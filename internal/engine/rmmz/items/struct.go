package items

type ItemText struct {
	ItemID    int
	ItemIndex int
	Field     string
	Text      string
}

type itemRecord struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
