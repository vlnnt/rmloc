package system

type SystemText struct {
	Field string
	Index int
	Key   string
	Text  string
}

type systemRecord struct {
	ArmorTypes  []string `json:"armorTypes"`
	Elements    []string `json:"elements"`
	EquipTypes  []string `json:"equipTypes"`
	SkillTypes  []string `json:"skillTypes"`
	WeaponTypes []string `json:"weaponTypes"`
	Terms       struct {
		Basic    []string          `json:"basic"`
		Commands []string          `json:"commands"`
		Params   []string          `json:"params"`
		Messages map[string]string `json:"messages"`
	} `json:"terms"`
}
