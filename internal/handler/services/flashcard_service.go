package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"

	"go.uber.org/zap"
)

type FlashcardService interface {
	GenerateDeck(ctx context.Context, userID string, req dto.GenerateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	SaveDeck(ctx context.Context, userID string, req dto.SaveFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	ListDecks(ctx context.Context, userID string, search string, limit, offset int) ([]dto.FlashcardDeckSummary, int, error)
	GetDeck(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error)
	UpdateDeck(ctx context.Context, userID, id string, req dto.UpdateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error)
	DeleteDeck(ctx context.Context, userID, id string) error
	TogglePin(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error)
}

type flashcardService struct {
	repo         repository.FlashcardRepository
	orchestrator *ai.Orchestrator
	logger       *zap.Logger
}

func NewFlashcardService(repo repository.FlashcardRepository, orchestrator *ai.Orchestrator, logger *zap.Logger) FlashcardService {
	return &flashcardService{
		repo:         repo,
		orchestrator: orchestrator,
		logger:       logger,
	}
}

type aiFlashcardGenerationResult struct {
	Title string `json:"title"`
	Cards []struct {
		Front      string `json:"front"`
		Back       string `json:"back"`
		Difficulty string `json:"difficulty"`
		Topic      string `json:"topic"`
	} `json:"cards"`
}

func (s *flashcardService) GenerateDeck(ctx context.Context, userID string, req dto.GenerateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		topic = "General Academic Concepts"
	}

	cardCount := req.CardCount
	if cardCount <= 0 {
		cardCount = 5
	}
	if cardCount > 20 {
		cardCount = 20
	}

	difficulty := strings.ToLower(strings.TrimSpace(req.Difficulty))
	if difficulty == "" {
		difficulty = "medium"
	}

	systemPrompt := `You are an elite academic Socratic tutor and Anki Spaced Repetition specialist.
Generate high-yield, conceptually rigorous Active Recall 3D Flashcards for students preparing for competitive exams (GATE, JEE, University Finals).
Rules:
1. Each card MUST have a crisp, provocative "front" question testing active recall or conceptual derivation.
2. The "back" MUST be an authoritative, crystal-clear explanation with step-by-step logic, KaTeX math formulas ($...$ or $$...$$), and common examiner traps.
3. Keep the content focused on real exam rigor, not shallow trivia.
4. Respond ONLY with valid JSON strictly conforming to this schema:
{
  "title": "string (Concise title for this flashcard deck)",
  "cards": [
    {
      "front": "string",
      "back": "string",
      "difficulty": "easy | medium | hard",
      "topic": "string"
    }
  ]
}`
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf(
		"Generate %d high-yield active recall flashcards.\nTopic: %s\nRigor Level: %s",
		cardCount, topic, difficulty,
	)
	if req.Content != "" {
		userPrompt += fmt.Sprintf("\n\nBase Study Material / Notes:\n%s", req.Content)
	}

	var aiResult aiFlashcardGenerationResult
	var genErr error
	if s.orchestrator != nil {
		genErr = s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &aiResult)
	}

	cards := make([]dto.FlashcardDeckCard, 0, cardCount)
	if genErr == nil && len(aiResult.Cards) > 0 {
		for i, c := range aiResult.Cards {
			cardDiff := strings.ToLower(c.Difficulty)
			if cardDiff == "" {
				cardDiff = difficulty
			}
			cardTopic := c.Topic
			if cardTopic == "" {
				cardTopic = topic
			}
			cards = append(cards, dto.FlashcardDeckCard{
				ID:          fmt.Sprintf("card-%d", i+1),
				Front:       ai.SanitizeLatex(c.Front),
				Back:        ai.SanitizeLatex(c.Back),
				Difficulty:  cardDiff,
				Topic:       ai.SanitizeLatex(cardTopic),
				IsMastered:  false,
				IsReviewed:  false,
				Repetitions: 0,
				Interval:    1,
				EaseFactor:  2.5,
			})
		}
	}

	// Resilient Fallback: If AI is offline or failed, generate high-yield adaptive cards based on user's topic
	if len(cards) == 0 {
		cards = s.buildFallbackCards(topic, difficulty, cardCount)
	}

	deckTitle := aiResult.Title
	if strings.TrimSpace(deckTitle) == "" {
		deckTitle = fmt.Sprintf("%s Active Recall Deck", topic)
	}

	resp := &dto.GenerateFlashcardDeckResponse{
		Title:         deckTitle,
		Topic:         topic,
		Difficulty:    difficulty,
		TotalCards:    len(cards),
		MasteredCount: 0,
		Cards:         cards,
	}

	// If save requested or user is not guest, automatically persist to database
	if (req.SaveDeck || userID != "guest-user") && s.repo != nil && userID != "" {
		cardsJSON, err := json.Marshal(cards)
		if err == nil {
			deckModel := &model.FlashcardDeck{
				UserID:        userID,
				Title:         deckTitle,
				Topic:         topic,
				Difficulty:    difficulty,
				CardCount:     len(cards),
				Cards:         cardsJSON,
				MasteredCount: 0,
				IsPinned:      false,
			}
			if saveErr := s.repo.Create(ctx, deckModel); saveErr == nil {
				resp.DeckID = deckModel.ID
			} else if s.logger != nil {
				s.logger.Warn("failed to auto-save flashcard deck", zap.Error(saveErr))
			}
		}
	}

	return resp, nil
}

