package repository

import (
	"context"
	"fmt"

	"goProjectStructure/internal/handler/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ChatRepository interface {
	SaveMessage(ctx context.Context, msg *model.ChatMessage) error
	ListMessages(ctx context.Context, userID, scopeType, scopeID string, limit int) ([]*model.ChatMessage, error)
}

type chatRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewChatRepo(db *pgxpool.Pool, logger *zap.Logger) ChatRepository {
	return &chatRepo{db: db, logger: logger}
}

func (r *chatRepo) SaveMessage(ctx context.Context, msg *model.ChatMessage) error {
	query := `
		INSERT INTO chat_messages (user_id, scope_type, scope_id, role, content, image_url, audio_url, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING id, created_at
	`
	return r.db.QueryRow(ctx, query,
		msg.UserID, msg.ScopeType, msg.ScopeID, msg.Role,
		msg.Content, msg.ImageURL, msg.AudioURL,
	).Scan(&msg.ID, &msg.CreatedAt)
}

func (r *chatRepo) ListMessages(ctx context.Context, userID, scopeType, scopeID string, limit int) ([]*model.ChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT id, user_id, scope_type, scope_id, role, content, COALESCE(image_url, ''), COALESCE(audio_url, ''), created_at
		FROM chat_messages
		WHERE user_id = $1 AND scope_type = $2 AND scope_id = $3
		ORDER BY created_at ASC
		LIMIT $4
	`
	rows, err := r.db.Query(ctx, query, userID, scopeType, scopeID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat messages: %w", err)
	}
	defer rows.Close()

	messages := []*model.ChatMessage{}
	for rows.Next() {
		m := &model.ChatMessage{}
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.ScopeType, &m.ScopeID,
			&m.Role, &m.Content, &m.ImageURL, &m.AudioURL,
			&m.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}
