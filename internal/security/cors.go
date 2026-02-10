package security

import (
	"net/http"
	"strings"
)

// CORSConfig representa a configuração de CORS
type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// DefaultCORSConfig retorna a configuração padrão de CORS
func DefaultCORSConfig() *CORSConfig {
	return &CORSConfig{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:8080",
			"https://eva-ia.org",
			"https://www.eva-ia.org",
			"https://eva-mind.app",
			"https://www.eva-mind.app",
		},
		AllowedMethods: []string{
			"GET",
			"POST",
			"PUT",
			"DELETE",
			"OPTIONS",
		},
		AllowedHeaders: []string{
			"Content-Type",
			"Authorization",
			"X-Requested-With",
		},
	}
}

// IsOriginAllowed verifica se a origem é permitida
func (c *CORSConfig) IsOriginAllowed(origin string) bool {
	origin = strings.TrimSpace(origin)

	// Rejeitar origens vazias
	if origin == "" {
		return false
	}

	// Verificar contra whitelist
	for _, allowedOrigin := range c.AllowedOrigins {
		if origin == allowedOrigin {
			return true
		}

		// Suportar wildcard subdomains (ex: *.eva-mind.app)
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := allowedOrigin[2:]
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// CORSMiddleware retorna um middleware de CORS seguro
func CORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = DefaultCORSConfig()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Verificar se a origem é permitida
			if config.IsOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				// Não definir headers CORS para origens não permitidas
				// Isso fará o browser bloquear a requisição
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 horas

			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CheckOriginWebSocket verifica origem para WebSocket (usar no upgrader)
func CheckOriginWebSocket(config *CORSConfig) func(r *http.Request) bool {
	if config == nil {
		config = DefaultCORSConfig()
	}

	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")

		// Permitir conexões sem Origin (apps nativos, chamadas internas)
		if origin == "" {
			return true
		}

		// Permitir conexões locais (127.0.0.1, localhost)
		if strings.HasPrefix(origin, "http://127.0.0.1") ||
			strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "https://127.0.0.1") ||
			strings.HasPrefix(origin, "https://localhost") {
			return true
		}

		return config.IsOriginAllowed(origin)
	}
}
