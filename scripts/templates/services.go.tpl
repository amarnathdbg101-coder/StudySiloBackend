package services

import (
	"context"
	"errors"

	"goProjectStructure/internal/handler/dto"
	"goProjectStructure/internal/handler/model"
	"goProjectStructure/internal/handler/repository"
	"goProjectStructure/internal/reuse"
)

type __PASCAL__Service struct {
	repo *repository.__PASCAL__Repository
}

func New__PASCAL__Service(repo *repository.__PASCAL__Repository) *__PASCAL__Service {
	return &__PASCAL__Service{repo: repo}
}

func (s *__PASCAL__Service) Create(ctx context.Context, req dto.Create__PASCAL__Request) (*model.__PASCAL__, error) {
	item := &model.__PASCAL__{
		Title:       req.Title,
		Description: req.Description,
	}
	err := s.repo.Create(ctx, item)
	return item, err
}

func (s *__PASCAL__Service) GetByID(ctx context.Context, id int64) (*model.__PASCAL__, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, errors.New("__LOWER__ not found")
	}
	return item, nil
}

func (s *__PASCAL__Service) List(ctx context.Context, p reuse.PaginationParams) ([]model.__PASCAL__, int64, error) {
	return s.repo.FindAll(ctx, p)
}

func (s *__PASCAL__Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
