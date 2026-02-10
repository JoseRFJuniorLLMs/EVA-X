package security

import (
	"log"
)

// SafeError retorna uma mensagem de erro segura sem expor detalhes internos
// Use esta função para todos os erros enviados ao cliente
func SafeError(err error, userMessage string) string {
	if err != nil {
		// Log do erro real para debugging (apenas servidor)
		log.Printf("❌ Internal error: %v", err)
	}

	// Retornar apenas mensagem genérica ao usuário
	return userMessage
}

// SafeErrorMap retorna um mapa de erro seguro
func SafeErrorMap(err error, userMessage string) map[string]interface{} {
	if err != nil {
		log.Printf("❌ Internal error: %v", err)
	}

	return map[string]interface{}{
		"success": false,
		"error":   userMessage,
	}
}

// SafeHTTPError retorna um erro HTTP seguro
func SafeHTTPError(err error, userMessage string, statusCode int) (string, int) {
	if err != nil {
		log.Printf("❌ Internal error: %v", err)
	}

	return userMessage, statusCode
}

// IsValidationError verifica se o erro é de validação
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}
	// Adicionar lógica para identificar erros de validação específicos
	return false
}

// ErrorCode retorna um código de erro genérico baseado no tipo
func ErrorCode(err error) string {
	if err == nil {
		return "UNKNOWN"
	}

	// Mapear erros para códigos genéricos
	// Evitar expor detalhes de implementação
	errStr := err.Error()

	switch {
	case contains(errStr, "not found"):
		return "NOT_FOUND"
	case contains(errStr, "invalid"):
		return "INVALID_INPUT"
	case contains(errStr, "unauthorized"):
		return "UNAUTHORIZED"
	case contains(errStr, "forbidden"):
		return "FORBIDDEN"
	default:
		return "INTERNAL_ERROR"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
