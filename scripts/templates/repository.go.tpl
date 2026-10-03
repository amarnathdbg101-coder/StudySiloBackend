package repository

import (
	"context"
	"fmt"

	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type __PASCAL__Repository struct {
	db *pgxpool.Pool
}

func New__PASCAL__Repository(db *pgxpool.Pool) *__PASCAL__Repository {
	return &__PASCAL__Repository{db: db}
}

func (r *__PASCAL__Repository) Create(ctx context.Context, item *model.__PASCAL__) error {
	query := `
		INSERT INTO __PLURAL__ (title, description, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query, item.Title, item.Description).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *__PASCAL__Repository) FindByID(ctx context.Context, id int64) (*model.__PASCAL__, error) {
	query := `SELECT id, title, description, created_at, updated_at FROM __PLURAL__ WHERE id = $1`
	var item model.__PASCAL__
	err := r.db.QueryRow(ctx, query, id).Scan(&item.ID, &item.Title, &item.Description, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query __LOWER__: %w", err)
	}
	return &item, nil
}

func (r *__PASCAL__Repository) FindAll(ctx context.Context, p reuse.PaginationParams) ([]model.__PASCAL__, int64, error) {
	var total int64
	countQuery := `SELECT COUNT(*) FROM __PLURAL__`
	if err := r.db.QueryRow(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		"SELECT id, title, description, created_at, updated_at FROM __PLURAL__ ORDER BY id %s LIMIT $1 OFFSET $2",
		p.Order,
	)

	rows, err := r.db.Query(ctx, query, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]model.__PASCAL__, 0)
	for rows.Next() {
		var item model.__PASCAL__
		if err := rows.Scan(&item.ID, &item.Title, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

func (r *__PASCAL__Repository) Update(ctx context.Context, item *model.__PASCAL__) error {
	query := `
		UPDATE __PLURAL__
		SET title = $1, description = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, item.Title, item.Description, item.ID)
	return err
}

func (r *__PASCAL__Repository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM __PLURAL__ WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
