package dto

// ---------------------------------------------------------------------------
// Multi-Modal AI Tutor DTOs
// ---------------------------------------------------------------------------

type DoubtRequest struct {
	Question      string   `json:"question" validate:"required,min=2"`
	ScopeType     string   `json:"scope_type" validate:"omitempty,oneof=global course folder note chat session"`
	ScopeID       string   `json:"scope_id"`
	SessionID     string   `json:"session_id,omitempty"`
	ImageBase64   string   `json:"image_base64"`
	ImageMimeType string   `json:"image_mime_type"`
	AudioBase64   string   `json:"audio_base64"`
	AudioMimeType string   `json:"audio_mime_type"`
	ContextNotes  string   `json:"context_notes"`
	Provider      string   `json:"provider"`
	Model         string   `json:"model"`
	Mode          string   `json:"mode"`
	TargetExam    string   `json:"target_exam"`
	AcademicLevel string   `json:"academic_level"`
	WeakTopics    []string `json:"weak_topics"`
	APIKey        string   `json:"api_key,omitempty"`
}

type DoubtResponse struct {
	Question         string   `json:"question"`
	SolutionMarkdown string   `json:"solution_markdown"`
	CoreConcept      string   `json:"core_concept"`
	CommonPitfalls   string   `json:"common_pitfalls"`
	FollowUpChips    []string `json:"follow_up_chips"`
	ActiveProvider   string   `json:"active_provider"`
	ActiveModel      string   `json:"active_model"`
	SessionID        string   `json:"session_id,omitempty"`
	SessionTitle     string   `json:"session_title,omitempty"`
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

// ---------------------------------------------------------------------------
// Diagram & Mind Map DTOs (Flutter CustomPainter / CanvasKit compatible)
// ---------------------------------------------------------------------------

type DiagramRequest struct {
	Concept     string `json:"concept" validate:"required,min=2"`
	DiagramType string `json:"diagram_type" validate:"omitempty,oneof=mindmap flowchart concept_tree"`
	Depth       int    `json:"depth" validate:"omitempty,min=1,max=5"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
}

type MindMapNode struct {
	ID        string         `json:"id"`
	Label     string         `json:"label"`
	Details   string         `json:"details"`
	ColorHex  string         `json:"color_hex"`
	Children  []*MindMapNode `json:"children"`
}

type GraphNode struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	NodeType    string   `json:"node_type,omitempty"`
	X           float64  `json:"x,omitempty"`
	Y           float64  `json:"y,omitempty"`
	Width       float64  `json:"width,omitempty"`
	Height      float64  `json:"height,omitempty"`
	Shape       string   `json:"shape,omitempty"`
	ColorHex    string   `json:"color_hex"`
	Formulas    []string `json:"formulas,omitempty"`
}

type GraphEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	FromID string `json:"from_id,omitempty"`
	ToID   string `json:"to_id,omitempty"`
	Label  string `json:"label"`
	Arrow  bool   `json:"arrow"`
}

type DiagramResponse struct {
	Concept     string        `json:"concept"`
	Title       string        `json:"title,omitempty"`
	DiagramType string        `json:"diagram_type"`
	Root        *MindMapNode  `json:"root,omitempty"`
	Nodes       []GraphNode   `json:"nodes,omitempty"`
	Edges       []GraphEdge   `json:"edges,omitempty"`
}
