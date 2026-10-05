package utils

import (
	"fmt"
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
	TrustedProxies []string
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

func validateProductionConfig(env, dbURL, jwtSecret, allowedOrigins string) error {
	if !strings.EqualFold(strings.TrimSpace(env), "production") {
		return nil
	}

	if strings.TrimSpace(dbURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when ENV=production")
	}
	trimmedJWTSecret := strings.TrimSpace(jwtSecret)
	if len(trimmedJWTSecret) < 32 ||
		trimmedJWTSecret == "default-dev-secret-key-change-in-production-32chars" ||
		trimmedJWTSecret == "super-secret-jwt-token-key-change-in-production-32chars" {
		return fmt.Errorf("JWT_SECRET must be a non-default secret of at least 32 characters when ENV=production")
	}

	origins := parseCommaList(allowedOrigins)
	if len(origins) == 0 {
		return fmt.Errorf("ALLOWED_ORIGINS must include at least one explicit origin when ENV=production")
	}

	return nil
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
		jwtSecret := os.Getenv("JWT_SECRET")
		allowedOrigins := os.Getenv("ALLOWED_ORIGINS")

		if dbURL == "" {
			log.Println("WARNING: DATABASE_URL not set in environment or .env. Using fallback local connection.")
			dbURL = "postgres://postgres:postgres@localhost:5432/myproject?sslmode=disable"
		}

		if jwtSecret == "" {
			log.Println("WARNING: JWT_SECRET not set. Using default development secret.")
			jwtSecret = "default-dev-secret-key-change-in-production-32chars"
		}

		if allowedOrigins == "" {
			allowedOrigins = "*"
		}

		cachedConfig = Config{
			Port:           port,
			Env:            env,
			DBURL:          dbURL,
			JwtSecret:      jwtSecret,
			AllowedOrigins: allowedOrigins,
			TrustedProxies: parseCommaList(os.Getenv("TRUSTED_PROXIES")),
			Bucket:         os.Getenv("R2_BUCKET"),
			AccessKey:      os.Getenv("R2_ACCESS_KEY"),
			SecretKey:      os.Getenv("R2_SECRET_KEY"),
			Endpoint:       os.Getenv("R2_ENDPOINT"),
			PublicURL:      os.Getenv("R2_PUBLIC_URL"),
			GeminiAPIKeys:  parseCommaList(getEnvFallback("GEMINI_API_KEYS", "GEMINI_API_KEY")),
			OpenAIAPIKeys:  parseCommaList(getEnvFallback("OPENAI_API_KEYS", "OPENAI_API_KEY")),
			ClaudeAPIKeys:  parseCommaList(getEnvFallback("CLAUDE_API_KEYS", "CLAUDE_API_KEY")),
		}
	})

	return cachedConfig
}

// MustLoadAPI applies web-server-specific production requirements after loading shared configuration.
func MustLoadAPI() Config {
	cfg := MustLoad()
	if err := validateProductionConfig(cfg.Env, os.Getenv("DATABASE_URL"), os.Getenv("JWT_SECRET"), os.Getenv("ALLOWED_ORIGINS")); err != nil {
		log.Fatal(err)
	}
	return cfg
}

func getEnvFallback(primary, fallback string) string {
	val := os.Getenv(primary)
	if val == "" {
		val = os.Getenv(fallback)
	}
	return val
}
