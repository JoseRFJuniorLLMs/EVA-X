package testutil

// TestFixtures contém dados de teste comuns
type TestFixtures struct{}

// NewTestFixtures cria fixtures de teste
func NewTestFixtures() *TestFixtures {
	return &TestFixtures{}
}

// ======================================================
// IDOSOS (Pacientes) de Teste
// ======================================================

// TestIdoso dados de um idoso de teste
type TestIdoso struct {
	ID       int64
	Nome     string
	CPF      string
	Email    string
	Telefone string
}

// GetTestIdoso retorna um idoso padrão de teste
func (f *TestFixtures) GetTestIdoso() TestIdoso {
	return TestIdoso{
		ID:       999,
		Nome:     "Maria Teste Silva",
		CPF:      "12345678901",
		Email:    "maria.teste@example.com",
		Telefone: "+5511999999999",
	}
}

// GetCreatorIdoso retorna o idoso do criador (Jose)
func (f *TestFixtures) GetCreatorIdoso() TestIdoso {
	return TestIdoso{
		ID:       1,
		Nome:     "Jose R F Junior",
		CPF:      "64525430249",
		Email:    "jose@example.com",
		Telefone: "+5511888888888",
	}
}

// ======================================================
// Contatos de Emergência de Teste
// ======================================================

// TestEmergencyContact contato de emergência de teste
type TestEmergencyContact struct {
	Nome      string
	Telefone  string
	Email     string
	Relacao   string
	Prioridade int
}

// GetTestEmergencyContacts retorna contatos de emergência de teste
func (f *TestFixtures) GetTestEmergencyContacts() []TestEmergencyContact {
	return []TestEmergencyContact{
		{
			Nome:       "Filho Teste",
			Telefone:   "+5511988887777",
			Email:      "filho@example.com",
			Relacao:    "filho",
			Prioridade: 1,
		},
		{
			Nome:       "Filha Teste",
			Telefone:   "+5511977776666",
			Email:      "filha@example.com",
			Relacao:    "filha",
			Prioridade: 2,
		},
		{
			Nome:       "Médico Teste",
			Telefone:   "+5511966665555",
			Email:      "medico@example.com",
			Relacao:    "medico",
			Prioridade: 3,
		},
	}
}

// ======================================================
// Respostas C-SSRS de Teste
// ======================================================

// CSSRSTestCase caso de teste para C-SSRS
type CSSRSTestCase struct {
	Name           string
	TriggerPhrase  string
	Responses      []int // 0=Não, 1=Sim para cada pergunta
	ExpectedRisk   string // none, low, moderate, high, imminent
	ExpectedAlert  bool
}

// GetCSSRSTestCases retorna casos de teste para C-SSRS
func (f *TestFixtures) GetCSSRSTestCases() []CSSRSTestCase {
	return []CSSRSTestCase{
		{
			Name:          "Sem risco - todas negativas",
			TriggerPhrase: "às vezes fico triste",
			Responses:     []int{0, 0, 0, 0, 0, 0},
			ExpectedRisk:  "none",
			ExpectedAlert: false,
		},
		{
			Name:          "Risco baixo - ideação passiva",
			TriggerPhrase: "seria melhor se eu não existisse",
			Responses:     []int{1, 0, 0, 0, 0, 0}, // Apenas Q1 positiva
			ExpectedRisk:  "low",
			ExpectedAlert: false,
		},
		{
			Name:          "Risco moderado - ideação ativa sem plano",
			TriggerPhrase: "pensei em me machucar",
			Responses:     []int{1, 1, 0, 0, 0, 0}, // Q1 e Q2 positivas
			ExpectedRisk:  "moderate",
			ExpectedAlert: true,
		},
		{
			Name:          "Risco alto - com plano",
			TriggerPhrase: "já pensei como fazer",
			Responses:     []int{1, 1, 1, 0, 0, 0}, // Q1, Q2, Q3 positivas
			ExpectedRisk:  "high",
			ExpectedAlert: true,
		},
		{
			Name:          "Risco iminente - com intenção",
			TriggerPhrase: "vou fazer hoje",
			Responses:     []int{1, 1, 1, 1, 0, 0}, // Q1-Q4 positivas
			ExpectedRisk:  "imminent",
			ExpectedAlert: true,
		},
		{
			Name:          "Risco iminente - tentativa recente",
			TriggerPhrase: "tentei semana passada",
			Responses:     []int{1, 1, 1, 1, 1, 0}, // Q1-Q5 positivas
			ExpectedRisk:  "imminent",
			ExpectedAlert: true,
		},
	}
}

