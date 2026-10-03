package services

import (
	"context"
	"testing"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
)

// Mock repositories for testing
type mockCourseRepo struct {
	courses map[string]*model.Course
}

func (m *mockCourseRepo) Create(ctx context.Context, c *model.Course) error {
	c.ID = "c-123"
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	m.courses[c.ID] = c
	return nil
}

func (m *mockCourseRepo) GetByID(ctx context.Context, id, userID string) (*model.Course, error) {
	c, ok := m.courses[id]
	if !ok || c.UserID != userID {
		return nil, nil
	}
	return c, nil
}

func (m *mockCourseRepo) ListByUser(ctx context.Context, userID string) ([]*model.Course, error) {
	list := []*model.Course{}
	for _, c := range m.courses {
		if c.UserID == userID {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *mockCourseRepo) Update(ctx context.Context, c *model.Course) error {
	m.courses[c.ID] = c
	return nil
}

func (m *mockCourseRepo) Delete(ctx context.Context, id, userID string) error {
	delete(m.courses, id)
	return nil
}

type mockFolderRepo struct {
	folders map[string]*model.Folder
}

func (m *mockFolderRepo) Create(ctx context.Context, f *model.Folder) error {
	f.ID = "f-123"
	m.folders[f.ID] = f
	return nil
}

func (m *mockFolderRepo) GetByID(ctx context.Context, id, userID string) (*model.Folder, error) {
	return m.folders[id], nil
}

func (m *mockFolderRepo) ListByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error) {
	return []*model.Folder{}, nil
}

func (m *mockFolderRepo) GetTreeByCourse(ctx context.Context, courseID, userID string) ([]*model.Folder, error) {
	return []*model.Folder{}, nil
}

func (m *mockFolderRepo) Update(ctx context.Context, f *model.Folder) error {
	return nil
}

func (m *mockFolderRepo) Delete(ctx context.Context, id, userID string) error {
	return nil
}

type mockNoteRepo struct{}

func (m *mockNoteRepo) Create(ctx context.Context, n *model.Note) error { return nil }
func (m *mockNoteRepo) GetByID(ctx context.Context, id, userID string) (*model.Note, error) {
	return nil, nil
}
func (m *mockNoteRepo) List(ctx context.Context, userID string, c, f, t *string) ([]*model.Note, error) {
	return []*model.Note{}, nil
}
func (m *mockNoteRepo) Update(ctx context.Context, n *model.Note) error { return nil }
func (m *mockNoteRepo) Delete(ctx context.Context, id, userID string) error { return nil }

func TestAcademicService_CourseFlow(t *testing.T) {
	courseRepo := &mockCourseRepo{courses: make(map[string]*model.Course)}
	folderRepo := &mockFolderRepo{folders: make(map[string]*model.Folder)}
	noteRepo := &mockNoteRepo{}

	service := NewAcademicService(courseRepo, folderRepo, noteRepo)
	ctx := context.Background()

	// 1. Create Course
	req := dto.CreateCourseRequest{
		Title:    "Physics 101",
		Code:     "PHY101",
		Semester: "Semester 1",
		ColorHex: "#4F46E5",
	}

	course, err := service.CreateCourse(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("failed to create course: %v", err)
	}

	if course.ID != "c-123" || course.Title != "Physics 101" {
		t.Fatalf("unexpected course result: %+v", course)
	}

	// 2. Fetch Course
	fetched, err := service.GetCourse(ctx, "c-123", "user-1")
	if err != nil || fetched.Title != "Physics 101" {
		t.Fatalf("failed to fetch course: %v", err)
	}

	// 3. Unauthorized access should fail
	_, err = service.GetCourse(ctx, "c-123", "user-other")
	if err == nil {
		t.Fatal("expected error on unauthorized access, got nil")
	}
}
