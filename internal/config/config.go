package config

import (
    "os"
    "strconv"
)

type Config struct {
    // Server
    Port        string
    Environment string
    MetricsPort string

    // Database
    DatabaseURL string

    // Twilio
    TwilioAccountSID  string
    TwilioAuthToken   string
    TwilioPhoneNumber string
    ServiceDomain     string

    // Google/Gemini
    GoogleAPIKey string
    ModelID      string

    // Scheduler
    SchedulerInterval int // minutes
    MaxRetries        int
}

func Load() *Config {
    return &Config{
        // Server
        Port:        getEnv("PORT", "8080"),
        Environment: getEnv("ENVIRONMENT", "development"),
        MetricsPort: getEnv("METRICS_PORT", "9090"),

        // Database
        DatabaseURL: getEnv("DATABASE_URL", ""),

        // Twilio
        TwilioAccountSID:  getEnv("TWILIO_ACCOUNT_SID", ""),
        TwilioAuthToken:   getEnv("TWILIO_AUTH_TOKEN", ""),
        TwilioPhoneNumber: getEnv("TWILIO_PHONE_NUMBER", ""),
        ServiceDomain:     getEnv("SERVICE_DOMAIN", ""),

        // Google/Gemini
        GoogleAPIKey: getEnv("GOOGLE_API_KEY", ""),
        ModelID:      getEnv("MODEL_ID", "gemini-2.5-flash-native-audio-preview-12-2025"),

        // Scheduler
        SchedulerInterval: getEnvInt("SCHEDULER_INTERVAL", 1),
        MaxRetries:        getEnvInt("MAX_RETRIES", 3),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intValue, err := strconv.Atoi(value); err == nil {
            return intValue
        }
    }
    return defaultValue
}
