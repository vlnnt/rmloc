package enemies

type EnemyText struct {
	EnemyID    int
	EnemyIndex int
	Field      string
	Text       string
}

type enemyRecord struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
