package repository

import (
	"context"
	"errors"
	"fmt"

	"goProjectStructure/internal/handler/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type CourseRepository interface {
	Create(ctx context.Context, course *model.Course) error
	GetByID(ctx context.Context, id, userID string) (*model.Course, error)
	ListByUser(ctx context.Context, userID string) ([]*model.Course, error)
	Update(ctx context.Context, course *model.Course) error
	Delete(ctx context.Context, id, userID string) error
}

type courseRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewCourseRepo(db *pgxpool.Pool, logger *zap.Logger) CourseRepository {
	return &courseRepo{db: db, logger: logger}
}

func (r *courseRepo) Create(ctx context.Context, course *model.Course) error {
	query := `
		INSERT INTO courses (user_id, title, code, semester, description, color_hex, icon_name, is_archived, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		course.UserID, course.Title, course.Code, course.Semester,
		course.Description, course.ColorHex, course.IconName, course.IsArchived,
	).Scan(&course.ID, &course.CreatedAt, &course.UpdatedAt)
}

func (r *courseRepo) GetByID(ctx context.Context, id, userID string) (*model.Course, error) {
	query := `
		SELECT id, user_id, title, code, semester, description, color_hex, icon_name, is_archived, created_at, updated_at
		FROM courses
		WHERE id = $1 AND user_id = $2
	`
	c := &model.Course{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&c.ID, &c.UserID, &c.Title, &c.Code, &c.Semester,
		&c.Description, &c.ColorHex, &c.IconName, &c.IsArchived,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query course: %w", err)
	}
	return c, nil
}

func (r *courseRepo) ListByUser(ctx context.Context, userID string) ([]*model.Course, error) {
	query := `
		SELECT id, user_id, title, code, semester, description, color_hex, icon_name, is_archived, created_at, updated_at
		FROM courses
		WHERE user_id = $1 AND is_archived = false
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query courses: %w", err)
	}
	defer rows.Close()

	courses := []*model.Course{}
	for rows.Next() {
		c := &model.Course{}
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.Code, &c.Semester,
			&c.Description, &c.ColorHex, &c.IconName, &c.IsArchived,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}

func (r *courseRepo) Update(ctx context.Context, course *model.Course) error {
	query := `
		UPDATE courses
		SET title = $1, code = $2, semester = $3, description = $4, color_hex = $5, icon_name = $6, is_archived = $7, updated_at = NOW()
		WHERE id = $8 AND user_id = $9
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		course.Title, course.Code, course.Semester, course.Description,
		course.ColorHex, course.IconName, course.IsArchived,
		course.ID, course.UserID,
	).Scan(&course.UpdatedAt)
}

func (r *courseRepo) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM courses WHERE id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("course not found or unauthorized")
	}
	return nil
}
