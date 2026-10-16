package dto

import (
	"encoding/json"
	"time"
)

// ---------------------------------------------------------------------------
// Multi-Modal AI Tutor DTOs
// ---------------------------------------------------------------------------

type DoubtRequest struct {
	Question             string             `json:"question" validate:"required,min=2"`
	ScopeType            string             `json:"scope_type" validate:"omitempty,oneof=global course folder note chat session"`
	ScopeID              string             `json:"scope_id"`
	SessionID            string             `json:"session_id,omitempty"`
	ConversationHistory  []ConversationTurn `json:"conversation_history,omitempty"`
	RegenerateLastAnswer bool               `json:"regenerate_last_answer,omitempty"`
	EditLastQuestion     bool               `json:"edit_last_question,omitempty"`
	ImageBase64          string             `json:"image_base64"`
	ImageMimeType        string             `json:"image_mime_type"`
	AudioBase64          string             `json:"audio_base64"`
	AudioMimeType        string             `json:"audio_mime_type"`
	ContextNotes         string             `json:"context_notes"`
	Provider             string             `json:"provider"`
	Model                string             `json:"model"`
	Mode                 string             `json:"mode"`
	TargetExam           string             `json:"target_exam"`
	AcademicLevel        string             `json:"academic_level"`
	WeakTopics           []string           `json:"weak_topics"`
	APIKey               string             `json:"api_key,omitempty"`
}

type ConversationTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DoubtResponse struct {
	Question           string   `json:"question"`
	SolutionMarkdown   string   `json:"solution_markdown"`
	CoreConcept        string   `json:"core_concept"`
	CommonPitfalls     string   `json:"common_pitfalls"`
	FollowUpChips      []string `json:"follow_up_chips"`
	ActiveProvider     string   `json:"active_provider"`
	ActiveModel        string   `json:"active_model"`
	SessionID          string   `json:"session_id,omitempty"`
	SessionTitle       string   `json:"session_title,omitempty"`
	AssistantMessageID string   `json:"assistant_message_id,omitempty"`
}

type CreateSessionRequest struct {
	Title string `json:"title" validate:"required,min=1,max=255"`
}

type UpdateSessionRequest struct {
	Title string `json:"title" validate:"required,min=1,max=255"`
}

// ---------------------------------------------------------------------------
// Notes Analyzer DTOs
// ---------------------------------------------------------------------------

type NotesAnalysisRequest struct {
	NoteID            string `json:"note_id" validate:"omitempty,uuid"`
	Content           string `json:"content"`
	IncludeFlashcards bool   `json:"include_flashcards"`
	IncludeSummary    bool   `json:"include_summary"`
	IncludeCheatSheet bool   `json:"include_cheat_sheet"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
}

type FlashcardItem struct {
	Front      string `json:"front"`
	Back       string `json:"back"`
	Difficulty string `json:"difficulty"`
	Topic      string `json:"topic"`
}

type NotesAnalysisResponse struct {
	Title      string          `json:"title"`
	Summary    string          `json:"summary"`
	KeyPoints  []string        `json:"key_points"`
	Flashcards []FlashcardItem `json:"flashcards"`
	CheatSheet string          `json:"cheat_sheet"`
}

type FlashcardReviewRequest struct {
	CardID      string  `json:"card_id" validate:"required"`
	Quality     int     `json:"quality" validate:"min=0,max=5"`
	Repetitions int     `json:"repetitions"`
	Interval    int     `json:"interval"`
	EaseFactor  float64 `json:"ease_factor"`
}

type FlashcardReviewResponse struct {
	CardID      string  `json:"card_id"`
	NextReview  string  `json:"next_review"`
	Interval    int     `json:"interval_days"`
	EaseFactor  float64 `json:"ease_factor"`
	Repetitions int     `json:"repetitions"`
}

type GenerateFlashcardDeckRequest struct {
	Topic      string `json:"topic" validate:"required,min=2"`
	Content    string `json:"content,omitempty"`
	CardCount  int    `json:"card_count" validate:"omitempty,min=3,max=20"`
	Difficulty string `json:"difficulty" validate:"omitempty,oneof=easy medium hard"`
	SaveDeck   bool   `json:"save_deck"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

type FlashcardDeckCard struct {
	ID          string  `json:"id"`
	Front       string  `json:"front"`
	Back        string  `json:"back"`
	Difficulty  string  `json:"difficulty"`
	Topic       string  `json:"topic"`
	IsMastered  bool    `json:"is_mastered"`
	IsReviewed  bool    `json:"is_reviewed"`
	Repetitions int     `json:"repetitions,omitempty"`
	Interval    int     `json:"interval_days,omitempty"`
	EaseFactor  float64 `json:"ease_factor,omitempty"`
}

type GenerateFlashcardDeckResponse struct {
	DeckID        string              `json:"deck_id,omitempty"`
	Title         string              `json:"title"`
	Topic         string              `json:"topic"`
	Difficulty    string              `json:"difficulty"`
	TotalCards    int                 `json:"total_cards"`
	MasteredCount int                 `json:"mastered_count"`
	Cards         []FlashcardDeckCard `json:"cards"`
}

type SaveFlashcardDeckRequest struct {
	Title         string              `json:"title" validate:"required,min=2"`
	Topic         string              `json:"topic" validate:"required,min=2"`
	Difficulty    string              `json:"difficulty" validate:"omitempty,oneof=easy medium hard"`
	Cards         []FlashcardDeckCard `json:"cards" validate:"required,min=1"`
	MasteredCount int                 `json:"mastered_count"`
}

type UpdateFlashcardDeckRequest struct {
	Title         *string             `json:"title,omitempty"`
	Cards         []FlashcardDeckCard `json:"cards,omitempty"`
	MasteredCount *int                `json:"mastered_count,omitempty"`
	IsPinned      *bool               `json:"is_pinned,omitempty"`
}

type FlashcardDeckSummary struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Topic         string    `json:"topic"`
	Difficulty    string    `json:"difficulty"`
	CardCount     int       `json:"card_count"`
	MasteredCount int       `json:"mastered_count"`
	IsPinned      bool      `json:"is_pinned"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type StudentMistakesResponse struct {
	TargetExam    string   `json:"target_exam"`
	AcademicLevel string   `json:"academic_level"`
	WeakTopics    []string `json:"weak_topics"`
	ExamTraps     []string `json:"exam_traps"`
	QuizAvgScore  float64  `json:"quiz_average_score"`
	TotalQuizzes  int      `json:"total_quizzes_taken"`
}

// ---------------------------------------------------------------------------
// Diagram & Mind Map DTOs (Flutter CustomPainter / CanvasKit compatible)
// ---------------------------------------------------------------------------

type DiagramRequest struct {
	Concept     string `json:"concept" validate:"required,min=2"`
	DiagramType string `json:"diagram_type" validate:"omitempty,oneof=mindmap flowchart concept_tree"`
	Archetype   string `json:"archetype,omitempty" validate:"omitempty,oneof=cycle flow tree comparison hub stack auto"`
	Depth       int    `json:"depth" validate:"omitempty,min=1,max=5"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
}

