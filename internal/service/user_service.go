package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
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

// UsersQuery holds GET /users query parameters.
type UsersQuery struct {
	ID     string
	Name   string
	Limit  string
	Offset string
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

// ValidateListQuery validates and converts users list query parameters.
func (s *UserService) ValidateListQuery(query UsersQuery) (repository.UserFilter, string, error) {
	filter := repository.UserFilter{
		ID:   strings.TrimSpace(query.ID),
		Name: strings.TrimSpace(query.Name),
	}

	if filter.ID != "" {
		if _, err := primitive.ObjectIDFromHex(filter.ID); err != nil {
			return repository.UserFilter{}, "id", ErrInvalidParameter
		}
	}

	if strings.TrimSpace(query.Limit) != "" {
		limit, err := strconv.ParseUint(strings.TrimSpace(query.Limit), 10, 64)
		if err != nil {
			return repository.UserFilter{}, "limit", ErrInvalidParameter
		}
		filter.Limit = limit
	}

	if strings.TrimSpace(query.Offset) != "" {
		offset, err := strconv.ParseUint(strings.TrimSpace(query.Offset), 10, 64)
		if err != nil {
			return repository.UserFilter{}, "offset", ErrInvalidParameter
		}
		filter.Offset = offset
	}

	return filter, "", nil
}

// List returns users by filter.
func (s *UserService) List(ctx context.Context, filter repository.UserFilter) ([]model.User, error) {
	users, err := s.users.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	return users, nil
}

// GetByID returns user by id.
func (s *UserService) GetByID(ctx context.Context, id string) (model.User, error) {
	user, err := s.users.GetByID(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.User{}, ErrNotFound
		}

		return model.User{}, fmt.Errorf("get user by id: %w", err)
	}

	return user, nil
}
