// Package services provides the implementation of the SyncService interface, which is responsible for handling synchronization operations between the client and server. It interacts with the SyncRepository to fetch the delta of changes since a specified timestamp for a given user. The service returns a structured response containing the updated courses, folders, notes, and quizzes along with the timestamp of synchronization.
package services

import (
	"context"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/repository"
)

type SyncService interface {
	GetDelta(ctx context.Context, userID string, since time.Time) (*dto.SyncResponse, error)
}

type syncService struct {
	syncRepo repository.SyncRepository
}

func NewSyncService(syncRepo repository.SyncRepository) SyncService {
	return &syncService{syncRepo: syncRepo}
}

func (s *syncService) GetDelta(ctx context.Context, userID string, since time.Time) (*dto.SyncResponse, error) {
	courses, folders, notes, quizzes, err := s.syncRepo.GetDelta(ctx, userID, since)
	if err != nil {
		return nil, err
	}

	return &dto.SyncResponse{
		SyncedAt: time.Now(),
		Courses:  courses,
		Folders:  folders,
		Notes:    notes,
		Quizzes:  quizzes,
	}, nil
}
