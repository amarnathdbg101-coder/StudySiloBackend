package services

import (
	"context"
	"errors"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
)

type AcademicService interface {
	// Course methods
	CreateCourse(ctx context.Context, userID string, req dto.CreateCourseRequest) (*model.Course, error)
	GetCourse(ctx context.Context, id, userID string) (*model.Course, error)
	ListCourses(ctx context.Context, userID string) ([]*model.Course, error)
	UpdateCourse(ctx context.Context, id, userID string, req dto.UpdateCourseRequest) (*model.Course, error)
	DeleteCourse(ctx context.Context, id, userID string) error

	// Folder methods
	CreateFolder(ctx context.Context, userID string, req dto.CreateFolderRequest) (*model.Folder, error)
	GetFolder(ctx context.Context, id, userID string) (*model.Folder, error)
	GetFolderTree(ctx context.Context, courseID, userID string) ([]*model.Folder, error)
	UpdateFolder(ctx context.Context, id, userID string, req dto.UpdateFolderRequest) (*model.Folder, error)
	DeleteFolder(ctx context.Context, id, userID string) error

	// Note methods
	CreateNote(ctx context.Context, userID string, req dto.CreateNoteRequest) (*model.Note, error)
	GetNote(ctx context.Context, id, userID string) (*model.Note, error)
	ListNotes(ctx context.Context, userID string, courseID, folderID *string, tag *string) ([]*model.Note, error)
	SearchNotes(ctx context.Context, userID, searchTerm string, limit int) ([]*model.Note, error)
	UpdateNote(ctx context.Context, id, userID string, req dto.UpdateNoteRequest) (*model.Note, error)
	DeleteNote(ctx context.Context, id, userID string) error
}

type academicService struct {
	courseRepo repository.CourseRepository
	folderRepo repository.FolderRepository
	noteRepo   repository.NoteRepository
}

func NewAcademicService(
	courseRepo repository.CourseRepository,
	folderRepo repository.FolderRepository,
	noteRepo repository.NoteRepository,
) AcademicService {
	return &academicService{
		courseRepo: courseRepo,
		folderRepo: folderRepo,
		noteRepo:   noteRepo,
	}
}

// Course implementations
func (s *academicService) CreateCourse(ctx context.Context, userID string, req dto.CreateCourseRequest) (*model.Course, error) {
	color := req.ColorHex
	if color == "" {
		color = "#4F46E5"
	}
	icon := req.IconName
	if icon == "" {
		icon = "book"
	}
	course := &model.Course{
		UserID:      userID,
		Title:       req.Title,
		Code:        req.Code,
		Semester:    req.Semester,
		Description: req.Description,
		ColorHex:    color,
		IconName:    icon,
		IsArchived:  false,
	}
	if err := s.courseRepo.Create(ctx, course); err != nil {
		return nil, err
	}
	return course, nil
}

func (s *academicService) GetCourse(ctx context.Context, id, userID string) (*model.Course, error) {
	c, err := s.courseRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, errors.New("course not found")
	}
	return c, nil
}

func (s *academicService) ListCourses(ctx context.Context, userID string) ([]*model.Course, error) {
	return s.courseRepo.ListByUser(ctx, userID)
}

func (s *academicService) UpdateCourse(ctx context.Context, id, userID string, req dto.UpdateCourseRequest) (*model.Course, error) {
	c, err := s.GetCourse(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		c.Title = *req.Title
	}
	if req.Code != nil {
		c.Code = *req.Code
	}
	if req.Semester != nil {
		c.Semester = *req.Semester
	}
	if req.Description != nil {
		c.Description = *req.Description
	}
	if req.ColorHex != nil {
		c.ColorHex = *req.ColorHex
	}
	if req.IconName != nil {
		c.IconName = *req.IconName
	}
	if req.IsArchived != nil {
		c.IsArchived = *req.IsArchived
	}

	if err := s.courseRepo.Update(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *academicService) DeleteCourse(ctx context.Context, id, userID string) error {
	return s.courseRepo.Delete(ctx, id, userID)
}

// Folder implementations
func (s *academicService) CreateFolder(ctx context.Context, userID string, req dto.CreateFolderRequest) (*model.Folder, error) {
	// Verify course belongs to user
	if _, err := s.GetCourse(ctx, req.CourseID, userID); err != nil {
		return nil, errors.New("associated course not found or unauthorized")
	}

	color := req.ColorHex
	if color == "" {
		color = "#4F46E5"
	}
	icon := req.IconName
	if icon == "" {
		icon = "folder"
	}
	folderType := req.FolderType
	if folderType == "" {
		folderType = "module"
	}

	folder := &model.Folder{
		UserID:      userID,
		CourseID:    req.CourseID,
		ParentID:    req.ParentID,
		Name:        req.Name,
		FolderType:  folderType,
		Description: req.Description,
		ColorHex:    color,
		IconName:    icon,
		Position:    req.Position,
	}

	if err := s.folderRepo.Create(ctx, folder); err != nil {
		return nil, err
	}
	return folder, nil
}

func (s *academicService) GetFolder(ctx context.Context, id, userID string) (*model.Folder, error) {
	f, err := s.folderRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, errors.New("folder not found")
	}
	return f, nil
}

