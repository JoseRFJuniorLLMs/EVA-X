package integration

import (
	"fmt"
	"time"
)

// ============================================================================
// HL7 FHIR R4 ADAPTER
// ============================================================================
// Converte recursos EVA-Mind para FHIR R4 (padrão internacional de interoperabilidade)
// Referência: https://www.hl7.org/fhir/R4/

// ============================================================================
// FHIR PATIENT RESOURCE
// ============================================================================

type FHIRPatient struct {
	ResourceType string                   `json:"resourceType"` // "Patient"
	ID           string                   `json:"id"`
	Meta         *FHIRMeta                `json:"meta,omitempty"`
	Identifier   []FHIRIdentifier         `json:"identifier,omitempty"`
	Active       bool                     `json:"active"`
	Name         []FHIRHumanName          `json:"name"`
	Telecom      []FHIRContactPoint       `json:"telecom,omitempty"`
	Gender       string                   `json:"gender"` // "male" | "female" | "other" | "unknown"
	BirthDate    string                   `json:"birthDate"` // YYYY-MM-DD
	Address      []FHIRAddress            `json:"address,omitempty"`
	Contact      []FHIRPatientContact     `json:"contact,omitempty"`
	Extension    []FHIRExtension          `json:"extension,omitempty"`
}

type FHIRMeta struct {
	VersionID   string    `json:"versionId,omitempty"`
	LastUpdated time.Time `json:"lastUpdated"`
	Source      string    `json:"source,omitempty"` // "EVA-Mind"
	Profile     []string  `json:"profile,omitempty"`
}

type FHIRIdentifier struct {
	Use    string         `json:"use,omitempty"` // "usual" | "official" | "temp" | "secondary"
	Type   *FHIRCodeableConcept `json:"type,omitempty"`
	System string         `json:"system"` // "https://eva-mind.com/patient-id"
	Value  string         `json:"value"`
}

type FHIRHumanName struct {
	Use    string   `json:"use,omitempty"` // "usual" | "official" | "nickname"
	Text   string   `json:"text,omitempty"`
	Family string   `json:"family,omitempty"`
	Given  []string `json:"given,omitempty"`
}

type FHIRContactPoint struct {
	System string `json:"system"` // "phone" | "email"
	Value  string `json:"value"`
	Use    string `json:"use,omitempty"` // "home" | "work" | "mobile"
}

type FHIRAddress struct {
	Use        string   `json:"use,omitempty"` // "home" | "work"
	Type       string   `json:"type,omitempty"` // "postal" | "physical"
	Text       string   `json:"text,omitempty"`
	Line       []string `json:"line,omitempty"`
	City       string   `json:"city,omitempty"`
	State      string   `json:"state,omitempty"`
	PostalCode string   `json:"postalCode,omitempty"`
	Country    string   `json:"country,omitempty"`
}

type FHIRPatientContact struct {
	Relationship []FHIRCodeableConcept `json:"relationship,omitempty"`
	Name         *FHIRHumanName        `json:"name,omitempty"`
	Telecom      []FHIRContactPoint    `json:"telecom,omitempty"`
}

type FHIRCodeableConcept struct {
	Coding []FHIRCoding `json:"coding,omitempty"`
	Text   string       `json:"text,omitempty"`
}

type FHIRCoding struct {
	System  string `json:"system,omitempty"`
	Code    string `json:"code,omitempty"`
	Display string `json:"display,omitempty"`
}

type FHIRExtension struct {
	URL   string      `json:"url"`
	Value interface{} `json:"valueString,omitempty"` // pode ser valueString, valueInteger, etc.
}

