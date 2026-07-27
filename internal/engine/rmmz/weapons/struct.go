package weapons

type WeaponText struct {
	WeaponID    int
	WeaponIndex int
	Field       string
	Text        string
}

type weaponRecord struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
