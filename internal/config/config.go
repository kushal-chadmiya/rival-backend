package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application runtime configuration.
type Config struct {
	Port               string
	DatabaseURL        string
	FrontendURL        string
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseJWTSecret      string
	SupabaseServiceRoleKey string
	SupabaseJWKSURL        string
	StorageBucket          string
	MaxUploadBytes         int64
	RequestTimeoutSecs     int
	AllowViewAsAdmin       bool
}

// Load loads and validates configuration from environment variables.
func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Port:               getenvDefault("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		FrontendURL:        getenvDefault("FRONTEND_URL", "http://localhost:3000"),
		SupabaseURL:        strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		SupabaseAnonKey:    os.Getenv("SUPABASE_ANON_KEY"),
		SupabaseJWTSecret:      os.Getenv("SUPABASE_JWT_SECRET"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseJWKSURL:        os.Getenv("SUPABASE_JWKS_URL"),
		StorageBucket:          getenvDefault("STORAGE_BUCKET", "task-attachments"),
		MaxUploadBytes:         getenvInt64Default("MAX_UPLOAD_BYTES", 10*1024*1024),
		RequestTimeoutSecs:     10,
		AllowViewAsAdmin:       getenvBoolDefault("ALLOW_VIEW_AS_ADMIN", true),
	}

	switch {
	case cfg.DatabaseURL == "":
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	case cfg.SupabaseURL == "":
		return Config{}, fmt.Errorf("SUPABASE_URL is required")
	case cfg.SupabaseAnonKey == "":
		return Config{}, fmt.Errorf("SUPABASE_ANON_KEY is required")
	}

	if cfg.SupabaseJWKSURL == "" {
		cfg.SupabaseJWKSURL = cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"
	}

	return cfg, nil
}

func getenvDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getenvBoolDefault(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func getenvInt64Default(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
