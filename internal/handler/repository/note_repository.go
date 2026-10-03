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

type NoteRepository interface {
	Create(ctx context.Context, note *model.Note) error
	GetByID(ctx context.Context, id, userID string) (*model.Note, error)
	List(ctx context.Context, userID string, courseID, folderID *string, tag *string) ([]*model.Note, error)
	Update(ctx context.Context, note *model.Note) error
	Delete(ctx context.Context, id, userID string) error
}

type noteRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewNoteRepo(db *pgxpool.Pool, logger *zap.Logger) NoteRepository {
	return &noteRepo{db: db, logger: logger}
}

func (r *noteRepo) Create(ctx context.Context, note *model.Note) error {
	query := `
		INSERT INTO notes (user_id, course_id, folder_id, module_name, title, description, content, file_url, file_type, file_size, tags, is_pinned, is_archived, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	if note.Tags == nil {
		note.Tags = []string{}
	}
	return r.db.QueryRow(ctx, query,
		note.UserID, note.CourseID, note.FolderID, note.ModuleName,
		note.Title, note.Description, note.Content, note.FileURL,
		note.FileType, note.FileSize, note.Tags, note.IsPinned, note.IsArchived,
	).Scan(&note.ID, &note.CreatedAt, &note.UpdatedAt)
}

func (r *noteRepo) GetByID(ctx context.Context, id, userID string) (*model.Note, error) {
	query := `
		SELECT id, user_id, course_id, folder_id, module_name, title, description, content, file_url, file_type, file_size, tags, is_pinned, is_archived, created_at, updated_at
		FROM notes
		WHERE id = $1 AND user_id = $2
	`
	n := &model.Note{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&n.ID, &n.UserID, &n.CourseID, &n.FolderID, &n.ModuleName,
		&n.Title, &n.Description, &n.Content, &n.FileURL,
		&n.FileType, &n.FileSize, &n.Tags, &n.IsPinned, &n.IsArchived,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query note: %w", err)
	}
	return n, nil
}

func (r *noteRepo) List(ctx context.Context, userID string, courseID, folderID *string, tag *string) ([]*model.Note, error) {
	query := `
		SELECT id, user_id, course_id, folder_id, module_name, title, description, content, file_url, file_type, file_size, tags, is_pinned, is_archived, created_at, updated_at
		FROM notes
		WHERE user_id = $1 
		  AND is_archived = false
		  AND ($2::uuid IS NULL OR course_id = $2::uuid)
		  AND ($3::uuid IS NULL OR folder_id = $3::uuid)
		  AND ($4::text IS NULL OR $4::text = ANY(tags))
		ORDER BY is_pinned DESC, updated_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID, courseID, folderID, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to list notes: %w", err)
	}
	defer rows.Close()

	notes := []*model.Note{}
	for rows.Next() {
		n := &model.Note{}
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.CourseID, &n.FolderID, &n.ModuleName,
			&n.Title, &n.Description, &n.Content, &n.FileURL,
			&n.FileType, &n.FileSize, &n.Tags, &n.IsPinned, &n.IsArchived,
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, err
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

func (r *noteRepo) Update(ctx context.Context, note *model.Note) error {
	query := `
		UPDATE notes
		SET course_id = $1, folder_id = $2, module_name = $3, title = $4, description = $5, content = $6, file_url = $7, file_type = $8, file_size = $9, tags = $10, is_pinned = $11, is_archived = $12, updated_at = NOW()
		WHERE id = $13 AND user_id = $14
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		note.CourseID, note.FolderID, note.ModuleName, note.Title,
		note.Description, note.Content, note.FileURL, note.FileType,
		note.FileSize, note.Tags, note.IsPinned, note.IsArchived,
		note.ID, note.UserID,
	).Scan(&note.UpdatedAt)
}

func (r *noteRepo) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM notes WHERE id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("note not found or unauthorized")
	}
	return nil
}
