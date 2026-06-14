package install

import (
	"context"
	"errors"
	"strings"

	"lightmonitor/internal/domain/system"
)

var (
	ErrAlreadyInstalled = errors.New("system already installed")
	ErrInvalidAdmin     = errors.New("invalid administrator account")
)

type Repository interface {
	IsInstalled(ctx context.Context) (bool, error)
	Install(ctx context.Context, admin system.User) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) IsInstalled(ctx context.Context) (bool, error) {
	return s.repo.IsInstalled(ctx)
}

func (s *Service) Install(ctx context.Context, username, password string) error {
	if strings.TrimSpace(username) == "" || len(password) < 8 {
		return ErrInvalidAdmin
	}

	installed, err := s.repo.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		return ErrAlreadyInstalled
	}

	passwordHash, err := system.HashPassword(password)
	if err != nil {
		return err
	}

	return s.repo.Install(ctx, system.User{
		Username:     strings.TrimSpace(username),
		PasswordHash: passwordHash,
		Role:         system.UserRoleAdmin,
		Enabled:      true,
	})
}
