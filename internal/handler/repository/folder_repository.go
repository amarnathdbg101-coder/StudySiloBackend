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

type FolderRepository interface {
	Create(ctx context.Context, folder *model.Folder) error
	GetByID(ctx context.Context, id, userID string) (*model.Folder, error)
	ListByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error)
	GetTreeByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error)
	Update(ctx context.Context, folder *model.Folder) error
	Delete(ctx context.Context, id, userID string) error
}

type folderRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewFolderRepo(db *pgxpool.Pool, logger *zap.Logger) FolderRepository {
	return &folderRepo{db: db, logger: logger}
}

func (r *folderRepo) Create(ctx context.Context, folder *model.Folder) error {
	query := `
		INSERT INTO folders (user_id, course_id, parent_id, name, folder_type, description, color_hex, icon_name, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		folder.UserID, folder.CourseID, folder.ParentID,
		folder.Name, folder.FolderType, folder.Description,
		folder.ColorHex, folder.IconName, folder.Position,
	).Scan(&folder.ID, &folder.CreatedAt, &folder.UpdatedAt)
}

func (r *folderRepo) GetByID(ctx context.Context, id, userID string) (*model.Folder, error) {
	query := `
		SELECT id, user_id, course_id, parent_id, name, folder_type, description, color_hex, icon_name, position, created_at, updated_at
		FROM folders
		WHERE id = $1 AND user_id = $2
	`
	f := &model.Folder{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&f.ID, &f.UserID, &f.CourseID, &f.ParentID,
		&f.Name, &f.FolderType, &f.Description,
		&f.ColorHex, &f.IconName, &f.Position,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query folder: %w", err)
	}
	return f, nil
}

func (r *folderRepo) ListByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error) {
	query := `
		SELECT id, user_id, course_id, parent_id, name, folder_type, description, color_hex, icon_name, position, created_at, updated_at
		FROM folders
		WHERE course_id = $1 AND user_id = $2
		ORDER BY position ASC, created_at ASC
	`
	rows, err := r.db.Query(ctx, query, courseID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query folders: %w", err)
	}
	defer rows.Close()

	folders := []*model.Folder{}
	for rows.Next() {
		f := &model.Folder{}
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.CourseID, &f.ParentID,
			&f.Name, &f.FolderType, &f.Description,
			&f.ColorHex, &f.IconName, &f.Position,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// GetTreeByCourse loads all folders and constructs a hierarchical parent-child tree.
func (r *folderRepo) GetTreeByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error) {
	flat, err := r.ListByCourse(ctx, courseID, userID)
	if err != nil {
		return nil, err
	}

	folderMap := make(map[string]*model.Folder)
	for _, f := range flat {
		f.Children = []*model.Folder{}
		folderMap[f.ID] = f
	}

	roots := []*model.Folder{}
	for _, f := range flat {
		if f.ParentID == nil || *f.ParentID == "" {
			roots = append(roots, f)
		} else if parent, exists := folderMap[*f.ParentID]; exists {
			parent.Children = append(parent.Children, f)
		} else {
			roots = append(roots, f)
		}
	}

	return roots, nil
}

func (r *folderRepo) Update(ctx context.Context, folder *model.Folder) error {
	query := `
		UPDATE folders
		SET name = $1, parent_id = $2, folder_type = $3, description = $4, color_hex = $5, icon_name = $6, position = $7, updated_at = NOW()
		WHERE id = $8 AND user_id = $9
		RETURNING updated_at
	`
	return r.db.QueryRow(ctx, query,
		folder.Name, folder.ParentID, folder.FolderType,
		folder.Description, folder.ColorHex, folder.IconName,
		folder.Position, folder.ID, folder.UserID,
	).Scan(&folder.UpdatedAt)
}

func (r *folderRepo) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM folders WHERE id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("folder not found or unauthorized")
	}
	return nil
}
