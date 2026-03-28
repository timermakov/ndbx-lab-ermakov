package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/timermakov/ndbx-lab-ermakov/internal/model"
	"github.com/timermakov/ndbx-lab-ermakov/internal/repository"
	"github.com/timermakov/ndbx-lab-ermakov/internal/service"
)

type userRepoStub struct {
	createFn        func(ctx context.Context, user model.User) (model.User, error)
	getByUsernameFn func(ctx context.Context, username string) (model.User, error)
}

func (s userRepoStub) EnsureIndexes(context.Context) error {
	return nil
}

func (s userRepoStub) Create(ctx context.Context, user model.User) (model.User, error) {
	if s.createFn != nil {
		return s.createFn(ctx, user)
	}
	return model.User{}, nil
}

func (s userRepoStub) GetByUsername(ctx context.Context, username string) (model.User, error) {
	if s.getByUsernameFn != nil {
		return s.getByUsernameFn(ctx, username)
	}
	return model.User{}, repository.ErrNotFound
}

func TestUserServiceRegisterInvalidFullName(t *testing.T) {
	t.Parallel()

	svc := service.NewUserService(userRepoStub{})
	_, field, err := svc.Register(context.Background(), service.RegisterInput{
		FullName: "",
		Username: "john",
		Password: "secret",
	})

	if !errors.Is(err, service.ErrInvalidField) {
		t.Fatalf("expected ErrInvalidField, got %v", err)
	}
	if field != "full_name" {
		t.Fatalf("expected full_name field, got %q", field)
	}
}

func TestUserServiceRegisterConflict(t *testing.T) {
	t.Parallel()

	svc := service.NewUserService(userRepoStub{
		createFn: func(_ context.Context, _ model.User) (model.User, error) {
			return model.User{}, repository.ErrAlreadyExists
		},
	})

	_, _, err := svc.Register(context.Background(), service.RegisterInput{
		FullName: "John Doe",
		Username: "john",
		Password: "secret",
	})

	if !errors.Is(err, service.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUserServiceLoginInvalidCredentials(t *testing.T) {
	t.Parallel()

	svc := service.NewUserService(userRepoStub{
		getByUsernameFn: func(_ context.Context, _ string) (model.User, error) {
			return model.User{}, repository.ErrNotFound
		},
	})

	_, _, err := svc.Login(context.Background(), "john", "wrong")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
