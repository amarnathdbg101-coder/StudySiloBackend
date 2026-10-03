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

type QuizRepository interface {
	CreateQuizWithQuestions(ctx context.Context, quiz *model.Quiz, questions []*model.QuizQuestion) error
	GetQuizByID(ctx context.Context, id, userID string) (*model.Quiz, error)
	ListQuizzesByUser(ctx context.Context, userID string) ([]*model.Quiz, error)
	SaveAttempt(ctx context.Context, attempt *model.QuizAttempt) error
	ListAttemptsByUser(ctx context.Context, userID, quizID string) ([]*model.QuizAttempt, error)
}

type quizRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewQuizRepo(db *pgxpool.Pool, logger *zap.Logger) QuizRepository {
	return &quizRepo{db: db, logger: logger}
}

func (r *quizRepo) CreateQuizWithQuestions(ctx context.Context, quiz *model.Quiz, questions []*model.QuizQuestion) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Insert Quiz
	quizQuery := `
		INSERT INTO quizzes (user_id, course_id, note_id, title, topic, difficulty, total_questions, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, created_at
	`
	if err := tx.QueryRow(ctx, quizQuery,
		quiz.UserID, quiz.CourseID, quiz.NoteID,
		quiz.Title, quiz.Topic, quiz.Difficulty, len(questions),
	).Scan(&quiz.ID, &quiz.CreatedAt); err != nil {
		return fmt.Errorf("failed to insert quiz: %w", err)
	}

	// 2. Insert Questions
	qQuery := `
		INSERT INTO quiz_questions (quiz_id, question_text, options, correct_option_index, explanation, trap_warning, difficulty, topic, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, created_at
	`
	for _, q := range questions {
		q.QuizID = quiz.ID
		optJSON, _ := json.Marshal(q.Options)
		if err := tx.QueryRow(ctx, qQuery,
			quiz.ID, q.QuestionText, optJSON, q.CorrectOptionIndex,
			q.Explanation, q.TrapWarning, q.Difficulty, q.Topic,
		).Scan(&q.ID, &q.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert question: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	quiz.Questions = questions
	return nil
}

func (r *quizRepo) GetQuizByID(ctx context.Context, id, userID string) (*model.Quiz, error) {
	query := `
		SELECT id, user_id, course_id, note_id, title, topic, difficulty, total_questions, created_at
		FROM quizzes
		WHERE id = $1 AND user_id = $2
	`
	q := &model.Quiz{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&q.ID, &q.UserID, &q.CourseID, &q.NoteID,
		&q.Title, &q.Topic, &q.Difficulty, &q.TotalQuestions,
		&q.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query quiz: %w", err)
	}

	// Load questions
	qqQuery := `
		SELECT id, quiz_id, question_text, options, correct_option_index, explanation, trap_warning, difficulty, topic, created_at
		FROM quiz_questions
		WHERE quiz_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, qqQuery, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	questions := []*model.QuizQuestion{}
	for rows.Next() {
		qq := &model.QuizQuestion{}
		var optJSON []byte
		if err := rows.Scan(
			&qq.ID, &qq.QuizID, &qq.QuestionText, &optJSON,
			&qq.CorrectOptionIndex, &qq.Explanation, &qq.TrapWarning,
			&qq.Difficulty, &qq.Topic, &qq.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(optJSON, &qq.Options)
		questions = append(questions, qq)
	}
	q.Questions = questions

	return q, nil
}

func (r *quizRepo) ListQuizzesByUser(ctx context.Context, userID string) ([]*model.Quiz, error) {
	query := `
		SELECT id, user_id, course_id, note_id, title, topic, difficulty, total_questions, created_at
		FROM quizzes
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query quizzes: %w", err)
	}
	defer rows.Close()

	quizzes := []*model.Quiz{}
	for rows.Next() {
		q := &model.Quiz{}
		if err := rows.Scan(
			&q.ID, &q.UserID, &q.CourseID, &q.NoteID,
			&q.Title, &q.Topic, &q.Difficulty, &q.TotalQuestions,
			&q.CreatedAt,
		); err != nil {
			return nil, err
		}
		quizzes = append(quizzes, q)
	}
	return quizzes, rows.Err()
}

func (r *quizRepo) SaveAttempt(ctx context.Context, attempt *model.QuizAttempt) error {
	answersJSON, _ := json.Marshal(attempt.Answers)
	query := `
		INSERT INTO quiz_attempts (quiz_id, user_id, score, total_score, percentage, answers, weak_areas_identified, time_taken_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		attempt.QuizID, attempt.UserID, attempt.Score, attempt.TotalScore,
		attempt.Percentage, answersJSON, attempt.WeakAreasIdentified,
		attempt.TimeTakenSeconds,
	).Scan(&attempt.ID, &attempt.CreatedAt)
}

func (r *quizRepo) ListAttemptsByUser(ctx context.Context, userID, quizID string) ([]*model.QuizAttempt, error) {
	query := `
		SELECT id, quiz_id, user_id, score, total_score, percentage, answers, weak_areas_identified, time_taken_seconds, created_at
		FROM quiz_attempts
		WHERE user_id = $1 AND ($2 = '' OR quiz_id = $2::uuid)
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID, quizID)
	if err != nil {
		return nil, fmt.Errorf("failed to query quiz attempts: %w", err)
	}
	defer rows.Close()

	attempts := []*model.QuizAttempt{}
	for rows.Next() {
		a := &model.QuizAttempt{}
		var answersJSON []byte
		if err := rows.Scan(
			&a.ID, &a.QuizID, &a.UserID, &a.Score, &a.TotalScore,
			&a.Percentage, &answersJSON, &a.WeakAreasIdentified,
			&a.TimeTakenSeconds, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(answersJSON, &a.Answers)
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}
