package dto

// ---------------------------------------------------------------------------
// Multi-Modal AI Tutor DTOs
// ---------------------------------------------------------------------------

type DoubtRequest struct {
	Question      string `json:"question" validate:"required,min=2"`
	ScopeType     string `json:"scope_type" validate:"omitempty,oneof=global course folder note"`
	ScopeID       string `json:"scope_id"`
	ImageBase64   string `json:"image_base64"`
	ImageMimeType string `json:"image_mime_type"`
	ContextNotes  string `json:"context_notes"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
}

type DoubtResponse struct {
	Question         string   `json:"question"`
	SolutionMarkdown string   `json:"solution_markdown"`
	CoreConcept      string   `json:"core_concept"`
	CommonPitfalls   string   `json:"common_pitfalls"`
	FollowUpChips    []string `json:"follow_up_chips"`
	ActiveProvider   string   `json:"active_provider"`
	ActiveModel      string   `json:"active_model"`
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
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Shape    string  `json:"shape"`
	ColorHex string  `json:"color_hex"`
}

type GraphEdge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Arrow bool   `json:"arrow"`
}

type DiagramResponse struct {
	Concept     string        `json:"concept"`
	DiagramType string        `json:"diagram_type"`
	Root        *MindMapNode  `json:"root,omitempty"`
	Nodes       []GraphNode   `json:"nodes,omitempty"`
	Edges       []GraphEdge   `json:"edges,omitempty"`
}
