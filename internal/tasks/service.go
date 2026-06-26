package tasks

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrValidation        = errors.New("task validation failed")
	ErrNotFound          = errors.New("task not found")
	ErrInvalidTransition = errors.New("task transition is invalid")
)

type Service struct {
	store  *Store
	now    func() time.Time
	nextID func() string
}

func NewService(store *Store, now func() time.Time, nextID func() string) *Service {
	return &Service{
		store:  store,
		now:    now,
		nextID: nextID,
	}
}

func (s *Service) List() ([]Task, error) {
	collection, err := s.store.Load()
	if err != nil {
		return nil, err
	}

	return collection.Tasks, nil
}

func (s *Service) Get(id string) (Task, error) {
	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}

	for _, task := range collection.Tasks {
		if task.ID == id {
			return task, nil
		}
	}

	return Task{}, fmt.Errorf("%w: task %s", ErrNotFound, id)
}

func (s *Service) Create(input CreateInput) (Task, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return Task{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if !isValidPriority(input.Priority) {
		return Task{}, fmt.Errorf("%w: invalid priority %q", ErrValidation, input.Priority)
	}

	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}

	now := s.now().UTC()
	task := Task{
		ID:          s.nextID(),
		Title:       title,
		Description: strings.TrimSpace(input.Description),
		Status:      StatusTodo,
		Priority:    input.Priority,
		Tags:        append([]string(nil), input.Tags...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	collection.Tasks = append(collection.Tasks, task)
	if err := s.store.Save(collection); err != nil {
		return Task{}, err
	}

	return task, nil
}

func (s *Service) Update(id string, input UpdateInput) (Task, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return Task{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if !isValidPriority(input.Priority) {
		return Task{}, fmt.Errorf("%w: invalid priority %q", ErrValidation, input.Priority)
	}

	collection, err := s.store.Load()
	if err != nil {
		return Task{}, err
	}

	for i := range collection.Tasks {
		if collection.Tasks[i].ID != id {
			continue
		}

		collection.Tasks[i].Title = title
		collection.Tasks[i].Description = strings.TrimSpace(input.Description)
		collection.Tasks[i].Priority = input.Priority
		collection.Tasks[i].Tags = append([]string(nil), input.Tags...)
		collection.Tasks[i].UpdatedAt = s.now().UTC()

		if err := s.store.Save(collection); err != nil {
			return Task{}, err
		}

		return collection.Tasks[i], nil
	}

	return Task{}, fmt.Errorf("%w: task %s", ErrNotFound, id)
}

func (s *Service) Start(id string) error {
	return s.transition(id, StatusTodo, StatusInProgress)
}

func (s *Service) Block(id string) error {
	return s.transition(id, StatusInProgress, StatusBlocked)
}

func (s *Service) Review(id string) error {
	return s.transition(id, StatusInProgress, StatusNeedsReview)
}

func (s *Service) Reopen(id string) error {
	return s.transition(id, StatusNeedsReview, StatusInProgress)
}

func (s *Service) Complete(id string) error {
	return s.transition(id, StatusNeedsReview, StatusDone)
}

func (s *Service) Resume(id string) error {
	return s.transition(id, StatusBlocked, StatusInProgress)
}

func (s *Service) MoveToTodo(id string) error {
	return s.transition(id, StatusBlocked, StatusTodo)
}

func (s *Service) transition(id, from, to string) error {
	collection, err := s.store.Load()
	if err != nil {
		return err
	}

	for i := range collection.Tasks {
		if collection.Tasks[i].ID != id {
			continue
		}
		if collection.Tasks[i].Status != from {
			return fmt.Errorf("%w: cannot move task from %s to %s", ErrInvalidTransition, collection.Tasks[i].Status, to)
		}

		collection.Tasks[i].Status = to
		collection.Tasks[i].UpdatedAt = s.now().UTC()
		return s.store.Save(collection)
	}

	return fmt.Errorf("%w: task %s", ErrNotFound, id)
}

func isValidPriority(priority string) bool {
	switch priority {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	default:
		return false
	}
}
