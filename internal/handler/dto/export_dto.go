package dto

type ChatTurnExportDTO struct {
	Role      string `json:"role"` // "user" | "assistant"
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

type ExportPDFRequest struct {
	NoteID           string              `json:"note_id" validate:"omitempty,uuid"`
	QuizID           string              `json:"quiz_id" validate:"omitempty,uuid"`
	Title            string              `json:"title" validate:"required,min=1,max=200"`
	Subject          string              `json:"subject,omitempty"`
	TargetExam       string              `json:"target_exam,omitempty"`
	StudentName      string              `json:"student_name,omitempty"`
	DocumentType     string              `json:"document_type,omitempty"` // "notes" | "chat" | "quiz" | "formula"
	Content          string              `json:"content"`
	ChatTurns        []ChatTurnExportDTO `json:"chat_turns,omitempty"`
	IncludeSolutions bool                `json:"include_solutions,omitempty"`
}
