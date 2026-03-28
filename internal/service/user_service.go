package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
)

// UserService implements user registration and authentication logic.
type UserService struct {
	users repository.UserRepository
}

// NewUserService creates a new UserService.
func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

// RegisterInput contains fields for user registration.
type RegisterInput struct {
	FullName string
	Username string
	Password string
}

// Register validates and creates a user with bcrypt password hashing.
func (s *UserService) Register(ctx context.Context, input RegisterInput) (model.User, string, error) {
	if strings.TrimSpace(input.FullName) == "" {
		return model.User{}, "full_name", ErrInvalidField
	}
	if strings.TrimSpace(input.Username) == "" {
		return model.User{}, "username", ErrInvalidField
	}
	if strings.TrimSpace(input.Password) == "" {
		return model.User{}, "password", ErrInvalidField
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, model.User{
		FullName:     strings.TrimSpace(input.FullName),
		Username:     strings.TrimSpace(input.Username),
		PasswordHash: string(hash),
	})
	if err != nil {
		if errors.Is(err, repository.ErrAlreadyExists) {
			return model.User{}, "", ErrAlreadyExists
		}

		return model.User{}, "", fmt.Errorf("create user: %w", err)
	}

	return user, "", nil
}

// Login validates credentials and returns authenticated user.
func (s *UserService) Login(ctx context.Context, username, password string) (model.User, string, error) {
	if strings.TrimSpace(username) == "" {
		return model.User{}, "username", ErrInvalidField
	}
	if strings.TrimSpace(password) == "" {
		return model.User{}, "password", ErrInvalidField
	}

	user, err := s.users.GetByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.User{}, "", ErrInvalidCredentials
		}

		return model.User{}, "", fmt.Errorf("get user by username: %w", err)
	}

	if compareErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); compareErr != nil {
		return model.User{}, "", ErrInvalidCredentials
	}

	return user, "", nil
}
