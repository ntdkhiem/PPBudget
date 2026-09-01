package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

// GenerateAPIToken generates a new secure API token for the user and saves it in user_settings
func (s *Service) GenerateAPIToken(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("%w: missing user id", apperrors.ErrInvalidInput)
	}

	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	token := hex.EncodeToString(bytes)

	err := s.repo.SetUserSetting(ctx, userID, "api_token", token)
	if err != nil {
		return "", fmt.Errorf("failed to store api token: %w", err)
	}
	return token, nil
}

// GetAPIToken returns the current API token for the user
func (s *Service) GetAPIToken(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("%w: missing user id", apperrors.ErrInvalidInput)
	}
	return s.repo.GetUserSetting(ctx, userID, "api_token")
}

// GetUserByAPIToken returns the user_id associated with the api token
func (s *Service) GetUserByAPIToken(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	return s.repo.GetUserByAPIToken(ctx, token)
}
