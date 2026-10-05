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
	CreateSession(ctx context.Context, session *model.ChatSession) error
	GetSession(ctx context.Context, userID, sessionID string) (*model.ChatSession, error)
	ListSessions(ctx context.Context, userID string, limit int) ([]*model.ChatSession, error)
	TouchSession(ctx context.Context, userID, sessionID string) error
	UpdateSession(ctx context.Context, userID, sessionID, title string) error
	DeleteSession(ctx context.Context, userID, sessionID string) error
	ListSessionMessages(ctx context.Context, userID, sessionID string, limit int) ([]*model.ChatMessage, error)
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

func (r *chatRepo) CreateSession(ctx context.Context, session *model.ChatSession) error {
	query := `
		INSERT INTO chat_sessions (id, user_id, title, created_at, updated_at)
		VALUES (COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, NOW(), NOW())
		RETURNING id::text, created_at, updated_at
	`
	return r.db.QueryRow(ctx, query, session.ID, session.UserID, session.Title).
		Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt)
}

func (r *chatRepo) GetSession(ctx context.Context, userID, sessionID string) (*model.ChatSession, error) {
	query := `
		SELECT id::text, user_id::text, title, created_at, updated_at
		FROM chat_sessions
		WHERE user_id = $1 AND id = $2::uuid
	`
	s := &model.ChatSession{}
	err := r.db.QueryRow(ctx, query, userID, sessionID).Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *chatRepo) ListSessions(ctx context.Context, userID string, limit int) ([]*model.ChatSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT s.id::text, s.user_id::text, s.title, s.created_at, s.updated_at,
		       COUNT(m.id) as message_count,
		       COALESCE((
		           SELECT content FROM chat_messages 
		           WHERE user_id = s.user_id AND scope_id = s.id::text
		           ORDER BY created_at DESC LIMIT 1
		       ), '') as last_message
		FROM chat_sessions s
		LEFT JOIN chat_messages m ON m.user_id = s.user_id AND m.scope_id = s.id::text
		WHERE s.user_id = $1
		GROUP BY s.id, s.user_id, s.title, s.created_at, s.updated_at
		ORDER BY s.updated_at DESC
		LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query chat sessions: %w", err)
	}
	defer rows.Close()

	sessions := []*model.ChatSession{}
	for rows.Next() {
		s := &model.ChatSession{}
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt,
			&s.MessageCount, &s.LastMessage,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *chatRepo) TouchSession(ctx context.Context, userID, sessionID string) error {
	query := `UPDATE chat_sessions SET updated_at = NOW() WHERE user_id = $1 AND id = $2::uuid`
	_, err := r.db.Exec(ctx, query, userID, sessionID)
	return err
}

func (r *chatRepo) UpdateSession(ctx context.Context, userID, sessionID, title string) error {
	query := `UPDATE chat_sessions SET title = $3, updated_at = NOW() WHERE user_id = $1 AND id = $2::uuid`
	_, err := r.db.Exec(ctx, query, userID, sessionID, title)
	return err
}

func (r *chatRepo) DeleteSession(ctx context.Context, userID, sessionID string) error {
	_, _ = r.db.Exec(ctx, `DELETE FROM chat_messages WHERE user_id = $1 AND scope_id = $2`, userID, sessionID)
	_, err := r.db.Exec(ctx, `DELETE FROM chat_sessions WHERE user_id = $1 AND id = $2::uuid`, userID, sessionID)
	return err
}

func (r *chatRepo) ListSessionMessages(ctx context.Context, userID, sessionID string, limit int) ([]*model.ChatMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `
		SELECT id, user_id, scope_type, scope_id, role, content, COALESCE(image_url, ''), COALESCE(audio_url, ''), created_at
		FROM chat_messages
		WHERE user_id = $1 AND scope_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := r.db.Query(ctx, query, userID, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query session messages: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}
