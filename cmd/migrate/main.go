package main

import (
	"fmt"
	"log"
	"os"
	"goProjectStructure/internal/utils"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run ./cmd/migrate [up | down | force <version> | version]")
		return
	}

	cfg := utils.MustLoad()
	m, err := migrate.New("file://internal/migration", cfg.DBURL)
	if err != nil {
		log.Fatalf("failed to create migration instance: %v", err)
	}

	switch os.Args[1] {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migration up failed: %v", err)
		}
		log.Println("Migrations applied successfully!")
	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migration down failed: %v", err)
		}
		log.Println("Rolled back 1 migration step successfully!")
	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Usage: go run ./cmd/migrate force <version_number>")
		}
		var version int
		if _, err := fmt.Sscanf(os.Args[2], "%d", &version); err != nil {
			log.Fatalf("invalid version integer: %v", err)
		}
		if err := m.Force(version); err != nil {
			log.Fatalf("force migration failed: %v", err)
		}
		log.Printf("Forced migration state to version %d", version)
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("error retrieving migration version: %v", err)
		}
		log.Printf("Current Database Version: %d, Dirty: %v", v, dirty)
	default:
		log.Fatalf("unknown migration command: %s. Use up, down, force, or version.", os.Args[1])
	}
}