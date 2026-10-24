package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"
)

type NotesAnalyzerService interface {
	AnalyzeNotes(ctx context.Context, userID string, req dto.NotesAnalysisRequest) (*dto.NotesAnalysisResponse, error)
}

type notesAnalyzerService struct {
	orchestrator *ai.Orchestrator
	noteRepo     repository.NoteRepository
}

func NewNotesAnalyzerService(orchestrator *ai.Orchestrator, noteRepo repository.NoteRepository) NotesAnalyzerService {
	return &notesAnalyzerService{
		orchestrator: orchestrator,
		noteRepo:     noteRepo,
	}
}

func (s *notesAnalyzerService) AnalyzeNotes(ctx context.Context, userID string, req dto.NotesAnalysisRequest) (*dto.NotesAnalysisResponse, error) {
	content := req.Content
	title := "Study Notes Analysis"

	// If note_id is provided, retrieve note content directly from repository
	if req.NoteID != "" && s.noteRepo != nil {
		note, err := s.noteRepo.GetByID(ctx, req.NoteID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch note: %w", err)
		}
		if note == nil {
			return nil, errors.New("note not found or unauthorized")
		}
		content = note.Content
		if content == "" {
			content = note.Description
		}
		title = note.Title
	}

	if content == "" {
		return nil, errors.New("content or valid note_id with text is required for analysis")
	}

	systemPrompt := `You are an expert academic summarizer and flashcard generator.
Analyze the provided student study notes and output a single JSON object strictly matching this schema:
{
  "title": "string",
  "summary": "string (high-level executive summary in markdown with LaTeX equations)",
  "key_points": ["string", "string"],
  "flashcards": [
    {
      "front": "question or term",
      "back": "answer or explanation with formula",
      "difficulty": "easy | medium | hard",
      "topic": "sub-topic name"
    }
  ],
  "cheat_sheet": "string (compact formula & definitions cheat sheet in markdown)"
}`
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf("Notes Title: %s\n\nNotes Content:\n%s", title, content)

	var analysis dto.NotesAnalysisResponse
	err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &analysis)
	if err != nil || analysis.Summary == "" {
		// If AI is offline or produced empty summary, generate dynamic summary directly from student's notes content
		summaryContent := content
		if len(summaryContent) > 300 {
			summaryContent = summaryContent[:300] + "..."
		}

		lines := strings.Split(content, "\n")
		var keyPoints []string
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" && len(trimmed) > 10 {
				keyPoints = append(keyPoints, trimmed)
				if len(keyPoints) >= 4 {
					break
				}
			}
		}
		if len(keyPoints) == 0 {
			keyPoints = []string{fmt.Sprintf("Key review points from %s", title)}
		}

		analysis.Title = title
		analysis.Summary = fmt.Sprintf("### %s Summary\n\n%s", title, summaryContent)
		analysis.KeyPoints = keyPoints
		analysis.Flashcards = []dto.FlashcardItem{
			{
				Front:      fmt.Sprintf("What is the central concept discussed in %s?", title),
				Back:       summaryContent,
				Difficulty: "medium",
				Topic:      title,
			},
		}
		analysis.CheatSheet = fmt.Sprintf("### ⚡ %s Quick Review Sheet\n\n%s", title, summaryContent)
		return &analysis, nil
	}

	if analysis.Title == "" {
		analysis.Title = title
	}

	return &analysis, nil
}
