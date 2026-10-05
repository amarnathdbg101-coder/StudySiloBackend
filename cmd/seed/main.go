package main

import (
	"context"
	"fmt"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"

	"goProjectStructure/internal/reuse"
	"goProjectStructure/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	log.Println("🌱 Starting Database Seeder...")

	cfg := utils.MustLoad()
	db, err := utils.ConnectDB(cfg.DBURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Seed Admin User
	if err := seedAdminUser(ctx, db); err != nil {
		return err
	}

	// 2. Add your custom domain seeders here:
	// seedCustomEntities(ctx, db)

	log.Println("🌱 Database seeding completed successfully!")
	return nil
}

type adminSeedConfig struct {
	Email    string
	Password string
	Name     string
}

func loadAdminSeedConfig() (adminSeedConfig, error) {
	config := adminSeedConfig{
		Email:    strings.TrimSpace(os.Getenv("SEED_ADMIN_EMAIL")),
		Password: os.Getenv("SEED_ADMIN_PASSWORD"),
		Name:     strings.TrimSpace(os.Getenv("SEED_ADMIN_NAME")),
	}
	if config.Email == "" || config.Password == "" {
		return adminSeedConfig{}, fmt.Errorf("SEED_ADMIN_EMAIL and SEED_ADMIN_PASSWORD are required")
	}
	parsedEmail, err := mail.ParseAddress(config.Email)
	if err != nil || parsedEmail.Address != config.Email {
		return adminSeedConfig{}, fmt.Errorf("SEED_ADMIN_EMAIL must be a valid email address")
	}
	if len(config.Password) < 12 {
		return adminSeedConfig{}, fmt.Errorf("SEED_ADMIN_PASSWORD must be at least 12 characters")
	}
	if config.Name == "" {
		config.Name = "System Administrator"
	}
	return config, nil
}

func seedAdminUser(ctx context.Context, db *pgxpool.Pool) error {
	config, err := loadAdminSeedConfig()
	if err != nil {
		return err
	}
	role := "admin"

	// Check if already exists
	var count int
	queryCheck := `SELECT COUNT(*) FROM users WHERE email = $1`
	if err := db.QueryRow(ctx, queryCheck, config.Email).Scan(&count); err != nil {
		return fmt.Errorf("check existing admin user: %w", err)
	}
	if count > 0 {
		log.Printf("🌱 Admin user (%s) already exists. Skipping.", config.Email)
		return nil
	}

	hashedPassword, err := reuse.HashPassword(config.Password)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	queryInsert := `
		INSERT INTO users (full_name, email, password_hash, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		ON CONFLICT (email) DO NOTHING
	`
	if _, err := db.Exec(ctx, queryInsert, config.Name, config.Email, hashedPassword, role); err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}
	log.Printf("🌱 Seeded admin user: %s", config.Email)
	return nil
}
