package repository

import (
	"context"
	"fmt"

	"goProjectStructure/internal/handler/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type StudyPlanRepository interface {
	Create(ctx context.Context, plan *model.StudyPlan) error
	GetByID(ctx context.Context, id, userID string) (*model.StudyPlan, error)
	ListByUser(ctx context.Context, userID string) ([]*model.StudyPlan, error)
	Update(ctx context.Context, plan *model.StudyPlan) error
	Delete(ctx context.Context, id, userID string) error
	RecordStudyMinutes(ctx context.Context, userID string, minutes int) (*model.CognitiveProfile, error)
}

type studyPlanRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewStudyPlanRepo(db *pgxpool.Pool, logger *zap.Logger) StudyPlanRepository {
	return &studyPlanRepo{db: db, logger: logger}
}

func (r *studyPlanRepo) Create(ctx context.Context, plan *model.StudyPlan) error {
	query := `
		INSERT INTO study_plans (id, user_id, course_id, title, due_date, is_completed, priority, created_at, updated_at)
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id::text, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		plan.ID, plan.UserID, plan.CourseID, plan.Title,
		plan.DueDate, plan.IsCompleted, plan.Priority,
	).Scan(&plan.ID, &plan.CreatedAt, &plan.UpdatedAt)
}

func (r *studyPlanRepo) GetByID(ctx context.Context, id, userID string) (*model.StudyPlan, error) {
	query := `
		SELECT id::text, user_id::text, course_id::text, title, due_date, is_completed, priority, created_at, updated_at
		FROM study_plans
		WHERE id = $1::uuid AND user_id = $2
	`
	p := &model.StudyPlan{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&p.ID, &p.UserID, &p.CourseID, &p.Title,
		&p.DueDate, &p.IsCompleted, &p.Priority,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *studyPlanRepo) ListByUser(ctx context.Context, userID string) ([]*model.StudyPlan, error) {
	query := `
		SELECT id::text, user_id::text, course_id::text, title, due_date, is_completed, priority, created_at, updated_at
		FROM study_plans
		WHERE user_id = $1
		ORDER BY is_completed ASC, due_date ASC NULLS LAST, created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query study plans: %w", err)
	}
	defer rows.Close()

	var plans []*model.StudyPlan
	for rows.Next() {
		p := &model.StudyPlan{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.CourseID, &p.Title,
			&p.DueDate, &p.IsCompleted, &p.Priority,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *studyPlanRepo) Update(ctx context.Context, plan *model.StudyPlan) error {
	query := `
		UPDATE study_plans
		SET title = $3, is_completed = $4, priority = $5, updated_at = NOW()
		WHERE id = $1::uuid AND user_id = $2
	`
	_, err := r.db.Exec(ctx, query, plan.ID, plan.UserID, plan.Title, plan.IsCompleted, plan.Priority)
	return err
}

func (r *studyPlanRepo) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM study_plans WHERE id = $1::uuid AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}

func (r *studyPlanRepo) RecordStudyMinutes(ctx context.Context, userID string, minutes int) (*model.CognitiveProfile, error) {
	query := `
		INSERT INTO cognitive_profiles (user_id, total_study_minutes, study_streak_days, last_active_at, updated_at)
		VALUES ($1, $2, 1, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			total_study_minutes = cognitive_profiles.total_study_minutes + $2,
			study_streak_days = CASE
				WHEN cognitive_profiles.last_active_at < NOW() - INTERVAL '48 hours' THEN 1
				WHEN cognitive_profiles.last_active_at < NOW() - INTERVAL '24 hours' THEN cognitive_profiles.study_streak_days + 1
				ELSE cognitive_profiles.study_streak_days
			END,
			last_active_at = NOW(),
			updated_at = NOW()
		RETURNING user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps, study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score, last_active_at, updated_at
	`
	p := &model.CognitiveProfile{}
	err := r.db.QueryRow(ctx, query, userID, minutes).Scan(
		&p.UserID, &p.TargetExam, &p.AcademicLevel,
		&p.WeakTopics, &p.StrongTopics, &p.ExamTraps,
		&p.StudyStreakDays, &p.TotalStudyMinutes, &p.TotalQuizzesTaken,
		&p.AverageQuizScore, &p.LastActiveAt, &p.UpdatedAt,
	)
	return p, err
}
