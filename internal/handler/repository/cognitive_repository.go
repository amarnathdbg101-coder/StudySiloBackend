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

type CognitiveRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.CognitiveProfile, error)
	Upsert(ctx context.Context, profile *model.CognitiveProfile) error
	RecordQuizResult(ctx context.Context, userID string, percentage float64, weakTopics, strongTopics, examTraps []string) (*model.CognitiveProfile, error)
}

type cognitiveRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewCognitiveRepo(db *pgxpool.Pool, logger *zap.Logger) CognitiveRepository {
	return &cognitiveRepo{db: db, logger: logger}
}

func (r *cognitiveRepo) GetByUserID(ctx context.Context, userID string) (*model.CognitiveProfile, error) {
	query := `
		SELECT user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps, study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score, last_active_at, updated_at
		FROM cognitive_profiles
		WHERE user_id = $1
	`
	p := &model.CognitiveProfile{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.UserID, &p.TargetExam, &p.AcademicLevel,
		&p.WeakTopics, &p.StrongTopics, &p.ExamTraps,
		&p.StudyStreakDays, &p.TotalStudyMinutes, &p.TotalQuizzesTaken,
		&p.AverageQuizScore, &p.LastActiveAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		// Auto-initialize default cognitive profile for new student
		defaultProfile := &model.CognitiveProfile{
			UserID:            userID,
			TargetExam:        "General / University",
			AcademicLevel:     "Undergraduate",
			WeakTopics:        []string{},
			StrongTopics:      []string{},
			ExamTraps:         []string{},
			StudyStreakDays:   1,
			TotalStudyMinutes: 0,
			TotalQuizzesTaken: 0,
			AverageQuizScore:  0.0,
		}
		if err := r.Upsert(ctx, defaultProfile); err != nil {
			return nil, err
		}
		return defaultProfile, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cognitive profile: %w", err)
	}
	return p, nil
}

func (r *cognitiveRepo) Upsert(ctx context.Context, profile *model.CognitiveProfile) error {
	query := `
		INSERT INTO cognitive_profiles (
			user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps,
			study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score,
			last_active_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			target_exam = EXCLUDED.target_exam,
			academic_level = EXCLUDED.academic_level,
			updated_at = NOW()
		RETURNING last_active_at, updated_at
	`
	return r.db.QueryRow(ctx, query,
		profile.UserID, profile.TargetExam, profile.AcademicLevel,
		profile.WeakTopics, profile.StrongTopics, profile.ExamTraps,
		profile.StudyStreakDays, profile.TotalStudyMinutes, profile.TotalQuizzesTaken,
		profile.AverageQuizScore,
	).Scan(&profile.LastActiveAt, &profile.UpdatedAt)
}

func (r *cognitiveRepo) RecordQuizResult(
	ctx context.Context,
	userID string,
	percentage float64,
	weakTopics, strongTopics, examTraps []string,
) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	newTotal := current.TotalQuizzesTaken + 1
	newAvg := ((current.AverageQuizScore * float64(current.TotalQuizzesTaken)) + percentage) / float64(newTotal)

	// Merge unique weak and strong topics
	weakSet := make(map[string]bool)
	for _, t := range current.WeakTopics {
		weakSet[t] = true
	}
	for _, t := range weakTopics {
		if t != "" {
			weakSet[t] = true
		}
	}
	mergedWeak := make([]string, 0, len(weakSet))
	for t := range weakSet {
		mergedWeak = append(mergedWeak, t)
	}

	strongSet := make(map[string]bool)
	for _, t := range current.StrongTopics {
		strongSet[t] = true
	}
	for _, t := range strongTopics {
		if t != "" {
			strongSet[t] = true
		}
	}
	mergedStrong := make([]string, 0, len(strongSet))
	for t := range strongSet {
		mergedStrong = append(mergedStrong, t)
	}

	// Update query
	query := `
		UPDATE cognitive_profiles
		SET total_quizzes_taken = $1,
		    average_quiz_score = $2,
		    weak_topics = $3,
		    strong_topics = $4,
		    study_streak_days = GREATEST(study_streak_days, 1),
		    last_active_at = NOW(),
		    updated_at = NOW()
		WHERE user_id = $5
		RETURNING user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps, study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score, last_active_at, updated_at
	`
	updated := &model.CognitiveProfile{}
	err = r.db.QueryRow(ctx, query, newTotal, newAvg, mergedWeak, mergedStrong, userID).Scan(
		&updated.UserID, &updated.TargetExam, &updated.AcademicLevel,
		&updated.WeakTopics, &updated.StrongTopics, &updated.ExamTraps,
		&updated.StudyStreakDays, &updated.TotalStudyMinutes, &updated.TotalQuizzesTaken,
		&updated.AverageQuizScore, &updated.LastActiveAt, &updated.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record quiz result: %w", err)
	}
	return updated, nil
}
