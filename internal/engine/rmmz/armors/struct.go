package armors

type ArmorText struct {
	ArmorID    int
	ArmorIndex int
	Field      string
	Text       string
}

type armorRecord struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