// Converter PatientDTO para FHIR Patient
func PatientDTOToFHIR(dto *PatientDTO) *FHIRPatient {
	patient := &FHIRPatient{
		ResourceType: "Patient",
		ID:           fmt.Sprintf("%d", dto.ID),
		Meta: &FHIRMeta{
			LastUpdated: dto.UpdatedAt,
			Source:      "EVA-Mind",
		},
		Identifier: []FHIRIdentifier{
			{
				Use:    "official",
				System: "https://eva-mind.com/patient-id",
				Value:  fmt.Sprintf("%d", dto.ID),
			},
		},
		Active: true,
		Name: []FHIRHumanName{
			{
				Use:  "official",
				Text: dto.Name,
			},
		},
		Gender:    mapGenderToFHIR(dto.Gender),
		BirthDate: dto.DateOfBirth,
	}

	// Adicionar contatos
	if dto.Email != nil {
		patient.Telecom = append(patient.Telecom, FHIRContactPoint{
			System: "email",
			Value:  *dto.Email,
			Use:    "home",
		})
	}

	if dto.Phone != nil {
		patient.Telecom = append(patient.Telecom, FHIRContactPoint{
			System: "phone",
			Value:  *dto.Phone,
			Use:    "mobile",
		})
	}

	// Adicionar endereço
	if dto.Address != nil {
		patient.Address = []FHIRAddress{
			{
				Use:  "home",
				Type: "physical",
				Text: *dto.Address,
			},
		}
	}

	return patient
}

func mapGenderToFHIR(gender string) string {
	switch gender {
	case "M", "Masculino", "male":
		return "male"
	case "F", "Feminino", "female":
		return "female"
	default:
		return "unknown"
	}
}

// ============================================================================
// FHIR OBSERVATION RESOURCE
// ============================================================================
// Usado para assessments (PHQ-9, GAD-7, etc.) e sinais vitais

type FHIRObservation struct {
	ResourceType    string                 `json:"resourceType"` // "Observation"
	ID              string                 `json:"id"`
	Meta            *FHIRMeta              `json:"meta,omitempty"`
	Status          string                 `json:"status"` // "registered" | "preliminary" | "final" | "amended"
	Category        []FHIRCodeableConcept  `json:"category,omitempty"`
	Code            FHIRCodeableConcept    `json:"code"`
	Subject         FHIRReference          `json:"subject"`
	EffectiveDateTime *time.Time           `json:"effectiveDateTime,omitempty"`
	Issued          *time.Time             `json:"issued,omitempty"`
	ValueQuantity   *FHIRQuantity          `json:"valueQuantity,omitempty"`
	ValueInteger    *int                   `json:"valueInteger,omitempty"`
	ValueString     *string                `json:"valueString,omitempty"`
	Interpretation  []FHIRCodeableConcept  `json:"interpretation,omitempty"`
	Note            []FHIRAnnotation       `json:"note,omitempty"`
	Component       []FHIRObservationComponent `json:"component,omitempty"`
}

type FHIRReference struct {
	Reference string  `json:"reference"` // "Patient/123"
	Display   *string `json:"display,omitempty"`
}

type FHIRQuantity struct {
	Value  float64 `json:"value"`
	Unit   string  `json:"unit,omitempty"`
	System string  `json:"system,omitempty"`
	Code   string  `json:"code,omitempty"`
}

type FHIRAnnotation struct {
	Text string `json:"text"`
}

type FHIRObservationComponent struct {
	Code          FHIRCodeableConcept `json:"code"`
	ValueQuantity *FHIRQuantity       `json:"valueQuantity,omitempty"`
	ValueInteger  *int                `json:"valueInteger,omitempty"`
}