type MindMapNode struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Subtitle     string            `json:"subtitle,omitempty"`
	Details      string            `json:"details,omitempty"`
	Description  string            `json:"description,omitempty"`
	Shape        string            `json:"shape,omitempty"` // "circle" | "box" | "diamond" | "capsule"
	Type         string            `json:"type,omitempty"`  // "start" | "concept" | "process" | "equation" | "trap"
	ColorHex     string            `json:"color_hex"`
	BranchID     string            `json:"branch_id,omitempty"`
	DomainType   string            `json:"domain_type,omitempty"`
	KeyTakeaway  string            `json:"key_takeaway,omitempty"`
	ExamTrap     string            `json:"exam_trap,omitempty"`
	VivaInsight  string            `json:"viva_insight,omitempty"`
	Importance   int               `json:"importance,omitempty"`
	Badge        string            `json:"badge,omitempty"`
	Metrics      map[string]string `json:"metrics,omitempty"`
	CodeSnippet  string            `json:"code_snippet,omitempty"`
	CrossLinks   []string          `json:"cross_links,omitempty"`
	CrossSubject string            `json:"cross_subject,omitempty"`
	Formulas     []string          `json:"formulas,omitempty"`
	FormulaLatex string            `json:"formula_latex,omitempty"`
	ChildrenIDs  []string          `json:"children_ids,omitempty"`
	Children     []*MindMapNode    `json:"children"`
}

type GraphNode struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Subtitle     string            `json:"subtitle,omitempty"`
	Description  string            `json:"description,omitempty"`
	Details      string            `json:"details,omitempty"`
	NodeType     string            `json:"node_type,omitempty"` // "root" | "branch" | "leaf"
	Type         string            `json:"type,omitempty"`      // "start" | "concept" | "process" | "equation" | "trap"
	Shape        string            `json:"shape,omitempty"`     // "circle" | "box" | "diamond" | "capsule"
	BranchID     string            `json:"branch_id,omitempty"`
	DomainType   string            `json:"domain_type,omitempty"`
	KeyTakeaway  string            `json:"key_takeaway,omitempty"`
	ExamTrap     string            `json:"exam_trap,omitempty"`
	VivaInsight  string            `json:"viva_insight,omitempty"`
	Importance   int               `json:"importance,omitempty"`
	Badge        string            `json:"badge,omitempty"`
	Metrics      map[string]string `json:"metrics,omitempty"`
	CodeSnippet  string            `json:"code_snippet,omitempty"`
	CrossLinks   []string          `json:"cross_links,omitempty"`
	CrossSubject string            `json:"cross_subject,omitempty"`
	X            float64           `json:"x,omitempty"`
	Y            float64           `json:"y,omitempty"`
	Width        float64           `json:"width,omitempty"`
	Height       float64           `json:"height,omitempty"`
	ColorHex     string            `json:"color_hex"`
	Formulas     []string          `json:"formulas,omitempty"`
	FormulaLatex string            `json:"formula_latex,omitempty"`
	ChildrenIDs  []string          `json:"children_ids,omitempty"`
}

