package medgemma

// ExamType define os tipos de exames suportados
type ExamType string

const (
	// Cuidados Básicos
	ExamPrescription    ExamType = "prescription"     // Receita médica
	ExamWound           ExamType = "wound"            // Ferida genérica
	ExamLabResult       ExamType = "lab_result"       // Resultado de exame
	ExamMedicationPhoto ExamType = "medication_photo" // Foto de medicamento

	// Doenças Infecciosas e Tropicais
	ExamMalariaSmear ExamType = "malaria_smear" // Esfregaço de malária
	ExamChestXray    ExamType = "chest_xray"    // Raio-X de tórax (TB)
	ExamRapidTest    ExamType = "rapid_test"    // Teste rápido (COVID, HIV, Dengue)

	// Dermatologia
	ExamSkinLesion    ExamType = "skin_lesion"    // Lesão cutânea (Mpox, melanoma)
	ExamPressureUlcer ExamType = "pressure_ulcer" // Úlcera de pressão (escara)
	ExamDiabeticFoot  ExamType = "diabetic_foot"  // Pé diabético

	ExamOther ExamType = "other"
)

// GetPromptForExam retorna o prompt especializado para cada tipo de exame
func GetPromptForExam(examType ExamType, metadata map[string]string) string {
	switch examType {

	case ExamMalariaSmear:
		return `Você é um parasitologista especializado em diagnóstico de malária.
Analise esta lâmina de microscopia de esfregaço sanguíneo.

TAREFA:
1. Identifique e conte anéis de Plasmodium dentro dos eritrócitos
2. Determine a espécie (P. falciparum, P. vivax, P. malariae, P. ovale)
3. Estime a parasitemia (% de células infectadas)
4. Avalie a gravidade da infecção

IMPORTANTE:
- Anéis em forma de anel são característicos de P. falciparum
- Trofozoítos ameboides sugerem P. vivax
- Conte pelo menos 100 células para precisão

Retorne no formato JSON:
{
  "result": "POSITIVE|NEGATIVE",
  "species": "P. falciparum|P. vivax|P. malariae|P. ovale|MIXED|UNKNOWN",
  "parasitemia": "0.5%",
  "infected_cells_count": 5,
  "total_cells_counted": 1000,
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "confidence": 0.95,
  "recommendations": ["Iniciar tratamento com artemisina", "Repetir exame em 48h"]
}`

	case ExamChestXray:
		return `Você é um radiologista especializado em tuberculose.
Analise este raio-X de tórax para triagem de TB.

SINAIS CLÁSSICOS DE TB:
- Infiltrados nos lobos superiores
- Cavitações
- Consolidações
- Padrão miliar (disseminação)
- Linfonodomegalia hilar

TAREFA:
1. Identifique achados sugestivos de TB
2. Calcule probabilidade de TB ativa
3. Classifique urgência de confirmação

IMPORTANTE:
- Esta é uma TRIAGEM, não diagnóstico definitivo
- Casos suspeitos devem fazer baciloscopia/cultura
- Considere diagnósticos diferenciais (pneumonia, câncer)

Retorne no formato JSON:
{
  "tb_probability": "0.85",
  "findings": ["Infiltrado apical direito", "Cavitação 2cm lobo superior"],
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "requires_confirmation": true,
  "urgency": "immediate|within_24h|within_week|routine",
  "differential_diagnosis": ["Pneumonia bacteriana", "Neoplasia pulmonar"],
  "recommendations": ["Baciloscopia urgente", "Cultura de escarro", "Isolamento respiratório"]
}`

	case ExamRapidTest:
		testType := metadata["test_type"]
		if testType == "" {
			testType = "COVID-19"
		}

		return `Você é um especialista em interpretação de testes rápidos (Lateral Flow Assays).
Analise esta imagem de teste rápido para ` + testType + `.

COMPONENTES DO TESTE:
- Linha C (Controle): DEVE estar visível para teste válido
- Linha T (Teste): Presença indica resultado positivo
- Intensidade da linha: Correlaciona com carga viral

TAREFA:
1. Verifique se o teste é válido (linha C presente)
2. Detecte linha T (mesmo se muito fraca)
3. Avalie intensidade da linha T
4. Determine resultado final

IMPORTANTE:
- Linhas muito fracas (ghost lines) ainda são POSITIVAS
- Use filtros de contraste para detectar linhas invisíveis a olho nu
- Teste inválido (sem linha C) deve ser repetido

Retorne no formato JSON:
{
  "test_valid": true,
  "control_line_present": true,
  "test_line_present": true,
  "test_line_intensity": "FRACA|MÉDIA|FORTE",
  "result": "POSITIVE|NEGATIVE|INVALID",
  "confidence": 0.98,
  "recommendations": ["Confirmar com PCR", "Isolamento imediato", "Notificar autoridades"]
}`

	case ExamSkinLesion:
		return `Você é um dermatologista especializado em lesões cutâneas e doenças infecciosas.
Analise esta lesão cutânea.

CRITÉRIOS ABCDE (Melanoma):
- A: Assimetria
- B: Bordas irregulares
- C: Cor variada
- D: Diâmetro > 6mm
- E: Evolução/mudança

SINAIS DE MPOX (Varíola dos Macacos):
- Lesões umbilicadas (centro deprimido)
- Distribuição centrífuga (face, palmas, plantas)
- Todas as lesões no mesmo estágio
- Linfonodomegalia associada

TAREFA:
1. Classifique o tipo de lesão
2. Avalie risco de melanoma (escala ABCDE)
3. Avalie probabilidade de Mpox
4. Detecte sinais de infecção secundária

Retorne no formato JSON:
{
  "lesion_type": "Vesícula|Pústula|Úlcera|Nódulo|Mancha",
  "melanoma_risk": "BAIXO|MÉDIO|ALTO",
  "abcde_score": {"A": true, "B": false, "C": true, "D": false, "E": true},
  "mpox_probability": 0.75,
  "mpox_features": ["Lesões umbilicadas", "Distribuição centrífuga"],
  "infection_signs": ["Eritema perilesional", "Secreção purulenta"],
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "recommendations": ["Biópsia para melanoma", "Isolamento para Mpox", "Teste PCR"]
}`

	case ExamPressureUlcer:
		return `Você é um especialista em cuidados de feridas e úlceras de pressão.
Analise esta úlcera de pressão (escara).

CLASSIFICAÇÃO (NPUAP):
- Estágio 1: Eritema não branqueável
- Estágio 2: Perda parcial da pele (bolha)
- Estágio 3: Perda total da pele (tecido subcutâneo visível)
- Estágio 4: Perda total dos tecidos (músculo/osso expostos)
- Não classificável: Escara/necrose cobrindo a base

TAREFA:
1. Classifique o estágio da úlcera
2. Meça dimensões aproximadas
3. Avalie sinais de infecção
4. Recomende tratamento

Retorne no formato JSON:
{
  "stage": "1|2|3|4|UNCLASSIFIABLE",
  "size": "5cm x 3cm x 2cm (profundidade)",
  "location": "Região sacral",
  "tissue_type": "Granulação|Necrose|Escara|Misto",
  "infection_signs": ["Odor fétido", "Secreção purulenta"],
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "recommendations": ["Desbridamento", "Curativo hidrocoloide", "Mudança de decúbito 2/2h"]
}`

	case ExamDiabeticFoot:
		return `Você é um especialista em pé diabético.
Analise esta imagem de pé de paciente diabético.

CLASSIFICAÇÃO DE WAGNER:
- Grau 0: Pé de risco (sem úlcera)
- Grau 1: Úlcera superficial
- Grau 2: Úlcera profunda (tendão/osso)
- Grau 3: Úlcera com abscesso/osteomielite
- Grau 4: Gangrena localizada
- Grau 5: Gangrena extensa

SINAIS DE ALERTA:
- Coloração escura (necrose)
- Ausência de pulsos
- Temperatura fria
- Odor fétido

TAREFA:
1. Classifique segundo Wagner
2. Avalie sinais de infecção/necrose
3. Determine urgência de intervenção

Retorne no formato JSON:
{
  "wagner_grade": "0|1|2|3|4|5",
  "ulcer_present": true,
  "ulcer_depth": "Superficial|Profunda|Até osso",
  "necrosis_present": true,
  "infection_signs": ["Eritema", "Edema", "Secreção"],
  "vascular_status": "Pulsos presentes|Ausentes",
  "severity": "BAIXO|MÉDIO|ALTO|CRÍTICO",
  "amputation_risk": "BAIXO|MÉDIO|ALTO",
  "recommendations": ["Desbridamento urgente", "Antibiótico IV", "Avaliação vascular"]
}`

	default:
		return `Você é um assistente médico especializado.
Analise esta imagem médica e forneça uma avaliação detalhada.

Retorne no formato JSON com os campos relevantes para o tipo de imagem.`
	}
}
