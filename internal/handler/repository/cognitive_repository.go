package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/reuse"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type CognitiveRepository interface {
	GetByUserID(ctx context.Context, userID string) (*model.CognitiveProfile, error)
	Upsert(ctx context.Context, profile *model.CognitiveProfile) error
	RecordQuizResult(ctx context.Context, userID string, percentage float64, weakTopics, strongTopics, examTraps []string) (*model.CognitiveProfile, error)
	RecordActivity(ctx context.Context, userID, activityType, topic string, durationMinutes int) (*model.CognitiveProfile, error)
	PromoteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error)
	DemoteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error)
	DeleteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error)
	SaveDiagnosis(ctx context.Context, userID, summary string, readinessScore int, topicMastery map[string]int, traps, recommendedActions []string) (*model.CognitiveProfile, error)
	GetCurriculum(ctx context.Context, examCode string) (*model.ExamCurriculum, error)
	ListCurricula(ctx context.Context) ([]*model.ExamCurriculum, error)
}

type cognitiveRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewCognitiveRepo(db *pgxpool.Pool, logger *zap.Logger) CognitiveRepository {
	return &cognitiveRepo{db: db, logger: logger}
}

func (r *cognitiveRepo) GetByUserID(ctx context.Context, userID string) (*model.CognitiveProfile, error) {
	cacheKey := "cognitive_profile:" + userID
	cache := reuse.GetCache()
	if val, found := cache.Get(cacheKey); found {
		if p, ok := val.(*model.CognitiveProfile); ok {
			return p, nil
		}
	}

	query := `
		SELECT
			user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps,
			study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score,
			COALESCE(ai_summary, ''),
			COALESCE(topic_mastery, '{}'::jsonb),
			COALESCE(readiness_score, 50),
			COALESCE(recommended_actions, '[]'::jsonb),
			COALESCE(total_doubts_asked, 0),
			last_active_at, updated_at
		FROM cognitive_profiles
		WHERE user_id = $1
	`
	p := &model.CognitiveProfile{
		TopicMastery:       make(map[string]int),
		RecommendedActions: []string{},
	}
	var rawMastery []byte
	var rawActions []byte

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&p.UserID, &p.TargetExam, &p.AcademicLevel,
		&p.WeakTopics, &p.StrongTopics, &p.ExamTraps,
		&p.StudyStreakDays, &p.TotalStudyMinutes, &p.TotalQuizzesTaken,
		&p.AverageQuizScore, &p.AISummary, &rawMastery, &p.ReadinessScore,
		&rawActions, &p.TotalDoubtsAsked, &p.LastActiveAt, &p.UpdatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		defaultProfile := &model.CognitiveProfile{
			UserID:             userID,
			TargetExam:         "General / University",
			AcademicLevel:      "Undergraduate",
			WeakTopics:         []string{},
			StrongTopics:       []string{},
			ExamTraps:          []string{},
			StudyStreakDays:    1,
			TotalStudyMinutes:  0,
			TotalQuizzesTaken:  0,
			AverageQuizScore:   0.0,
			AISummary:          "Student is actively building conceptual foundations across their syllabus.",
			TopicMastery:       make(map[string]int),
			ReadinessScore:     50,
			RecommendedActions: []string{"Solve doubts in AI Tutor", "Generate an adaptive active-recall quiz", "Create a visual mind map"},
			TotalDoubtsAsked:   0,
		}
		if err := r.Upsert(ctx, defaultProfile); err != nil {
			return nil, err
		}
		return defaultProfile, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get cognitive profile: %w", err)
	}

	if len(rawMastery) > 0 {
		_ = json.Unmarshal(rawMastery, &p.TopicMastery)
	}
	if len(rawActions) > 0 {
		_ = json.Unmarshal(rawActions, &p.RecommendedActions)
	}

	cache.Set(cacheKey, p, 5*time.Minute)
	return p, nil
}