// Converter AssessmentDTO (PHQ-9) para FHIR Observation
func PHQ9ToFHIR(dto *AssessmentDTO) *FHIRObservation {
	obs := &FHIRObservation{
		ResourceType: "Observation",
		ID:           dto.ID,
		Meta: &FHIRMeta{
			LastUpdated: dto.CreatedAt,
			Source:      "EVA-Mind",
		},
		Status: mapAssessmentStatusToFHIR(dto.Status),
		Category: []FHIRCodeableConcept{
			{
				Coding: []FHIRCoding{
					{
						System:  "http://terminology.hl7.org/CodeSystem/observation-category",
						Code:    "survey",
						Display: "Survey",
					},
				},
			},
		},
		Code: FHIRCodeableConcept{
			Coding: []FHIRCoding{
				{
					System:  "http://loinc.org",
					Code:    "44249-1",
					Display: "PHQ-9 quick depression assessment panel",
				},
			},
			Text: "Patient Health Questionnaire-9 (PHQ-9)",
		},
		Subject: FHIRReference{
			Reference: fmt.Sprintf("Patient/%d", dto.PatientID),
		},
		EffectiveDateTime: dto.CompletedAt,
		Issued:            dto.CompletedAt,
	}

	if dto.TotalScore != nil {
		obs.ValueInteger = dto.TotalScore

		// Adicionar interpretação baseada no score
		if *dto.TotalScore >= 20 {
			obs.Interpretation = []FHIRCodeableConcept{
				{
					Coding: []FHIRCoding{
						{
							System:  "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation",
							Code:    "H",
							Display: "High",
						},
					},
					Text: "Severe depression",
				},
			}
		} else if *dto.TotalScore >= 15 {
			obs.Interpretation = []FHIRCodeableConcept{
				{
					Text: "Moderately severe depression",
				},
			}
		} else if *dto.TotalScore >= 10 {
			obs.Interpretation = []FHIRCodeableConcept{
				{
					Text: "Moderate depression",
				},
			}
		} else if *dto.TotalScore >= 5 {
			obs.Interpretation = []FHIRCodeableConcept{
				{
					Text: "Mild depression",
				},
			}
		} else {
			obs.Interpretation = []FHIRCodeableConcept{
				{
					Coding: []FHIRCoding{
						{
							System:  "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation",
							Code:    "N",
							Display: "Normal",
						},
					},
					Text: "Minimal depression",
				},
			}
		}
	}

	// Adicionar notas se houver
	if dto.Notes != nil {
		obs.Note = []FHIRAnnotation{
			{Text: *dto.Notes},
		}
	}

	return obs
}

func mapAssessmentStatusToFHIR(status string) string {
	switch status {
	case "completed":
		return "final"
	case "in_progress":
		return "preliminary"
	case "pending":
		return "registered"
	default:
		return "registered"
	}
}

// ============================================================================
// FHIR QUESTIONNAIRE RESPONSE
// ============================================================================
// Alternativa para representar respostas de questionários

type FHIRQuestionnaireResponse struct {
	ResourceType string                              `json:"resourceType"` // "QuestionnaireResponse"
	ID           string                              `json:"id"`
	Meta         *FHIRMeta                           `json:"meta,omitempty"`
	Status       string                              `json:"status"` // "in-progress" | "completed"
	Subject      FHIRReference                       `json:"subject"`
	Authored     time.Time                           `json:"authored"`
	Author       *FHIRReference                      `json:"author,omitempty"`
	Item         []FHIRQuestionnaireResponseItem     `json:"item,omitempty"`
}

type FHIRQuestionnaireResponseItem struct {
	LinkID string                              `json:"linkId"`
	Text   string                              `json:"text,omitempty"`
	Answer []FHIRQuestionnaireResponseAnswer   `json:"answer,omitempty"`
}

type FHIRQuestionnaireResponseAnswer struct {
	ValueInteger *int    `json:"valueInteger,omitempty"`
	ValueString  *string `json:"valueString,omitempty"`
	ValueBoolean *bool   `json:"valueBoolean,omitempty"`
}

// ============================================================================
// FHIR CONDITION RESOURCE
// ============================================================================
// Diagnósticos e condições de saúde