type GraphEdge struct {
	From         string `json:"from"`
	To           string `json:"to"`
	FromID       string `json:"from_id,omitempty"`
	ToID         string `json:"to_id,omitempty"`
	Label        string `json:"label"`
	Arrow        bool   `json:"arrow"`
	IsCrossLink  bool   `json:"is_cross_link,omitempty"`
	CrossSubject string `json:"cross_subject,omitempty"`
	Style        string `json:"style,omitempty"` // "solid" | "dashed"
}

type DiagramResponse struct {
	Concept     string       `json:"concept"`
	Title       string       `json:"title,omitempty"`
	DiagramType string       `json:"diagram_type"`
	DomainType  string       `json:"domain_type,omitempty"`
	Archetype   string       `json:"archetype,omitempty"` // "cycle" | "flow" | "tree" | "comparison" | "hub" | "stack"
	Summary     string       `json:"summary,omitempty"`
	Root        *MindMapNode `json:"root,omitempty"`
	Nodes       []GraphNode  `json:"nodes,omitempty"`
	Edges       []GraphEdge  `json:"edges,omitempty"`
}

type MindMapToQuizRequest struct {
	Title        string      `json:"title,omitempty"`
	Concept      string      `json:"concept" validate:"required"`
	Archetype    string      `json:"archetype,omitempty"`
	Nodes        []GraphNode `json:"nodes" validate:"required,min=1"`
	NumQuestions int         `json:"num_questions" validate:"omitempty,min=1,max=10"`
	Difficulty   string      `json:"difficulty" validate:"omitempty,oneof=easy medium hard"`
}

type MindMapToFlashcardRequest struct {
	Title      string      `json:"title,omitempty"`
	Concept    string      `json:"concept" validate:"required"`
	Archetype  string      `json:"archetype,omitempty"`
	Nodes      []GraphNode `json:"nodes" validate:"required,min=1"`
	CardCount  int         `json:"card_count" validate:"omitempty,min=3,max=20"`
	Difficulty string      `json:"difficulty" validate:"omitempty,oneof=easy medium hard"`
	SaveDeck   bool        `json:"save_deck"`
}

type ExportMindMapPDFRequest struct {
	Title     string      `json:"title"`
	Concept   string      `json:"concept"`
	Archetype string      `json:"archetype,omitempty"`
	Summary   string      `json:"summary,omitempty"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	PaperSize string      `json:"paper_size,omitempty"` // "A4" | "A3"
}

type ExpandNodeRequest struct {
	Concept     string `json:"concept" validate:"required"`
	NodeID      string `json:"node_id" validate:"required"`
	NodeLabel   string `json:"node_label" validate:"required"`
	NodeDetails string `json:"node_details,omitempty"`
	BranchColor string `json:"branch_color,omitempty"`
	MindMapID   string `json:"mind_map_id,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
}

type ExpandNodeResponse struct {
	NodeID   string      `json:"node_id"`
	NewNodes []GraphNode `json:"new_nodes"`
	NewEdges []GraphEdge `json:"new_edges"`
}

type SaveMindMapRequest struct {
	Title       string          `json:"title" validate:"required,min=2,max=255"`
	Concept     string          `json:"concept" validate:"required,min=2,max=255"`
	DiagramType string          `json:"diagram_type" validate:"omitempty,oneof=mindmap flowchart concept_tree"`
	DomainType  string          `json:"domain_type" validate:"omitempty"`
	NoteID      *string         `json:"note_id,omitempty"`
	Data        json.RawMessage `json:"data" validate:"required"`
	IsPinned    bool            `json:"is_pinned,omitempty"`
}

type UpdateMindMapRequest struct {
	Title    *string          `json:"title,omitempty" validate:"omitempty,min=2,max=255"`
	Data     *json.RawMessage `json:"data,omitempty"`
	IsPinned *bool            `json:"is_pinned,omitempty"`
}

type SavedMindMapSummary struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Concept     string    `json:"concept"`
	DiagramType string    `json:"diagram_type"`
	DomainType  string    `json:"domain_type"`
	NoteID      *string   `json:"note_id,omitempty"`
	IsPinned    bool      `json:"is_pinned"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SavedMindMapResponse struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Concept     string          `json:"concept"`
	DiagramType string          `json:"diagram_type"`
	DomainType  string          `json:"domain_type"`
	NoteID      *string         `json:"note_id,omitempty"`
	Data        json.RawMessage `json:"data"`
	IsPinned    bool            `json:"is_pinned"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
