package service

import (
	"context"
	"fmt"
	"strings"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

// GetUserByEmail retrieves a user by email address.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, fmt.Errorf("%w: email cannot be empty", apperrors.ErrInvalidInput)
	}
	return s.repo.GetUserByEmail(ctx, email)
}

// GetUserByID retrieves a user by UUID.
func (s *Service) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: user id cannot be empty", apperrors.ErrInvalidInput)
	}
	return s.repo.GetUserByID(ctx, id)
}

// CreateUser creates a new user with the given email and hashed password.
func (s *Service) CreateUser(ctx context.Context, email, passwordHash string) (*domain.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return nil, fmt.Errorf("%w: email cannot be empty", apperrors.ErrInvalidInput)
	}
	if passwordHash == "" {
		return nil, fmt.Errorf("%w: password hash cannot be empty", apperrors.ErrInvalidInput)
	}
	return s.repo.CreateUser(ctx, email, passwordHash)
}