type FHIRCondition struct {
	ResourceType    string                `json:"resourceType"` // "Condition"
	ID              string                `json:"id"`
	Meta            *FHIRMeta             `json:"meta,omitempty"`
	ClinicalStatus  FHIRCodeableConcept   `json:"clinicalStatus"`
	VerificationStatus FHIRCodeableConcept `json:"verificationStatus,omitempty"`
	Category        []FHIRCodeableConcept `json:"category,omitempty"`
	Severity        *FHIRCodeableConcept  `json:"severity,omitempty"`
	Code            FHIRCodeableConcept   `json:"code"`
	Subject         FHIRReference         `json:"subject"`
	OnsetDateTime   *time.Time            `json:"onsetDateTime,omitempty"`
	RecordedDate    *time.Time            `json:"recordedDate,omitempty"`
	Note            []FHIRAnnotation      `json:"note,omitempty"`
}

// ============================================================================
// FHIR MEDICATION REQUEST
// ============================================================================

type FHIRMedicationRequest struct {
	ResourceType       string               `json:"resourceType"` // "MedicationRequest"
	ID                 string               `json:"id"`
	Meta               *FHIRMeta            `json:"meta,omitempty"`
	Status             string               `json:"status"` // "active" | "completed" | "stopped"
	Intent             string               `json:"intent"` // "order" | "plan"
	MedicationCodeableConcept *FHIRCodeableConcept `json:"medicationCodeableConcept,omitempty"`
	Subject            FHIRReference        `json:"subject"`
	AuthoredOn         *time.Time           `json:"authoredOn,omitempty"`
	DosageInstruction  []FHIRDosage         `json:"dosageInstruction,omitempty"`
}

type FHIRDosage struct {
	Text   string `json:"text,omitempty"`
	Timing *FHIRTiming `json:"timing,omitempty"`
}

type FHIRTiming struct {
	Code FHIRCodeableConcept `json:"code,omitempty"`
}

// ============================================================================
// FHIR BUNDLE (COLEÇÃO DE RECURSOS)
// ============================================================================

type FHIRBundle struct {
	ResourceType string            `json:"resourceType"` // "Bundle"
	ID           string            `json:"id,omitempty"`
	Meta         *FHIRMeta         `json:"meta,omitempty"`
	Type         string            `json:"type"` // "collection" | "searchset" | "transaction"
	Total        *int              `json:"total,omitempty"`
	Link         []FHIRBundleLink  `json:"link,omitempty"`
	Entry        []FHIRBundleEntry `json:"entry"`
}

type FHIRBundleLink struct {
	Relation string `json:"relation"` // "self" | "next" | "previous"
	URL      string `json:"url"`
}

type FHIRBundleEntry struct {
	FullURL  string      `json:"fullUrl,omitempty"`
	Resource interface{} `json:"resource"` // Pode ser Patient, Observation, etc.
}

// Criar bundle com paciente + observações
func CreatePatientBundle(patient *FHIRPatient, observations []*FHIRObservation) *FHIRBundle {
	bundle := &FHIRBundle{
		ResourceType: "Bundle",
		Type:         "collection",
		Entry:        []FHIRBundleEntry{},
	}

	// Adicionar paciente
	bundle.Entry = append(bundle.Entry, FHIRBundleEntry{
		FullURL:  fmt.Sprintf("Patient/%s", patient.ID),
		Resource: patient,
	})

	// Adicionar observações
	for _, obs := range observations {
		bundle.Entry = append(bundle.Entry, FHIRBundleEntry{
			FullURL:  fmt.Sprintf("Observation/%s", obs.ID),
			Resource: obs,
		})
	}

	total := len(bundle.Entry)
	bundle.Total = &total

	return bundle
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// Converter para JSON FHIR
func ToFHIRJSON(resource interface{}) (string, error) {
	return ToJSONCompact(resource)
}

// Validar se recurso FHIR está bem formado (básico)
func ValidateFHIRResource(resource interface{}) error {
	// Implementação básica - você pode expandir com validação completa
	_, err := ToFHIRJSON(resource)
	return err
}
