package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/middleware"
	"goProjectStructure/internal/reuse/ai"
)

type StudyPlanService interface {
	CreateStudyPlan(ctx context.Context, userID string, req dto.CreateStudyPlanRequest) (*model.StudyPlan, error)
	ListStudyPlans(ctx context.Context, userID string) ([]*model.StudyPlan, error)
	UpdateStudyPlan(ctx context.Context, id, userID string, req dto.UpdateStudyPlanRequest) (*model.StudyPlan, error)
	DeleteStudyPlan(ctx context.Context, id, userID string) error
	RecordStudyTime(ctx context.Context, userID string, req dto.RecordStudyTimeRequest) (*model.CognitiveProfile, error)

	// AI Roadmap CRUD - AI se generate aur DB mein save/manage karne ke liye
	GenerateAIStudyPlan(ctx context.Context, userID string, req dto.GenerateAIStudyPlanRequest) (*dto.GenerateAIStudyPlanResponse, error)
	SaveRoadmap(ctx context.Context, userID string, req dto.SaveRoadmapRequest) (*model.AIRoadmap, error)
	GetRoadmap(ctx context.Context, id, userID string) (*model.AIRoadmap, error)
	ListRoadmaps(ctx context.Context, userID string) ([]*model.AIRoadmap, error)
	UpdateRoadmap(ctx context.Context, id, userID string, req dto.UpdateRoadmapRequest) (*model.AIRoadmap, error)
	DeleteRoadmap(ctx context.Context, id, userID string) error
}

type studyPlanService struct {
	repo         repository.StudyPlanRepository
	roadmapRepo  repository.AIRoadmapRepository // AI roadmaps ke liye alag repo
	orchestrator *ai.Orchestrator
}

func NewStudyPlanService(repo repository.StudyPlanRepository, roadmapRepo repository.AIRoadmapRepository, orchestrator *ai.Orchestrator) StudyPlanService {
	return &studyPlanService{
		repo:         repo,
		roadmapRepo:  roadmapRepo,
		orchestrator: orchestrator,
	}
}

func (s *studyPlanService) CreateStudyPlan(ctx context.Context, userID string, req dto.CreateStudyPlanRequest) (*model.StudyPlan, error) {
	priority := strings.ToLower(req.Priority)
	if priority == "" {
		priority = "medium"
	}

	var dueDate *time.Time
	if req.DueDate != nil && *req.DueDate != "" {
		if t, err := time.Parse(time.RFC3339, *req.DueDate); err == nil {
			dueDate = &t
		}
	}

	plan := &model.StudyPlan{
		UserID:      userID,
		CourseID:    req.CourseID,
		Title:       req.Title,
		DueDate:     dueDate,
		IsCompleted: false,
		Priority:    priority,
	}

	if err := s.repo.Create(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to create study plan: %w", err)
	}
	return plan, nil
}

func (s *studyPlanService) ListStudyPlans(ctx context.Context, userID string) ([]*model.StudyPlan, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *studyPlanService) UpdateStudyPlan(ctx context.Context, id, userID string, req dto.UpdateStudyPlanRequest) (*model.StudyPlan, error) {
	plan, err := s.repo.GetByID(ctx, id, userID)
	if err != nil || plan == nil {
		return nil, fmt.Errorf("study plan not found")
	}

	if req.Title != nil {
		plan.Title = *req.Title
	}
	if req.IsCompleted != nil {
		plan.IsCompleted = *req.IsCompleted
	}
	if req.Priority != nil {
		plan.Priority = strings.ToLower(*req.Priority)
	}

	if err := s.repo.Update(ctx, plan); err != nil {
		return nil, fmt.Errorf("failed to update study plan: %w", err)
	}
	return plan, nil
}

func (s *studyPlanService) DeleteStudyPlan(ctx context.Context, id, userID string) error {
	return s.repo.Delete(ctx, id, userID)
}

func (s *studyPlanService) RecordStudyTime(ctx context.Context, userID string, req dto.RecordStudyTimeRequest) (*model.CognitiveProfile, error) {
	return s.repo.RecordStudyMinutes(ctx, userID, req.Minutes)
}

