package utils

import (
	"log"
	"os"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	Env            string
	DBURL          string
	JwtSecret      string
	AllowedOrigins string
	Bucket         string
	AccessKey      string
	SecretKey      string
	Endpoint       string
	PublicURL      string
	GeminiAPIKeys  []string
	OpenAIAPIKeys  []string
	ClaudeAPIKeys  []string
}

var (
	configOnce   sync.Once
	cachedConfig Config
)

func parseCommaList(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := []string{}
	for _, p := range strings.Split(raw, ",") {
		clean := strings.TrimSpace(p)
		if clean != "" {
			parts = append(parts, clean)
		}
	}
	return parts
}

// MustLoad loads environment variables from .env or environment with safe production defaults
func MustLoad() Config {
	configOnce.Do(func() {
		// Load .env file only in local development (ignore error if file not found)
		_ = godotenv.Load(".env")

		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}

		env := os.Getenv("ENV")
		if env == "" {
			env = "development"
		}

		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			log.Println("WARNING: DATABASE_URL not set in environment or .env. Using fallback local connection.")
			dbURL = "postgres://postgres:postgres@localhost:5432/myproject?sslmode=disable"
		}

		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			log.Println("WARNING: JWT_SECRET not set. Using default development secret.")
			jwtSecret = "default-dev-secret-key-change-in-production-32chars"
		}

		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
		if allowedOrigins == "" {
			allowedOrigins = "*"
		}

		cachedConfig = Config{
			Port:           port,
			Env:            env,
			DBURL:          dbURL,
			JwtSecret:      jwtSecret,
			AllowedOrigins: allowedOrigins,
			Bucket:         os.Getenv("R2_BUCKET"),
			AccessKey:      os.Getenv("R2_ACCESS_KEY"),
			SecretKey:      os.Getenv("R2_SECRET_KEY"),
			Endpoint:       os.Getenv("R2_ENDPOINT"),
			PublicURL:      os.Getenv("R2_PUBLIC_URL"),
			GeminiAPIKeys:  parseCommaList(os.Getenv("GEMINI_API_KEYS")),
			OpenAIAPIKeys:  parseCommaList(os.Getenv("OPENAI_API_KEYS")),
			ClaudeAPIKeys:  parseCommaList(os.Getenv("CLAUDE_API_KEYS")),
		}
	})

	return cachedConfig
}