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
	ErrFlashcardDeckNotFound = errors.New("flashcard deck not found")
)

type FlashcardRepository interface {
	Create(ctx context.Context, deck *model.FlashcardDeck) error
	GetByID(ctx context.Context, userID, id string) (*model.FlashcardDeck, error)
	List(ctx context.Context, userID string, search string, limit, offset int) ([]*model.FlashcardDeck, int, error)
	Update(ctx context.Context, userID, id string, title *string, cards json.RawMessage, cardCount *int, masteredCount *int, isPinned *bool) (*model.FlashcardDeck, error)
	Delete(ctx context.Context, userID, id string) error
	TogglePin(ctx context.Context, userID, id string) (*model.FlashcardDeck, error)
}

type flashcardRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewFlashcardRepo(db *pgxpool.Pool, logger *zap.Logger) FlashcardRepository {
	return &flashcardRepo{db: db, logger: logger}
}

func (r *flashcardRepo) Create(ctx context.Context, d *model.FlashcardDeck) error {
	query := `
		INSERT INTO flashcard_decks (user_id, title, topic, difficulty, card_count, cards, mastered_count, is_pinned, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	if d.Difficulty == "" {
		d.Difficulty = "medium"
	}
	if len(d.Cards) == 0 {
		d.Cards = json.RawMessage("[]")
	}

	err := r.db.QueryRow(ctx, query,
		d.UserID, d.Title, d.Topic, d.Difficulty, d.CardCount, d.Cards, d.MasteredCount, d.IsPinned,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create flashcard deck: %w", err)
	}
	return nil
}

func (r *flashcardRepo) GetByID(ctx context.Context, userID, id string) (*model.FlashcardDeck, error) {
	query := `
		SELECT id, user_id, title, topic, difficulty, card_count, cards, mastered_count, is_pinned, created_at, updated_at
		FROM flashcard_decks
		WHERE id = $1 AND user_id = $2
	`
	d := &model.FlashcardDeck{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&d.ID, &d.UserID, &d.Title, &d.Topic, &d.Difficulty, &d.CardCount, &d.Cards, &d.MasteredCount, &d.IsPinned, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlashcardDeckNotFound
		}
		return nil, fmt.Errorf("failed to get flashcard deck: %w", err)
	}
	return d, nil
}

func (r *flashcardRepo) List(ctx context.Context, userID string, search string, limit, offset int) ([]*model.FlashcardDeck, int, error) {
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
			FROM flashcard_decks
			WHERE user_id = $1 AND (title ILIKE $2 OR topic ILIKE $2)
		`
		countArgs = []interface{}{userID, searchTerm}

		listQuery = `
			SELECT id, user_id, title, topic, difficulty, card_count, cards, mastered_count, is_pinned, created_at, updated_at
			FROM flashcard_decks
			WHERE user_id = $1 AND (title ILIKE $2 OR topic ILIKE $2)
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $3 OFFSET $4
		`
		listArgs = []interface{}{userID, searchTerm, limit, offset}
	} else {
		countQuery = `
			SELECT COUNT(*)
			FROM flashcard_decks
			WHERE user_id = $1
		`
		countArgs = []interface{}{userID}

		listQuery = `
			SELECT id, user_id, title, topic, difficulty, card_count, cards, mastered_count, is_pinned, created_at, updated_at
			FROM flashcard_decks
			WHERE user_id = $1
			ORDER BY is_pinned DESC, updated_at DESC
			LIMIT $2 OFFSET $3
		`
		listArgs = []interface{}{userID, limit, offset}
	}

	var total int
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count flashcard decks: %w", err)
	}

	rows, err := r.db.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list flashcard decks: %w", err)
	}
	defer rows.Close()

	var result []*model.FlashcardDeck
	for rows.Next() {
		d := &model.FlashcardDeck{}
		if err := rows.Scan(
			&d.ID, &d.UserID, &d.Title, &d.Topic, &d.Difficulty, &d.CardCount, &d.Cards, &d.MasteredCount, &d.IsPinned, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan flashcard deck row: %w", err)
		}
		result = append(result, d)
	}

	return result, total, nil
}

func (r *flashcardRepo) Update(ctx context.Context, userID, id string, title *string, cards json.RawMessage, cardCount *int, masteredCount *int, isPinned *bool) (*model.FlashcardDeck, error) {
	current, err := r.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if title != nil {
		current.Title = *title
	}
	if len(cards) > 0 {
		current.Cards = cards
	}
	if cardCount != nil {
		current.CardCount = *cardCount
	}
	if masteredCount != nil {
		current.MasteredCount = *masteredCount
	}
	if isPinned != nil {
		current.IsPinned = *isPinned
	}

	query := `
		UPDATE flashcard_decks
		SET title = $1, cards = $2, card_count = $3, mastered_count = $4, is_pinned = $5, updated_at = NOW()
		WHERE id = $6 AND user_id = $7
		RETURNING updated_at
	`
	err = r.db.QueryRow(ctx, query,
		current.Title, current.Cards, current.CardCount, current.MasteredCount, current.IsPinned, id, userID,
	).Scan(&current.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update flashcard deck: %w", err)
	}

	return current, nil
}

func (r *flashcardRepo) Delete(ctx context.Context, userID, id string) error {
	query := `DELETE FROM flashcard_decks WHERE id = $1 AND user_id = $2`
	cmdTag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete flashcard deck: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrFlashcardDeckNotFound
	}
	return nil
}

func (r *flashcardRepo) TogglePin(ctx context.Context, userID, id string) (*model.FlashcardDeck, error) {
	query := `
		UPDATE flashcard_decks
		SET is_pinned = NOT is_pinned, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, title, topic, difficulty, card_count, cards, mastered_count, is_pinned, created_at, updated_at
	`
	d := &model.FlashcardDeck{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&d.ID, &d.UserID, &d.Title, &d.Topic, &d.Difficulty, &d.CardCount, &d.Cards, &d.MasteredCount, &d.IsPinned, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrFlashcardDeckNotFound
		}
		return nil, fmt.Errorf("failed to toggle pin on flashcard deck: %w", err)
	}
	return d, nil
}
