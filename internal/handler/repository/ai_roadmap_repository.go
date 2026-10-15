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

// AIRoadmapRepository - AI generated study roadmaps ke liye CRUD operations ka interface
type AIRoadmapRepository interface {
	// Create - naya AI roadmap DB mein save karo
	Create(ctx context.Context, roadmap *model.AIRoadmap) error

	// GetByID - ek roadmap fetch karo by ID (user verification ke saath)
	GetByID(ctx context.Context, id, userID string) (*model.AIRoadmap, error)

	// ListByUser - user ke saare roadmaps list karo (latest pehle)
	ListByUser(ctx context.Context, userID string) ([]*model.AIRoadmap, error)

	// Update - roadmap ka schedule/overview update karo (PATCH)
	Update(ctx context.Context, roadmap *model.AIRoadmap) error

	// Delete - roadmap permanently delete karo
	Delete(ctx context.Context, id, userID string) error
}

// aiRoadmapRepo - concrete implementation with pgxpool
type aiRoadmapRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewAIRoadmapRepo - constructor, dependency injection
func NewAIRoadmapRepo(db *pgxpool.Pool, logger *zap.Logger) AIRoadmapRepository {
	return &aiRoadmapRepo{db: db, logger: logger}
}

// Create - AI generated roadmap ko DB mein persist karo
// schedule ko JSONB mein marshal karke store karta hai
func (r *aiRoadmapRepo) Create(ctx context.Context, roadmap *model.AIRoadmap) error {
	// schedule slice ko JSON bytes mein convert karo PostgreSQL JSONB ke liye
	scheduleJSON, err := json.Marshal(roadmap.Schedule)
	if err != nil {
		return fmt.Errorf("failed to marshal schedule: %w", err)
	}

	query := `
		INSERT INTO ai_roadmaps (user_id, exam_name, overview, syllabus_topics, target_days, schedule, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, TRUE, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	return r.db.QueryRow(ctx, query,
		roadmap.UserID,
		roadmap.ExamName,
		roadmap.Overview,
		roadmap.SyllabusTopics, // pgx TEXT[] ke liye directly pass karo
		roadmap.TargetDays,
		scheduleJSON,
	).Scan(&roadmap.ID, &roadmap.CreatedAt, &roadmap.UpdatedAt)
}

// GetByID - single roadmap fetch karo with authorization check
func (r *aiRoadmapRepo) GetByID(ctx context.Context, id, userID string) (*model.AIRoadmap, error) {
	query := `
		SELECT id, user_id, exam_name, overview, syllabus_topics, target_days, schedule, is_active, created_at, updated_at
		FROM ai_roadmaps
		WHERE id = $1::uuid AND user_id = $2::uuid
	`

	roadmap := &model.AIRoadmap{}
	var scheduleJSON []byte

	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&roadmap.ID,
		&roadmap.UserID,
		&roadmap.ExamName,
		&roadmap.Overview,
		&roadmap.SyllabusTopics,
		&roadmap.TargetDays,
		&scheduleJSON,
		&roadmap.IsActive,
		&roadmap.CreatedAt,
		&roadmap.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // Not found - nil return karo bina error ke
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get roadmap by id: %w", err)
	}

	// JSONB se schedule unmarshal karo
	if err := json.Unmarshal(scheduleJSON, &roadmap.Schedule); err != nil {
		r.logger.Warn("failed to unmarshal roadmap schedule", zap.Error(err))
		roadmap.Schedule = []model.RoadmapTask{}
	}

	return roadmap, nil
}

// ListByUser - user ke saare roadmaps fetch karo (newest first)
func (r *aiRoadmapRepo) ListByUser(ctx context.Context, userID string) ([]*model.AIRoadmap, error) {
	query := `
		SELECT id, user_id, exam_name, overview, syllabus_topics, target_days, schedule, is_active, created_at, updated_at
		FROM ai_roadmaps
		WHERE user_id = $1::uuid
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list roadmaps: %w", err)
	}
	defer rows.Close()

	var roadmaps []*model.AIRoadmap

	for rows.Next() {
		roadmap := &model.AIRoadmap{}
		var scheduleJSON []byte

		if err := rows.Scan(
			&roadmap.ID,
			&roadmap.UserID,
			&roadmap.ExamName,
			&roadmap.Overview,
			&roadmap.SyllabusTopics,
			&roadmap.TargetDays,
			&scheduleJSON,
			&roadmap.IsActive,
			&roadmap.CreatedAt,
			&roadmap.UpdatedAt,
		); err != nil {
			r.logger.Error("failed to scan roadmap row", zap.Error(err))
			continue
		}

		// JSONB se schedule parse karo
		if err := json.Unmarshal(scheduleJSON, &roadmap.Schedule); err != nil {
			roadmap.Schedule = []model.RoadmapTask{}
		}

		roadmaps = append(roadmaps, roadmap)
	}

	return roadmaps, rows.Err()
}

// Update - roadmap ka content update karo (overview, schedule, is_active)
// Partial update support ke liye sirf diye gaye fields update hote hain
func (r *aiRoadmapRepo) Update(ctx context.Context, roadmap *model.AIRoadmap) error {
	scheduleJSON, err := json.Marshal(roadmap.Schedule)
	if err != nil {
		return fmt.Errorf("failed to marshal schedule: %w", err)
	}

	query := `
		UPDATE ai_roadmaps
		SET
			exam_name = $3,
			overview = $4,
			syllabus_topics = $5,
			target_days = $6,
			schedule = $7,
			is_active = $8,
			updated_at = NOW()
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING updated_at
	`

	err = r.db.QueryRow(ctx, query,
		roadmap.ID,
		roadmap.UserID,
		roadmap.ExamName,
		roadmap.Overview,
		roadmap.SyllabusTopics,
		roadmap.TargetDays,
		scheduleJSON,
		roadmap.IsActive,
	).Scan(&roadmap.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("roadmap not found or unauthorized")
	}
	return err
}

// Delete - roadmap permanently delete karo (hard delete with user check)
func (r *aiRoadmapRepo) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM ai_roadmaps WHERE id = $1::uuid AND user_id = $2::uuid`

	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete roadmap: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("roadmap not found or unauthorized")
	}
	return nil
}
