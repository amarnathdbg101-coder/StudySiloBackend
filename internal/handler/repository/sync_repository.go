package repository

import (
	"context"
	"fmt"
	"time"

	"goProjectStructure/internal/handler/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type SyncRepository interface {
	GetDelta(ctx context.Context, userID string, since time.Time) ([]*model.Course, []*model.Folder, []*model.Note, []*model.Quiz, error)
}

type syncRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewSyncRepo(db *pgxpool.Pool, logger *zap.Logger) SyncRepository {
	return &syncRepo{db: db, logger: logger}
}

func (r *syncRepo) GetDelta(ctx context.Context, userID string, since time.Time) ([]*model.Course, []*model.Folder, []*model.Note, []*model.Quiz, error) {
	// 1. Courses updated since
	coursesQuery := `
		SELECT id, user_id, title, code, semester, description, color_hex, icon_name, is_archived, created_at, updated_at
		FROM courses
		WHERE user_id = $1 AND updated_at >= $2
		ORDER BY updated_at ASC
	`
	cRows, err := r.db.Query(ctx, coursesQuery, userID, since)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to sync courses: %w", err)
	}
	defer cRows.Close()

	courses := []*model.Course{}
	for cRows.Next() {
		c := &model.Course{}
		if err := cRows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.Code, &c.Semester,
			&c.Description, &c.ColorHex, &c.IconName, &c.IsArchived,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, nil, nil, nil, err
		}
		courses = append(courses, c)
	}

	// 2. Folders updated since
	foldersQuery := `
		SELECT id, user_id, course_id, parent_id, name, folder_type, description, color_hex, icon_name, position, created_at, updated_at
		FROM folders
		WHERE user_id = $1 AND updated_at >= $2
		ORDER BY position ASC, updated_at ASC
	`
	fRows, err := r.db.Query(ctx, foldersQuery, userID, since)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to sync folders: %w", err)
	}
	defer fRows.Close()

	folders := []*model.Folder{}
	for fRows.Next() {
		f := &model.Folder{}
		if err := fRows.Scan(
			&f.ID, &f.UserID, &f.CourseID, &f.ParentID,
			&f.Name, &f.FolderType, &f.Description,
			&f.ColorHex, &f.IconName, &f.Position,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, nil, nil, nil, err
		}
		folders = append(folders, f)
	}

	// 3. Notes updated since
	notesQuery := `
		SELECT id, user_id, course_id, folder_id, module_name, title, description, content, file_url, file_type, file_size, tags, is_pinned, is_archived, created_at, updated_at
		FROM notes
		WHERE user_id = $1 AND updated_at >= $2
		ORDER BY updated_at ASC
	`
	nRows, err := r.db.Query(ctx, notesQuery, userID, since)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to sync notes: %w", err)
	}
	defer nRows.Close()

	notes := []*model.Note{}
	for nRows.Next() {
		n := &model.Note{}
		if err := nRows.Scan(
			&n.ID, &n.UserID, &n.CourseID, &n.FolderID, &n.ModuleName,
			&n.Title, &n.Description, &n.Content, &n.FileURL,
			&n.FileType, &n.FileSize, &n.Tags, &n.IsPinned, &n.IsArchived,
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return nil, nil, nil, nil, err
		}
		notes = append(notes, n)
	}

	// 4. Quizzes created since
	quizzesQuery := `
		SELECT id, user_id, course_id, note_id, title, topic, difficulty, total_questions, created_at
		FROM quizzes
		WHERE user_id = $1 AND created_at >= $2
		ORDER BY created_at ASC
	`
	qRows, err := r.db.Query(ctx, quizzesQuery, userID, since)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to sync quizzes: %w", err)
	}
	defer qRows.Close()

	quizzes := []*model.Quiz{}
	for qRows.Next() {
		q := &model.Quiz{}
		if err := qRows.Scan(
			&q.ID, &q.UserID, &q.CourseID, &q.NoteID,
			&q.Title, &q.Topic, &q.Difficulty, &q.TotalQuestions,
			&q.CreatedAt,
		); err != nil {
			return nil, nil, nil, nil, err
		}
		quizzes = append(quizzes, q)
	}

	return courses, folders, notes, quizzes, nil
}
