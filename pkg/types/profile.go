package types

// IdosoProfile encapsula dados ricos sobre o usuário para personalização
type IdosoProfile struct {
	ID        int64
	Name      string
	NeuroType []string // Ex: ["tdah", "ansioso"]
	BaseType  int      // Eneatipo (1-9)
}