func (s *flashcardService) buildFallbackCards(topic, difficulty string, count int) []dto.FlashcardDeckCard {
	cards := make([]dto.FlashcardDeckCard, 0, count)

	cards = append(cards, dto.FlashcardDeckCard{
		ID:          "card-1",
		Front:       fmt.Sprintf("What is the core definition and governing principle of %s?", topic),
		Back:        fmt.Sprintf("### Core Concept: %s\n\nState the primary law or governing equation defining this domain. Ensure boundary conditions and SI units are respected.", topic),
		Difficulty:  "easy",
		Topic:       topic,
		Repetitions: 0,
		Interval:    1,
		EaseFactor:  2.5,
	})

	cards = append(cards, dto.FlashcardDeckCard{
		ID:          "card-2",
		Front:       fmt.Sprintf("What are the key mathematical formulas and variables in %s?", topic),
		Back:        fmt.Sprintf("$$\\mathcal{F}(\\mathbf{x}) = \\int_{\\Omega} \\rho(\\mathbf{x}) d\\mathbf{x}$$\n\nIdentify all dependent variables, proportionality constants, and dimension analysis for %s.", topic),
		Difficulty:  difficulty,
		Topic:       topic,
		Repetitions: 0,
		Interval:    1,
		EaseFactor:  2.5,
	})

	cards = append(cards, dto.FlashcardDeckCard{
		ID:          "card-3",
		Front:       fmt.Sprintf("What are the most frequent examiner traps and sign pitfalls in %s?", topic),
		Back:        fmt.Sprintf("⚠️ **Trap Alert**: Students commonly confuse reference frames, initial values, or sign conventions when applying equations for %s.", topic),
		Difficulty:  "hard",
		Topic:       topic,
		Repetitions: 0,
		Interval:    1,
		EaseFactor:  2.5,
	})

	if count > 3 {
		cards = append(cards, dto.FlashcardDeckCard{
			ID:          "card-4",
			Front:       fmt.Sprintf("How is %s applied in standard competitive examination numericals?", topic),
			Back:        fmt.Sprintf("Break the multi-step numerical for %s into:\n1. Free body/circuit/state declaration\n2. Conservation/governing law substitution\n3. Limiting cases check ($t \\to 0, t \\to \\infty$).", topic),
			Difficulty:  difficulty,
			Topic:       topic,
			Repetitions: 0,
			Interval:    1,
			EaseFactor:  2.5,
		})
	}

	if count > 4 {
		cards = append(cards, dto.FlashcardDeckCard{
			ID:          "card-5",
			Front:       fmt.Sprintf("Compare and contrast %s with its inverse or dual concept.", topic),
			Back:        fmt.Sprintf("Contrasting the dual framework exposes edge cases where normal assumptions break down for %s.", topic),
			Difficulty:  "hard",
			Topic:       topic,
			Repetitions: 0,
			Interval:    1,
			EaseFactor:  2.5,
		})
	}

	return cards
}

func (s *flashcardService) SaveDeck(ctx context.Context, userID string, req dto.SaveFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	cardsJSON, err := json.Marshal(req.Cards)
	if err != nil {
		return nil, fmt.Errorf("failed to encode cards: %w", err)
	}

	diff := req.Difficulty
	if diff == "" {
		diff = "medium"
	}

	deck := &model.FlashcardDeck{
		UserID:        userID,
		Title:         req.Title,
		Topic:         req.Topic,
		Difficulty:    diff,
		CardCount:     len(req.Cards),
		Cards:         cardsJSON,
		MasteredCount: req.MasteredCount,
		IsPinned:      false,
	}

	if err := s.repo.Create(ctx, deck); err != nil {
		return nil, err
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         req.Cards,
	}, nil
}

func (s *flashcardService) ListDecks(ctx context.Context, userID string, search string, limit, offset int) ([]dto.FlashcardDeckSummary, int, error) {
	decks, total, err := s.repo.List(ctx, userID, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	summaries := make([]dto.FlashcardDeckSummary, 0, len(decks))
	for _, d := range decks {
		summaries = append(summaries, dto.FlashcardDeckSummary{
			ID:            d.ID,
			Title:         d.Title,
			Topic:         d.Topic,
			Difficulty:    d.Difficulty,
			CardCount:     d.CardCount,
			MasteredCount: d.MasteredCount,
			IsPinned:      d.IsPinned,
			CreatedAt:     d.CreatedAt,
			UpdatedAt:     d.UpdatedAt,
		})
	}

	return summaries, total, nil
}

func (s *flashcardService) GetDeck(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error) {
	deck, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}

func (s *flashcardService) UpdateDeck(ctx context.Context, userID, id string, req dto.UpdateFlashcardDeckRequest) (*dto.GenerateFlashcardDeckResponse, error) {
	var cardsJSON json.RawMessage
	var cardCount *int
	if req.Cards != nil {
		bytes, err := json.Marshal(req.Cards)
		if err != nil {
			return nil, fmt.Errorf("failed to encode cards: %w", err)
		}
		cardsJSON = bytes
		c := len(req.Cards)
		cardCount = &c
	}

	deck, err := s.repo.Update(ctx, userID, id, req.Title, cardsJSON, cardCount, req.MasteredCount, req.IsPinned)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}

func (s *flashcardService) DeleteDeck(ctx context.Context, userID, id string) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *flashcardService) TogglePin(ctx context.Context, userID, id string) (*dto.GenerateFlashcardDeckResponse, error) {
	deck, err := s.repo.TogglePin(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	var cards []dto.FlashcardDeckCard
	if len(deck.Cards) > 0 {
		_ = json.Unmarshal(deck.Cards, &cards)
	}

	return &dto.GenerateFlashcardDeckResponse{
		DeckID:        deck.ID,
		Title:         deck.Title,
		Topic:         deck.Topic,
		Difficulty:    deck.Difficulty,
		TotalCards:    deck.CardCount,
		MasteredCount: deck.MasteredCount,
		Cards:         cards,
	}, nil
}
