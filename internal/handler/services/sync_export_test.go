package services

import (
	"bytes"
	"context"
	"testing"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
)

type mockSyncRepo struct{}

func (m *mockSyncRepo) GetDelta(ctx context.Context, userID string, since time.Time) ([]*model.Course, []*model.Folder, []*model.Note, []*model.Quiz, error) {
	courses := []*model.Course{
		{ID: "c1", UserID: userID, Title: "Math 101"},
	}
	folders := []*model.Folder{
		{ID: "f1", UserID: userID, CourseID: "c1", Name: "Calculus"},
	}
	notes := []*model.Note{
		{ID: "n1", UserID: userID, CourseID: &courses[0].ID, Title: "Derivatives", Content: "d/dx (x^n) = n*x^(n-1)"},
	}
	quizzes := []*model.Quiz{
		{ID: "q1", UserID: userID, Title: "Calculus Quiz", Topic: "Derivatives"},
	}
	return courses, folders, notes, quizzes, nil
}

func TestSyncService_GetDelta(t *testing.T) {
	syncRepo := &mockSyncRepo{}
	service := NewSyncService(syncRepo)

	ctx := context.Background()
	res, err := service.GetDelta(ctx, "user-1", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("failed to get delta: %v", err)
	}

	if len(res.Courses) != 1 || len(res.Folders) != 1 || len(res.Notes) != 1 || len(res.Quizzes) != 1 {
		t.Errorf("unexpected delta counts: %+v", res)
	}

	if res.SyncedAt.IsZero() {
		t.Error("expected non-zero SyncedAt timestamp")
	}
}

func TestPDFExportService_ExportStudyPack(t *testing.T) {
	noteRepo := &mockNoteRepo{}
	quizRepo := &mockQuizRepo{quizzes: make(map[string]*model.Quiz)}
	service := NewPDFExportService(noteRepo, quizRepo)

	ctx := context.Background()
	req := dto.ExportPDFRequest{
		Title:   "Physics Kinematics Cheat Sheet",
		Content: "# Motion in One Dimension\n\n- Position: $x(t)$\n- Velocity: $v = dx/dt$\n- Acceleration: $a = dv/dt$\n\n### Core Equations\n$v = u + at$\n$s = ut + 0.5at^2$",
	}

	pdfBytes, err := service.ExportStudyPack(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("failed to generate PDF: %v", err)
	}

	if len(pdfBytes) < 100 {
		t.Fatalf("generated PDF too small: %d bytes", len(pdfBytes))
	}

	// Verify standard PDF magic header bytes "%PDF-"
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Errorf("expected PDF header magic bytes, got: %s", string(pdfBytes[:8]))
	}
}