func (r *cognitiveRepo) Upsert(ctx context.Context, profile *model.CognitiveProfile) error {
	query := `
		INSERT INTO cognitive_profiles (
			user_id, target_exam, academic_level, weak_topics, strong_topics, exam_traps,
			study_streak_days, total_study_minutes, total_quizzes_taken, average_quiz_score,
			ai_summary, topic_mastery, readiness_score, recommended_actions, total_doubts_asked,
			last_active_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			target_exam = EXCLUDED.target_exam,
			academic_level = EXCLUDED.academic_level,
			weak_topics = EXCLUDED.weak_topics,
			strong_topics = EXCLUDED.strong_topics,
			exam_traps = EXCLUDED.exam_traps,
			study_streak_days = EXCLUDED.study_streak_days,
			total_study_minutes = EXCLUDED.total_study_minutes,
			total_quizzes_taken = EXCLUDED.total_quizzes_taken,
			average_quiz_score = EXCLUDED.average_quiz_score,
			ai_summary = EXCLUDED.ai_summary,
			topic_mastery = EXCLUDED.topic_mastery,
			readiness_score = EXCLUDED.readiness_score,
			recommended_actions = EXCLUDED.recommended_actions,
			total_doubts_asked = EXCLUDED.total_doubts_asked,
			updated_at = NOW()
		RETURNING last_active_at, updated_at
	`
	if profile.WeakTopics == nil {
		profile.WeakTopics = []string{}
	}
	if profile.StrongTopics == nil {
		profile.StrongTopics = []string{}
	}
	if profile.ExamTraps == nil {
		profile.ExamTraps = []string{}
	}
	if profile.TopicMastery == nil {
		profile.TopicMastery = make(map[string]int)
	}
	if profile.RecommendedActions == nil {
		profile.RecommendedActions = []string{}
	}

	masteryBytes, _ := json.Marshal(profile.TopicMastery)
	actionsBytes, _ := json.Marshal(profile.RecommendedActions)

	err := r.db.QueryRow(ctx, query,
		profile.UserID, profile.TargetExam, profile.AcademicLevel,
		profile.WeakTopics, profile.StrongTopics, profile.ExamTraps,
		profile.StudyStreakDays, profile.TotalStudyMinutes, profile.TotalQuizzesTaken,
		profile.AverageQuizScore, profile.AISummary, masteryBytes, profile.ReadinessScore,
		actionsBytes, profile.TotalDoubtsAsked,
	).Scan(&profile.LastActiveAt, &profile.UpdatedAt)

	if err == nil {
		reuse.GetCache().Set("cognitive_profile:"+profile.UserID, profile, 5*time.Minute)
	}
	return err
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
			if current.TopicMastery != nil {
				current.TopicMastery[t] = int(percentage)
			}
		}
	}

	strongSet := make(map[string]bool)
	for _, t := range current.StrongTopics {
		strongSet[t] = true
	}
	for _, t := range strongTopics {
		if t != "" {
			strongSet[t] = true
			delete(weakSet, t) // promoted
			if current.TopicMastery != nil {
				current.TopicMastery[t] = 90
			}
		}
	}

	mergedWeak := make([]string, 0, len(weakSet))
	for t := range weakSet {
		mergedWeak = append(mergedWeak, t)
	}

	mergedStrong := make([]string, 0, len(strongSet))
	for t := range strongSet {
		mergedStrong = append(mergedStrong, t)
	}

	// Dynamic readiness score calculation
	totalTopics := len(mergedStrong) + len(mergedWeak)
	readiness := int(newAvg)
	if totalTopics > 0 {
		ratioScore := (float64(len(mergedStrong)) / float64(totalTopics)) * 100.0
		readiness = int((newAvg * 0.6) + (ratioScore * 0.4))
	}
	if readiness > 100 {
		readiness = 100
	} else if readiness < 10 {
		readiness = 10
	}

	current.TotalQuizzesTaken = newTotal
	current.AverageQuizScore = newAvg
	current.WeakTopics = mergedWeak
	current.StrongTopics = mergedStrong
	current.ReadinessScore = readiness
	current.StudyStreakDays = calculateStreak(current.LastActiveAt, current.StudyStreakDays)

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *cognitiveRepo) RecordActivity(ctx context.Context, userID, activityType, topic string, durationMinutes int) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	current.TotalStudyMinutes += durationMinutes
	current.StudyStreakDays = calculateStreak(current.LastActiveAt, current.StudyStreakDays)

	if activityType == "doubt" {
		current.TotalDoubtsAsked++
		if topic != "" {
			// If not already in strong topics or weak topics, tag into weak topics for AI reinforcement
			hasInStrong := false
			for _, st := range current.StrongTopics {
				if st == topic {
					hasInStrong = true
					break
				}
			}
			if !hasInStrong {
				hasInWeak := false
				for _, wt := range current.WeakTopics {
					if wt == topic {
						hasInWeak = true
						break
					}
				}
				if !hasInWeak {
					current.WeakTopics = append(current.WeakTopics, topic)
					if current.TopicMastery == nil {
						current.TopicMastery = make(map[string]int)
					}
					current.TopicMastery[topic] = 45 // newly probed concept
				}
			}
		}
	}

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *cognitiveRepo) PromoteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Remove from weak
	newWeak := make([]string, 0, len(current.WeakTopics))
	for _, w := range current.WeakTopics {
		if w != topic {
			newWeak = append(newWeak, w)
		}
	}
	current.WeakTopics = newWeak

	// Add to strong
	alreadyInStrong := false
	for _, s := range current.StrongTopics {
		if s == topic {
			alreadyInStrong = true
			break
		}
	}
	if !alreadyInStrong {
		current.StrongTopics = append(current.StrongTopics, topic)
	}

	if current.TopicMastery == nil {
		current.TopicMastery = make(map[string]int)
	}
	current.TopicMastery[topic] = 90

	// Recompute readiness
	totalTopics := len(current.StrongTopics) + len(current.WeakTopics)
	if totalTopics > 0 {
		ratioScore := (float64(len(current.StrongTopics)) / float64(totalTopics)) * 100.0
		current.ReadinessScore = int((current.AverageQuizScore * 0.5) + (ratioScore * 0.5))
		if current.ReadinessScore < 20 {
			current.ReadinessScore = 20
		}
	}

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *cognitiveRepo) DemoteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Remove from strong
	newStrong := make([]string, 0, len(current.StrongTopics))
	for _, s := range current.StrongTopics {
		if s != topic {
			newStrong = append(newStrong, s)
		}
	}
	current.StrongTopics = newStrong

	// Add to weak
	alreadyInWeak := false
	for _, w := range current.WeakTopics {
		if w == topic {
			alreadyInWeak = true
			break
		}
	}
	if !alreadyInWeak {
		current.WeakTopics = append(current.WeakTopics, topic)
	}

	if current.TopicMastery == nil {
		current.TopicMastery = make(map[string]int)
	}
	current.TopicMastery[topic] = 40

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *cognitiveRepo) DeleteTopic(ctx context.Context, userID, topic string) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	newWeak := make([]string, 0, len(current.WeakTopics))
	for _, w := range current.WeakTopics {
		if w != topic {
			newWeak = append(newWeak, w)
		}
	}
	current.WeakTopics = newWeak

	newStrong := make([]string, 0, len(current.StrongTopics))
	for _, s := range current.StrongTopics {
		if s != topic {
			newStrong = append(newStrong, s)
		}
	}
	current.StrongTopics = newStrong

	if current.TopicMastery != nil {
		delete(current.TopicMastery, topic)
	}

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *cognitiveRepo) SaveDiagnosis(
	ctx context.Context,
	userID, summary string,
	readinessScore int,
	topicMastery map[string]int,
	traps, recommendedActions []string,
) (*model.CognitiveProfile, error) {
	current, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	current.AISummary = summary
	current.ReadinessScore = readinessScore
	if topicMastery != nil {
		if current.TopicMastery == nil {
			current.TopicMastery = make(map[string]int)
		}
		for k, v := range topicMastery {
			current.TopicMastery[k] = v
		}
	}
	if len(traps) > 0 {
		trapSet := make(map[string]bool)
		for _, t := range current.ExamTraps {
			trapSet[t] = true
		}
		for _, t := range traps {
			trapSet[t] = true
		}
		current.ExamTraps = make([]string, 0, len(trapSet))
		for t := range trapSet {
			current.ExamTraps = append(current.ExamTraps, t)
		}
	}
	if len(recommendedActions) > 0 {
		current.RecommendedActions = recommendedActions
	}

	if err := r.Upsert(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func calculateStreak(lastActive time.Time, currentStreak int) int {
	now := time.Now()
	// Normalize to days
	lastDay := time.Date(lastActive.Year(), lastActive.Month(), lastActive.Day(), 0, 0, 0, 0, time.UTC)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	diffDays := int(today.Sub(lastDay).Hours() / 24)

	if diffDays == 0 {
		if currentStreak <= 0 {
			return 1
		}
		return currentStreak
	} else if diffDays == 1 {
		return currentStreak + 1
	}
	return 1 // reset after missing a day
}

// GetCurriculum retrieves an exam curriculum by its unique code (e.g. "gate_cs", "neet_medical")
func (r *cognitiveRepo) GetCurriculum(ctx context.Context, examCode string) (*model.ExamCurriculum, error) {
	if r.db == nil {
		return nil, errors.New("database connection pool not initialized")
	}

	query := `
		SELECT id, exam_code, display_name, keywords, high_yield_topics, common_traps, created_at, updated_at
		FROM exam_curricula
		WHERE LOWER(exam_code) = LOWER($1)
		LIMIT 1;
	`
	c := &model.ExamCurriculum{}
	err := r.db.QueryRow(ctx, query, examCode).Scan(
		&c.ID, &c.ExamCode, &c.DisplayName,
		&c.Keywords, &c.HighYieldTopics, &c.CommonTraps,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListCurricula fetches all active academic curricula configured in PostgreSQL
func (r *cognitiveRepo) ListCurricula(ctx context.Context) ([]*model.ExamCurriculum, error) {
	if r.db == nil {
		return nil, errors.New("database connection pool not initialized")
	}

	query := `
		SELECT id, exam_code, display_name, keywords, high_yield_topics, common_traps, created_at, updated_at
		FROM exam_curricula
		ORDER BY display_name ASC;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.ExamCurriculum
	for rows.Next() {
		c := &model.ExamCurriculum{}
		if err := rows.Scan(
			&c.ID, &c.ExamCode, &c.DisplayName,
			&c.Keywords, &c.HighYieldTopics, &c.CommonTraps,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
