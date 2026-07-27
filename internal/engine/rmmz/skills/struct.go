package skills

type SkillText struct {
	SkillID    int
	SkillIndex int
	Field      string
	Text       string
}

type skillRecord struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Message1    string `json:"message1"`
	Message2    string `json:"message2"`
}
