package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"goProjectStructure/internal/handler/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	ErrMindMapNotFound = errors.New("mind map not found")
)

type MindMapRepository interface {
	Create(ctx context.Context, m *model.MindMap) error
	GetByID(ctx context.Context, userID, id string) (*model.MindMap, error)
	List(ctx context.Context, userID string, search string, limit, offset int) ([]*model.MindMap, int, error)
	Update(ctx context.Context, userID, id string, title *string, data json.RawMessage, isPinned *bool) (*model.MindMap, error)
	Delete(ctx context.Context, userID, id string) error
	TogglePin(ctx context.Context, userID, id string) (*model.MindMap, error)
}

type mindMapRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewMindMapRepo(db *pgxpool.Pool, logger *zap.Logger) MindMapRepository {
	return &mindMapRepo{db: db, logger: logger}
}

func (r *mindMapRepo) Create(ctx context.Context, m *model.MindMap) error {
	query := `
		INSERT INTO mind_maps (user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	if m.DiagramType == "" {
		m.DiagramType = "mindmap"
	}
	if m.DomainType == "" {
		m.DomainType = "general"
	}

	err := r.db.QueryRow(ctx, query,
		m.UserID, m.Title, m.Concept, m.DiagramType, m.DomainType, m.NoteID, m.Data, m.IsPinned,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create mind map: %w", err)
	}
	return nil
}

func (r *mindMapRepo) GetByID(ctx context.Context, userID, id string) (*model.MindMap, error) {
	query := `
		SELECT id, user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at
		FROM mind_maps
		WHERE id = $1 AND user_id = $2
	`
	m := &model.MindMap{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&m.ID, &m.UserID, &m.Title, &m.Concept, &m.DiagramType, &m.DomainType,
		&m.NoteID, &m.Data, &m.IsPinned, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMindMapNotFound
		}
		return nil, fmt.Errorf("failed to get mind map: %w", err)
	}
	return m, nil
}

func (r *mindMapRepo) List(ctx context.Context, userID string, search string, limit, offset int) ([]*model.MindMap, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var countQuery string
	var listQuery string
	var countArgs []interface{}
	var listArgs []interface{}

	if search != "" {
		searchTerm := "%" + search + "%"
		countQuery = `
			SELECT COUNT(*)
			FROM mind_maps
			WHERE user_id = $1 AND (title ILIKE $2 OR concept ILIKE $2)
		`
		countArgs = []interface{}{userID, searchTerm}

		listQuery = `
			SELECT id, user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at
			FROM mind_maps
			WHERE user_id = $1 AND (title ILIKE $2 OR concept ILIKE $2)
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $3 OFFSET $4
		`
		listArgs = []interface{}{userID, searchTerm, limit, offset}
	} else {
		countQuery = `
			SELECT COUNT(*)
			FROM mind_maps
			WHERE user_id = $1
		`
		countArgs = []interface{}{userID}

		listQuery = `
			SELECT id, user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at
			FROM mind_maps
			WHERE user_id = $1
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $2 OFFSET $3
		`
		listArgs = []interface{}{userID, limit, offset}
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count mind maps: %w", err)
	}

	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list mind maps: %w", err)
	}
	defer rows.Close()

	var result []*model.MindMap
	for rows.Next() {
		m := &model.MindMap{}
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.Title, &m.Concept, &m.DiagramType, &m.DomainType,
			&m.NoteID, &m.Data, &m.IsPinned, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan mind map row: %w", err)
		}
		result = append(result, m)
	}

	return result, total, nil
}

func (r *mindMapRepo) Update(ctx context.Context, userID, id string, title *string, data json.RawMessage, isPinned *bool) (*model.MindMap, error) {
	// Build dynamic update query or use COALESCE
	query := `
		UPDATE mind_maps
		SET
			title = COALESCE($3, title),
			data = CASE WHEN $4::jsonb IS NOT NULL THEN $4::jsonb ELSE data END,
			is_pinned = COALESCE($5, is_pinned),
			updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at
	`
	var rawDataParam *string
	if len(data) > 0 {
		str := string(data)
		rawDataParam = &str
	}

	m := &model.MindMap{}
	err := r.db.QueryRow(ctx, query, id, userID, title, rawDataParam, isPinned).Scan(
		&m.ID, &m.UserID, &m.Title, &m.Concept, &m.DiagramType, &m.DomainType,
		&m.NoteID, &m.Data, &m.IsPinned, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMindMapNotFound
		}
		return nil, fmt.Errorf("failed to update mind map: %w", err)
	}
	return m, nil
}

func (r *mindMapRepo) Delete(ctx context.Context, userID, id string) error {
	query := `DELETE FROM mind_maps WHERE id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete mind map: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMindMapNotFound
	}
	return nil
}

func (r *mindMapRepo) TogglePin(ctx context.Context, userID, id string) (*model.MindMap, error) {
	query := `
		UPDATE mind_maps
		SET is_pinned = NOT is_pinned, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, title, concept, diagram_type, domain_type, note_id, data, is_pinned, created_at, updated_at
	`
	m := &model.MindMap{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&m.ID, &m.UserID, &m.Title, &m.Concept, &m.DiagramType, &m.DomainType,
		&m.NoteID, &m.Data, &m.IsPinned, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrMindMapNotFound
		}
		return nil, fmt.Errorf("failed to toggle mind map pin: %w", err)
	}
	return m, nil
}
