package dto

import (
	"time"

	"goProjectStructure/internal/handler/model"
)

// SyncResponse holds delta entities for offline-first Flutter synchronization.
type SyncResponse struct {
	SyncedAt time.Time       `json:"synced_at"`
	Courses  []*model.Course `json:"courses"`
	Folders  []*model.Folder `json:"folders"`
	Notes    []*model.Note   `json:"notes"`
	Quizzes  []*model.Quiz   `json:"quizzes"`
}
