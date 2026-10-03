package main

import (
	"context"
	"log"
	"os"
	"time"

	"goProjectStructure/internal/reuse"
	"goProjectStructure/internal/utils"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	log.Println("🌱 Starting Database Seeder...")

	cfg := utils.MustLoad()
	db, err := utils.ConnectDB(cfg.DBURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Seed Admin User
	seedAdminUser(ctx, db)

	// 2. Add your custom domain seeders here:
	// seedCustomEntities(ctx, db)

	log.Println("🌱 Database seeding completed successfully!")
	os.Exit(0)
}

func seedAdminUser(ctx context.Context, db *pgxpool.Pool) {
	email := "admin@example.com"
	password := "Admin@123456"
	name := "System Administrator"
	role := "admin"

	// Check if already exists
	var count int
	queryCheck := `SELECT COUNT(*) FROM users WHERE email = $1`
	err := db.QueryRow(ctx, queryCheck, email).Scan(&count)
	if err == nil && count > 0 {
		log.Printf("🌱 Admin user (%s) already exists. Skipping.", email)
		return
	}

	hashedPassword, err := reuse.HashPassword(password)
	if err != nil {
		log.Fatalf("🌱 Failed to hash admin password: %v", err)
	}

	queryInsert := `
		INSERT INTO users (name, email, password, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, NOW(), NOW())
		ON CONFLICT (email) DO NOTHING
	`
	_, err = db.Exec(ctx, queryInsert, name, email, hashedPassword, role)
	if err != nil {
		log.Printf(" Warning: Could not insert admin user: %v", err)
	} else {
		log.Printf("ðŸŽ‰ Seeded default admin user: %s (password: %s)", email, password)
	}
}