package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DoubtCacheRepository defines the persistence contract for caching academic doubt solutions
type DoubtCacheRepository interface {
	Get(ctx context.Context, queryHash, question, mode, language string) (*dto.DoubtResponse, error)
	Set(ctx context.Context, queryHash, question, mode, language, modelName string, resp *dto.DoubtResponse) error
}

type doubtCacheRepo struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewDoubtCacheRepo creates a new PostgreSQL-backed doubt cache repository
func NewDoubtCacheRepo(db *pgxpool.Pool, logger *zap.Logger) DoubtCacheRepository {
	return &doubtCacheRepo{
		db:     db,
		logger: logger,
	}
}

// ComputeDoubtCacheKey computes a deterministic SHA-256 hash for standalone doubts.
// It applies strict safety guards to guarantee ZERO FALSE POSITIVES:
// 1. Never caches conversational follow-ups (history > 0).
// 2. Never caches requests with images or custom notes.
// 3. Never caches explicit regeneration/edit turns.
// 4. Requires question to be an actual academic query (length >= 15).
func ComputeDoubtCacheKey(req dto.DoubtRequest) string {
	// Guard 1: Never cache follow-up conversational questions
	if len(req.ConversationHistory) > 0 {
		return ""
	}
	// Guard 2: Never cache requests with attachments or private notes
	if req.ImageBase64 != "" || req.AudioBase64 != "" || strings.TrimSpace(req.ContextNotes) != "" {
		return ""
	}
	// Guard 3: Never cache forced regenerations or edits
	if req.RegenerateLastAnswer || req.EditLastQuestion {
		return ""
	}

	trimmedQ := strings.TrimSpace(req.Question)
	// Normalizes internal spaces (e.g., multiple spaces become single space), but preserves all characters
	words := strings.Fields(trimmedQ)
	if len(words) < 3 {
		return ""
	}
	normalizedQ := strings.ToLower(strings.Join(words, " "))
	if len(normalizedQ) < 15 {
		return ""
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "academic"
	}
	lang := strings.ToLower(strings.TrimSpace(req.AILanguage))
	if lang == "" {
		lang = strings.ToLower(strings.TrimSpace(req.Language))
	}
	if lang == "" {
		lang = "auto"
	}

	modelID := strings.TrimSpace(req.Model)

	// Hash strictly combines Mode + Language + Model + Full Normalized Question
	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("mode:%s|lang:%s|model:%s|q:%s", mode, lang, modelID, normalizedQ)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Get looks up a cached doubt solution from PostgreSQL.
// Strict Double Check: Matches both query_hash AND full question equality.
func (r *doubtCacheRepo) Get(ctx context.Context, queryHash, question, mode, language string) (*dto.DoubtResponse, error) {
	if r.db == nil || queryHash == "" {
		return nil, nil
	}

	if mode == "" {
		mode = "academic"
	}
	if language == "" {
		language = "auto"
	}

	query := `
		SELECT solution_markdown, spoken_transcript, core_concept, common_pitfalls, follow_up_chips, active_provider, model
		FROM ai_doubt_cache
		WHERE query_hash = $1
		  AND LOWER(TRIM(question)) = LOWER(TRIM($2))
		  AND mode = $3
		  AND language = $4
		LIMIT 1;
	`
	normalizedQ := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(question)), " "))

	var (
		solutionMarkdown string
		spokenTranscript string
		coreConcept      string
		commonPitfalls   string
		chipsJSON        []byte
		activeProvider   string
		modelName        string
	)

	err := r.db.QueryRow(ctx, query, queryHash, normalizedQ, mode, language).Scan(
		&solutionMarkdown,
		&spokenTranscript,
		&coreConcept,
		&commonPitfalls,
		&chipsJSON,
		&activeProvider,
		&modelName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("doubt cache lookup failed: %w", err)
	}

	// Asynchronously increment hit_count in background (doesn't block response)
	go func(hash string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = r.db.Exec(bgCtx, `UPDATE ai_doubt_cache SET hit_count = hit_count + 1, updated_at = NOW() WHERE query_hash = $1`, hash)
	}(queryHash)

	var chips []string
	if len(chipsJSON) > 0 {
		_ = json.Unmarshal(chipsJSON, &chips)
	}

	return &dto.DoubtResponse{
		Question:         question,
		SolutionMarkdown: solutionMarkdown,
		SpokenTranscript: spokenTranscript,
		CoreConcept:      coreConcept,
		CommonPitfalls:   commonPitfalls,
		FollowUpChips:    chips,
		ActiveProvider:   activeProvider,
		ActiveModel:      modelName,
	}, nil
}

// Set stores a freshly resolved doubt solution in the PostgreSQL cache.
func (r *doubtCacheRepo) Set(ctx context.Context, queryHash, question, mode, language, modelName string, resp *dto.DoubtResponse) error {
	if r.db == nil || queryHash == "" || resp == nil || resp.SolutionMarkdown == "" {
		return nil
	}

	if mode == "" {
		mode = "academic"
	}
	if language == "" {
		language = "auto"
	}

	chipsJSON, _ := json.Marshal(resp.FollowUpChips)
	normalizedQ := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(question)), " "))

	query := `
		INSERT INTO ai_doubt_cache (
			query_hash, question, mode, language, model, active_provider,
			solution_markdown, spoken_transcript, core_concept, common_pitfalls,
			follow_up_chips, hit_count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, 1, NOW(), NOW()
		)
		ON CONFLICT (query_hash) DO UPDATE SET
			hit_count = ai_doubt_cache.hit_count + 1,
			updated_at = NOW();
	`
	_, err := r.db.Exec(ctx, query,
		queryHash, normalizedQ, mode, language, modelName, resp.ActiveProvider,
		resp.SolutionMarkdown, resp.SpokenTranscript, resp.CoreConcept, resp.CommonPitfalls,
		chipsJSON,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to cache doubt response in PostgreSQL", zap.Error(err))
		}
	}
	return err
}
