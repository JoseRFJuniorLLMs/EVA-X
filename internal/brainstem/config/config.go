package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	Port        string
	Environment string
	MetricsPort string

	// Database
	DatabaseURL string

	// Twilio (para fallback SMS e chamadas)
	ServiceDomain     string
	TwilioAccountSID  string
	TwilioAuthToken   string
	TwilioPhoneNumber string

	// Google/Gemini
	GoogleAPIKey        string
	ModelID             string
	GeminiAnalysisModel string
	VisionModelID       string

	// Scheduler
	SchedulerInterval int
	MaxRetries        int

	// Firebase
	FirebaseCredentialsPath string

	// Alert System
	AlertRetryInterval   int  // Intervalo entre tentativas de reenvio (minutos)
	AlertEscalationTime  int  // Tempo até escalonamento (minutos)
	EnableSMSFallback    bool // Habilitar SMS como fallback
	EnableEmailFallback  bool // Habilitar Email como fallback
	EnableCallFallback   bool // Habilitar ligação como fallback
	CriticalAlertTimeout int  // Timeout para alertas críticos (minutos)

	// SMTP Configuration
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string

	// Auth
	JWTSecret string

	// Google Services
	GoogleMapsAPIKey string

	// Neo4j
	Neo4jURI      string
	Neo4jUsername string
	Neo4jPassword string

	// Redis
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// Qdrant
	QdrantHost string
	QdrantPort int
	AppURL     string

	// Feature Flags (V2)
	EnableGoogleSearch   bool
	EnableCodeExecution  bool
	EnableContextCaching bool
}

func Load() (*Config, error) {
	// Tenta carregar do diretório atual
	if err := godotenv.Load(); err != nil {
		// Se falhar, tenta carregar do diretório do executável (comum em SystemD)
		ex, exErr := os.Executable()
		if exErr == nil {
			exPath := filepath.Dir(ex)
			envPath := filepath.Join(exPath, ".env")
			if err2 := godotenv.Load(envPath); err2 != nil {
				log.Printf("⚠️ Arquivo .env não encontrado em %s nem no diretório atual.", envPath)
			} else {
				log.Printf("✅ Carregado .env do diretório do binário: %s", envPath)
			}
		} else {
			log.Printf("⚠️ Erro ao determinar path do executável: %v", exErr)
		}
	}

	return &Config{
		// Server (porta interna 8091 - nginx faz proxy SSL na 8090)
		Port:        getEnvWithDefault("PORT", "8091"),
		Environment: getEnvWithDefault("ENVIRONMENT", "development"),
		MetricsPort: getEnvWithDefault("METRICS_PORT", "9090"),

		// Database
		DatabaseURL: os.Getenv("DATABASE_URL"),

		// Twilio
		TwilioAccountSID:  os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:   os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioPhoneNumber: os.Getenv("TWILIO_PHONE_NUMBER"),

		// Google/Gemini
		GoogleAPIKey: strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")),

		// 🚨 EXPRESS ORDER: Gemini 2.5 para VOZ (Definitivo)
		ModelID:             getEnvWithDefault("MODEL_ID", "gemini-2.5-flash-native-audio-preview-12-2025"),
		GeminiAnalysisModel: getEnvWithDefault("GEMINI_ANALYSIS_MODEL", "gemini-2.5-flash"),
		// Modelo de Apoio para Ferramentas (Delegation)
		VisionModelID: getEnvWithDefault("VISION_MODEL_ID", "gemini-2.0-flash-exp"),

		// Scheduler
		SchedulerInterval: getEnvInt("SCHEDULER_INTERVAL", 1),
		MaxRetries:        getEnvInt("MAX_RETRIES", 3),

		// Firebase
		FirebaseCredentialsPath: os.Getenv("FIREBASE_CREDENTIALS_PATH"),

		// Alert System
		AlertRetryInterval:   getEnvInt("ALERT_RETRY_INTERVAL", 5),
		AlertEscalationTime:  getEnvInt("ALERT_ESCALATION_TIME", 5),
		EnableSMSFallback:    getEnvBool("ENABLE_SMS_FALLBACK", false),
		EnableEmailFallback:  getEnvBool("ENABLE_EMAIL_FALLBACK", true),
		EnableCallFallback:   getEnvBool("ENABLE_CALL_FALLBACK", false),
		CriticalAlertTimeout: getEnvInt("CRITICAL_ALERT_TIMEOUT", 5),

		// SMTP
		SMTPHost:      getEnvWithDefault("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:      getEnvInt("SMTP_PORT", 587),
		SMTPUsername:  os.Getenv("SMTP_USERNAME"),
		SMTPPassword:  os.Getenv("SMTP_PASSWORD"),
		SMTPFromName:  getEnvWithDefault("SMTP_FROM_NAME", "EVA - Assistente Virtual"),
		SMTPFromEmail: getEnvWithDefault("SMTP_FROM_EMAIL", "web2ajax@gmail.com"),

		// Auth
		JWTSecret: getEnvWithDefault("JWT_SECRET", "super-secret-default-key-change-me"),

		// Neo4j
		Neo4jURI:      getEnvWithDefault("NEO4J_URI", "neo4j://localhost:7687"),
		Neo4jUsername: getEnvWithDefault("NEO4J_USERNAME", "neo4j"),
		Neo4jPassword: getEnvWithDefault("NEO4J_PASSWORD", "password"),

		// Redis
		RedisHost:     getEnvWithDefault("REDIS_HOST", "localhost"),
		RedisPort:     getEnvWithDefault("REDIS_PORT", "6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),

		// Qdrant
		QdrantHost: getEnvWithDefault("QDRANT_HOST", "localhost"),
		QdrantPort: getEnvInt("QDRANT_PORT", 6334),
		AppURL:     getEnv("APP_URL", "https://eva-mind-fzpn.fly.dev"),

		// Load Feature Flags (Default: false for safety/compatibility)
		EnableGoogleSearch:   getEnvBool("ENABLE_GOOGLE_SEARCH", false),
		EnableCodeExecution:  getEnvBool("ENABLE_CODE_EXECUTION", false),
		EnableContextCaching: getEnvBool("ENABLE_CONTEXT_CACHING", true),
	}, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val := getEnv(key, "")
	if val == "" {
		return fallback
	}
	// Aceita "true", "1", "yes", "on"
	return val == "true" || val == "1" || val == "yes" || val == "on"
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Validate valida se todas as configurações obrigatórias estão presentes
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if c.GoogleAPIKey == "" {
		return fmt.Errorf("GOOGLE_API_KEY is required")
	}

	if c.FirebaseCredentialsPath == "" {
		return fmt.Errorf("FIREBASE_CREDENTIALS_PATH is required")
	}

	// Verificar se fallbacks estão habilitados mas sem credenciais
	if c.EnableSMSFallback && (c.TwilioAccountSID == "" || c.TwilioAuthToken == "") {
		log.Println("⚠️  SMS fallback habilitado mas credenciais Twilio não configuradas")
	}

	if c.EnableEmailFallback && (c.SMTPUsername == "" || c.SMTPPassword == "") {
		log.Println("⚠️  Email fallback habilitado mas credenciais SMTP não configuradas")
	}

	return nil
}
