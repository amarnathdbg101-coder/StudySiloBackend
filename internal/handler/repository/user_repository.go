package repository

import (
	"context"
	"errors"
	"fmt"
	"goProjectStructure/internal/handler/model"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrDuplicateEmail = errors.New("email already registered")
)

type UserRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewUserRepo(db *pgxpool.Pool, logger *zap.Logger) *UserRepo {
	return &UserRepo{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new user record into the database
func (r *UserRepo) Create(ctx context.Context, u *model.User) (*model.User, error) {
	query := `
		INSERT INTO users (email, password_hash, full_name, phone, role, is_active, created_at, updated_at)
		VALUES (LOWER($1), $2, $3, NULLIF(TRIM($4), ''), $5, $6, NOW(), NOW())
		RETURNING id, email, password_hash, full_name, COALESCE(phone, ''), COALESCE(avatar_url, ''), role, is_active, created_at, updated_at
	`
	created := &model.User{}
	err := r.db.QueryRow(ctx, query,
		u.Email,
		u.PasswordHash,
		u.FullName,
		u.Phone,
		u.Role,
		u.IsActive,
	).Scan(
		&created.ID,
		&created.Email,
		&created.PasswordHash,
		&created.FullName,
		&created.Phone,
		&created.AvatarURL,
		&created.Role,
		&created.IsActive,
		&created.CreatedAt,
		&created.UpdatedAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if strings.Contains(pgErr.ConstraintName, "email") || strings.Contains(pgErr.Message, "email") {
				return nil, ErrDuplicateEmail
			}
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return created, nil
}

// FindByEmail retrieves a user by their unique email
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, COALESCE(phone, ''), COALESCE(avatar_url, ''), role, is_active, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	u := &model.User{}
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Phone,
		&u.AvatarURL,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return u, nil
}

// FindByID retrieves a user by their primary key ID
func (r *UserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, COALESCE(phone, ''), COALESCE(avatar_url, ''), role, is_active, created_at, updated_at
		FROM users
		WHERE id::text = $1
	`
	u := &model.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.FullName,
		&u.Phone,
		&u.AvatarURL,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	return u, nil
}