func (s *academicService) GetFolderTree(ctx context.Context, courseID, userID string) ([]*model.Folder, error) {
	return s.folderRepo.GetTreeByCourse(ctx, courseID, userID)
}

func (s *academicService) UpdateFolder(ctx context.Context, id, userID string, req dto.UpdateFolderRequest) (*model.Folder, error) {
	f, err := s.GetFolder(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		f.Name = *req.Name
	}
	if req.ParentID != nil {
		f.ParentID = req.ParentID
	}
	if req.FolderType != nil {
		f.FolderType = *req.FolderType
	}
	if req.Description != nil {
		f.Description = *req.Description
	}
	if req.ColorHex != nil {
		f.ColorHex = *req.ColorHex
	}
	if req.IconName != nil {
		f.IconName = *req.IconName
	}
	if req.Position != nil {
		f.Position = *req.Position
	}

	if err := s.folderRepo.Update(ctx, f); err != nil {
		return nil, err
	}
	return f, nil
}

func (s *academicService) DeleteFolder(ctx context.Context, id, userID string) error {
	return s.folderRepo.Delete(ctx, id, userID)
}

// Note implementations
func (s *academicService) CreateNote(ctx context.Context, userID string, req dto.CreateNoteRequest) (*model.Note, error) {
	note := &model.Note{
		UserID:      userID,
		CourseID:    req.CourseID,
		FolderID:    req.FolderID,
		ModuleName:  req.ModuleName,
		Title:       req.Title,
		Description: req.Description,
		Content:     req.Content,
		FileURL:     req.FileURL,
		FileType:    req.FileType,
		FileSize:    req.FileSize,
		Tags:        req.Tags,
		IsPinned:    req.IsPinned,
		IsArchived:  false,
	}

	if err := s.noteRepo.Create(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *academicService) GetNote(ctx context.Context, id, userID string) (*model.Note, error) {
	n, err := s.noteRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, errors.New("note not found")
	}
	return n, nil
}

func (s *academicService) ListNotes(ctx context.Context, userID string, courseID, folderID *string, tag *string) ([]*model.Note, error) {
	return s.noteRepo.List(ctx, userID, courseID, folderID, tag)
}

func (s *academicService) UpdateNote(ctx context.Context, id, userID string, req dto.UpdateNoteRequest) (*model.Note, error) {
	n, err := s.GetNote(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if req.CourseID != nil {
		n.CourseID = req.CourseID
	}
	if req.FolderID != nil {
		n.FolderID = req.FolderID
	}
	if req.ModuleName != nil {
		n.ModuleName = *req.ModuleName
	}
	if req.Title != nil {
		n.Title = *req.Title
	}
	if req.Description != nil {
		n.Description = *req.Description
	}
	if req.Content != nil {
		n.Content = *req.Content
	}
	if req.FileURL != nil {
		n.FileURL = *req.FileURL
	}
	if req.FileType != nil {
		n.FileType = *req.FileType
	}
	if req.FileSize != nil {
		n.FileSize = *req.FileSize
	}
	if req.Tags != nil {
		n.Tags = *req.Tags
	}
	if req.IsPinned != nil {
		n.IsPinned = *req.IsPinned
	}
	if req.IsArchived != nil {
		n.IsArchived = *req.IsArchived
	}

	if err := s.noteRepo.Update(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

func (s *academicService) SearchNotes(ctx context.Context, userID, searchTerm string, limit int) ([]*model.Note, error) {
	return s.noteRepo.Search(ctx, userID, searchTerm, limit)
}

func (s *academicService) DeleteNote(ctx context.Context, id, userID string) error {
	return s.noteRepo.Delete(ctx, id, userID)
}
