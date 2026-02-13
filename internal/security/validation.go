package security

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// CPF: exatamente 11 dígitos numéricos
	cpfRegex = regexp.MustCompile(`^\d{11}$`)

	// Email: validação básica RFC 5322 simplificada
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// Roles permitidos
	validRoles = map[string]bool{
		"admin":     true,
		"cuidador":  true,
		"idoso":     true,
		"familiar":  true,
	}
)

// ValidateCPF valida formato e dígitos verificadores do CPF
func ValidateCPF(cpf string) error {
	// Remover caracteres não numéricos
	cpf = strings.ReplaceAll(cpf, ".", "")
	cpf = strings.ReplaceAll(cpf, "-", "")
	cpf = strings.TrimSpace(cpf)

	// Verificar formato
	if !cpfRegex.MatchString(cpf) {
		return fmt.Errorf("CPF must contain exactly 11 digits")
	}

	// Verificar se todos os dígitos são iguais (CPF inválido)
	allSame := true
	for i := 1; i < len(cpf); i++ {
		if cpf[i] != cpf[0] {
			allSame = false
			break
		}
	}
	if allSame {
		return fmt.Errorf("invalid CPF")
	}

	// Validar dígitos verificadores
	if !validateCPFChecksum(cpf) {
		return fmt.Errorf("invalid CPF checksum")
	}

	return nil
}

// validateCPFChecksum valida os dígitos verificadores do CPF
func validateCPFChecksum(cpf string) bool {
	// Converter para slice de ints
	digits := make([]int, 11)
	for i, c := range cpf {
		digits[i] = int(c - '0')
	}

	// Validar primeiro dígito verificador
	sum := 0
	for i := 0; i < 9; i++ {
		sum += digits[i] * (10 - i)
	}
	remainder := sum % 11
	expectedDigit1 := 0
	if remainder >= 2 {
		expectedDigit1 = 11 - remainder
	}
	if digits[9] != expectedDigit1 {
		return false
	}

	// Validar segundo dígito verificador
	sum = 0
	for i := 0; i < 10; i++ {
		sum += digits[i] * (11 - i)
	}
	remainder = sum % 11
	expectedDigit2 := 0
	if remainder >= 2 {
		expectedDigit2 = 11 - remainder
	}
	if digits[10] != expectedDigit2 {
		return false
	}

	return true
}

// ValidateEmail valida formato de email
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)

	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}

	if len(email) > 254 {
		return fmt.Errorf("email too long")
	}

	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// ValidateRole valida se o role é permitido
func ValidateRole(role string) error {
	role = strings.ToLower(strings.TrimSpace(role))

	if role == "" {
		return fmt.Errorf("role cannot be empty")
	}

	if !validRoles[role] {
		return fmt.Errorf("invalid role: must be one of admin, cuidador, idoso, familiar")
	}

	return nil
}

// SanitizeCPF remove caracteres não numéricos do CPF
func SanitizeCPF(cpf string) string {
	cpf = strings.ReplaceAll(cpf, ".", "")
	cpf = strings.ReplaceAll(cpf, "-", "")
	cpf = strings.ReplaceAll(cpf, " ", "")
	return cpf
}

// ValidateName valida nome de pessoa
func ValidateName(name string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) < 2 {
		return fmt.Errorf("name too short")
	}

	if len(name) > 200 {
		return fmt.Errorf("name too long")
	}

	// Verificar caracteres válidos (letras, espaços, alguns caracteres especiais)
	validName := regexp.MustCompile(`^[a-zA-ZÀ-ÿ\s'\-\.]+$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("name contains invalid characters")
	}

	return nil
}

// ValidateSessionID valida formato de session ID
func ValidateSessionID(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)

	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Session ID deve ter formato UUID ou similar (mínimo 8 caracteres alfanuméricos)
	if len(sessionID) < 8 {
		return fmt.Errorf("session ID too short")
	}

	if len(sessionID) > 128 {
		return fmt.Errorf("session ID too long")
	}

	// Apenas caracteres alfanuméricos e hífens
	validSessionID := regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`)
	if !validSessionID.MatchString(sessionID) {
		return fmt.Errorf("session ID contains invalid characters")
	}

	return nil
}