// GenerateAIStudyPlan - AI se spaced repetition roadmap generate karo
// Agar req.SaveToDB == true hai toh result automatically DB mein save ho jaata hai
func (s *studyPlanService) GenerateAIStudyPlan(ctx context.Context, userID string, req dto.GenerateAIStudyPlanRequest) (*dto.GenerateAIStudyPlanResponse, error) {
	if s.orchestrator == nil {
		return nil, fmt.Errorf("AI orchestrator not initialized")
	}

	systemPrompt := `You are an expert academic curriculum designer and active recall study planner.
Generate a structured study roadmap strictly in JSON format matching this schema:
{
  "exam_name": "string",
  "overview": "string (high-level strategy & spaced repetition advice)",
  "schedule": [
    {
      "day_number": 1,
      "title": "Module/Topic Title",
      "focus_area": "Core Concept / Derivation / Problem Solving",
      "priority": "high | medium | low",
      "subtasks": ["Action task 1", "Action task 2"]
    }
  ]
}`
	aiLanguage := middleware.GetAILanguage(ctx)
	systemPrompt += ai.BuildLanguageInstruction(aiLanguage)

	userPrompt := fmt.Sprintf("Exam Name: %s\nTarget Study Days: %d\nSyllabus Topics:\n- %s",
		req.ExamName, req.TargetDays, strings.Join(req.Syllabus, "\n- "))

	var response dto.GenerateAIStudyPlanResponse
	if err := s.orchestrator.GenerateStructured(ctx, systemPrompt, userPrompt, &response); err != nil {
		// Fallback schedule generator if AI unmarshal fails
		response = dto.GenerateAIStudyPlanResponse{
			ExamName: req.ExamName,
			Overview: fmt.Sprintf("Comprehensive %d-day active recall study plan for %s.", req.TargetDays, req.ExamName),
			Schedule: []dto.GeneratedStudyTask{
				{
					DayNumber: 1,
					Title:     "Foundational Concepts & Core Principles",
					FocusArea: "Theory & Definitions",
					Priority:  "high",
					Subtasks:  []string{"Read core definitions", "Create 5 active recall flashcards"},
				},
				{
					DayNumber: 2,
					Title:     "Formula Derivations & Numerical Practice",
					FocusArea: "Problem Solving",
					Priority:  "high",
					Subtasks:  []string{"Solve 10 practice numericals", "Review common exam traps"},
				},
			},
		}
	}

	// Agar save_to_db flag true hai toh automatically DB mein persist karo
	if req.SaveToDB && userID != "guest-user" && userID != "" {
		savedRoadmap, saveErr := s.SaveRoadmap(ctx, userID, dto.SaveRoadmapRequest{
			ExamName:   response.ExamName,
			Overview:   response.Overview,
			Syllabus:   req.Syllabus,
			TargetDays: req.TargetDays,
			Schedule:   response.Schedule,
		})
		if saveErr == nil {
			response.RoadmapID = &savedRoadmap.ID
		}
	}

	return &response, nil
}

// SaveRoadmap - already generated roadmap ko explicitly DB mein save karo (Create)
func (s *studyPlanService) SaveRoadmap(ctx context.Context, userID string, req dto.SaveRoadmapRequest) (*model.AIRoadmap, error) {
	// DTO ke GeneratedStudyTask ko model.RoadmapTask mein convert karo
	tasks := make([]model.RoadmapTask, len(req.Schedule))
	for i, t := range req.Schedule {
		tasks[i] = model.RoadmapTask{
			DayNumber: t.DayNumber,
			Title:     t.Title,
			FocusArea: t.FocusArea,
			Priority:  t.Priority,
			Subtasks:  t.Subtasks,
		}
	}

	roadmap := &model.AIRoadmap{
		UserID:         userID,
		ExamName:       req.ExamName,
		Overview:       req.Overview,
		SyllabusTopics: req.Syllabus,
		TargetDays:     req.TargetDays,
		Schedule:       tasks,
		IsActive:       true,
	}

	if err := s.roadmapRepo.Create(ctx, roadmap); err != nil {
		return nil, fmt.Errorf("failed to save roadmap: %w", err)
	}
	return roadmap, nil
}

// GetRoadmap - ek roadmap fetch karo by ID
func (s *studyPlanService) GetRoadmap(ctx context.Context, id, userID string) (*model.AIRoadmap, error) {
	roadmap, err := s.roadmapRepo.GetByID(ctx, id, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get roadmap: %w", err)
	}
	if roadmap == nil {
		return nil, fmt.Errorf("roadmap not found")
	}
	return roadmap, nil
}

// ListRoadmaps - user ke saare saved roadmaps list karo
func (s *studyPlanService) ListRoadmaps(ctx context.Context, userID string) ([]*model.AIRoadmap, error) {
	roadmaps, err := s.roadmapRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list roadmaps: %w", err)
	}
	if roadmaps == nil {
		return []*model.AIRoadmap{}, nil
	}
	return roadmaps, nil
}

// UpdateRoadmap - existing roadmap ka content update karo (PATCH)
func (s *studyPlanService) UpdateRoadmap(ctx context.Context, id, userID string, req dto.UpdateRoadmapRequest) (*model.AIRoadmap, error) {
	// Pehle existing roadmap fetch karo
	roadmap, err := s.roadmapRepo.GetByID(ctx, id, userID)
	if err != nil || roadmap == nil {
		return nil, fmt.Errorf("roadmap not found or unauthorized")
	}

	// Sirf woh fields update karo jo request mein aaye hain
	if req.ExamName != nil {
		roadmap.ExamName = *req.ExamName
	}
	if req.Overview != nil {
		roadmap.Overview = *req.Overview
	}
	if req.IsActive != nil {
		roadmap.IsActive = *req.IsActive
	}
	if len(req.Schedule) > 0 {
		tasks := make([]model.RoadmapTask, len(req.Schedule))
		for i, t := range req.Schedule {
			tasks[i] = model.RoadmapTask{
				DayNumber: t.DayNumber,
				Title:     t.Title,
				FocusArea: t.FocusArea,
				Priority:  t.Priority,
				Subtasks:  t.Subtasks,
			}
		}
		roadmap.Schedule = tasks
	}

	if err := s.roadmapRepo.Update(ctx, roadmap); err != nil {
		return nil, fmt.Errorf("failed to update roadmap: %w", err)
	}
	return roadmap, nil
}

// DeleteRoadmap - roadmap permanently delete karo
func (s *studyPlanService) DeleteRoadmap(ctx context.Context, id, userID string) error {
	if err := s.roadmapRepo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("failed to delete roadmap: %w", err)
	}
	return nil
}

