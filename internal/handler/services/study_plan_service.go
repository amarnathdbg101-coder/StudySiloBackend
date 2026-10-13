package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse/ai"
)

type StudyPlanService interface {
	CreateStudyPlan(ctx context.Context, userID string, req dto.CreateStudyPlanRequest) (*model.StudyPlan, error)
	ListStudyPlans(ctx context.Context, userID string) ([]*model.StudyPlan, error)
	UpdateStudyPlan(ctx context.Context, id, userID string, req dto.UpdateStudyPlanRequest) (*model.StudyPlan, error)
	DeleteStudyPlan(ctx context.Context, id, userID string) error
	RecordStudyTime(ctx context.Context, userID string, req dto.RecordStudyTimeRequest) (*model.CognitiveProfile, error)
	GenerateAIStudyPlan(ctx context.Context, userID string, req dto.GenerateAIStudyPlanRequest) (*dto.GenerateAIStudyPlanResponse, error)
}

type studyPlanService struct {
	repo         repository.StudyPlanRepository
	orchestrator *ai.Orchestrator
}

func NewStudyPlanService(repo repository.StudyPlanRepository, orchestrator *ai.Orchestrator) StudyPlanService {
	return &studyPlanService{repo: repo, orchestrator: orchestrator}
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

	return &response, nil
}
