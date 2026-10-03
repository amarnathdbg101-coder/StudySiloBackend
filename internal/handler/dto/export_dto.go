package dto

type ExportPDFRequest struct {
	NoteID  string `json:"note_id" validate:"omitempty,uuid"`
	QuizID  string `json:"quiz_id" validate:"omitempty,uuid"`
	Title   string `json:"title" validate:"required,min=1,max=200"`
	Content string `json:"content"`
}