// ======================================================
// Respostas PHQ-9 de Teste
// ======================================================

// PHQ9TestCase caso de teste para PHQ-9
type PHQ9TestCase struct {
	Name            string
	Responses       []int // 0-3 para cada pergunta (9 perguntas)
	ExpectedScore   int
	ExpectedLevel   string // minimal, mild, moderate, moderately_severe, severe
	Question9Value  int    // Valor da Q9 (ideação suicida)
}

// GetPHQ9TestCases retorna casos de teste para PHQ-9
func (f *TestFixtures) GetPHQ9TestCases() []PHQ9TestCase {
	return []PHQ9TestCase{
		{
			Name:           "Depressão mínima",
			Responses:      []int{0, 0, 1, 0, 0, 0, 0, 0, 0}, // Score = 1
			ExpectedScore:  1,
			ExpectedLevel:  "minimal",
			Question9Value: 0,
		},
		{
			Name:           "Depressão leve",
			Responses:      []int{1, 1, 1, 1, 1, 0, 0, 0, 0}, // Score = 5
			ExpectedScore:  5,
			ExpectedLevel:  "mild",
			Question9Value: 0,
		},
		{
			Name:           "Depressão moderada",
			Responses:      []int{2, 2, 1, 1, 1, 1, 1, 1, 0}, // Score = 10
			ExpectedScore:  10,
			ExpectedLevel:  "moderate",
			Question9Value: 0,
		},
		{
			Name:           "Depressão moderadamente severa",
			Responses:      []int{2, 2, 2, 2, 2, 2, 2, 1, 0}, // Score = 15
			ExpectedScore:  15,
			ExpectedLevel:  "moderately_severe",
			Question9Value: 0,
		},
		{
			Name:           "Depressão severa",
			Responses:      []int{3, 3, 3, 3, 2, 2, 2, 2, 0}, // Score = 20
			ExpectedScore:  20,
			ExpectedLevel:  "severe",
			Question9Value: 0,
		},
		{
			Name:           "Com ideação suicida (Q9 positiva)",
			Responses:      []int{2, 2, 2, 1, 1, 1, 1, 1, 2}, // Score = 13, Q9 = 2
			ExpectedScore:  13,
			ExpectedLevel:  "moderate",
			Question9Value: 2,
		},
	}
}

// ======================================================
// Respostas GAD-7 de Teste
// ======================================================

// GAD7TestCase caso de teste para GAD-7
type GAD7TestCase struct {
	Name          string
	Responses     []int // 0-3 para cada pergunta (7 perguntas)
	ExpectedScore int
	ExpectedLevel string // minimal, mild, moderate, severe
}

// GetGAD7TestCases retorna casos de teste para GAD-7
func (f *TestFixtures) GetGAD7TestCases() []GAD7TestCase {
	return []GAD7TestCase{
		{
			Name:          "Ansiedade mínima",
			Responses:     []int{0, 1, 0, 0, 0, 1, 0}, // Score = 2
			ExpectedScore: 2,
			ExpectedLevel: "minimal",
		},
		{
			Name:          "Ansiedade leve",
			Responses:     []int{1, 1, 1, 1, 1, 1, 0}, // Score = 6
			ExpectedScore: 6,
			ExpectedLevel: "mild",
		},
		{
			Name:          "Ansiedade moderada",
			Responses:     []int{2, 2, 1, 1, 2, 2, 1}, // Score = 11
			ExpectedScore: 11,
			ExpectedLevel: "moderate",
		},
		{
			Name:          "Ansiedade severa",
			Responses:     []int{3, 3, 2, 2, 2, 3, 2}, // Score = 17
			ExpectedScore: 17,
			ExpectedLevel: "severe",
		},
	}
}

// ======================================================
// Frases de Gatilho de Teste
// ======================================================

// GetSuicidalTriggerPhrases retorna frases que devem acionar C-SSRS
func (f *TestFixtures) GetSuicidalTriggerPhrases() []string {
	return []string{
		"não quero mais viver",
		"seria melhor se eu não existisse",
		"quero acabar com tudo",
		"penso em me machucar",
		"não aguento mais",
		"vou dar um fim nisso",
		"queria dormir e não acordar",
		"minha família estaria melhor sem mim",
	}
}

// GetNonSuicidalPhrases retorna frases que NÃO devem acionar C-SSRS
func (f *TestFixtures) GetNonSuicidalPhrases() []string {
	return []string{
		"estou muito triste hoje",
		"me sinto sozinho",
		"sinto falta do meu marido",
		"não tenho vontade de sair",
		"estou cansado",
		"não dormi bem",
		"estou preocupado com a saúde",
	}
}
