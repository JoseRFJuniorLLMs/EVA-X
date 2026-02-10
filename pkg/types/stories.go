package types

// TherapeuticStory representa uma história metafórica para intervenção
type TherapeuticStory struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	TargetEmotions []string `json:"target_emotions"` // Ex: ["ansiedade", "solidão"]
	Archetype      string   `json:"archetype"`       // Ex: "O Sábio", "O Herói"
	Moral          string   `json:"moral"`
	Tags           []string `json:"tags"`
	MinAge         int      `json:"min_age"`
	NeuroTypes     []string `json:"neuro_types"` // Ex: ["autismo", "tdah"] support
}
