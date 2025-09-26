package auth

import (
	"context"
	"fmt"
)

type Service struct {
	UserRepo               UserRepository
	PasswordResetTokenRepo PasswordResetTokenRepository
}

func (svc *Service) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := svc.UserRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return user, nil
}

func (svc *Service) GetUserByEmailAddress(ctx context.Context, emailAddress string) (*User, error) {
	user, err := svc.UserRepo.GetByEmailAddress(ctx, emailAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email address: %w", err)
	}

	return user, nil
}

func (svc *Service) GetUserByID(ctx context.Context, id string) (*User, error) {
	user, err := svc.UserRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

func (svc *Service) UserExistsByUsername(ctx context.Context, username string) (bool, error) {
	exists, err := svc.UserRepo.ExistsByUsername(ctx, username)
	if err != nil {
		return false, fmt.Errorf("failed to check if user by username exists: %w", err)
	}

	return exists, nil
}

func (svc *Service) UserExistsByEmailAddress(ctx context.Context, emailAddress string) (bool, error) {
	exists, err := svc.UserRepo.ExistsByEmailAddress(ctx, emailAddress)
	if err != nil {
		return false, fmt.Errorf("failed to check if user by email address exists: %w", err)
	}

	return exists, nil
}

func (svc *Service) CreateUser(ctx context.Context, user *User) error {
	err := svc.UserRepo.Create(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (svc *Service) UpdateUser(ctx context.Context, user *User) error {
	err := svc.UserRepo.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (svc *Service) CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error {
	err := svc.PasswordResetTokenRepo.Create(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to create password reset token: %w", err)
	}

	return nil
}

func (svc *Service) GetPasswordResetTokenByToken(ctx context.Context, tokenStr string) (*PasswordResetToken, error) {
	token, err := svc.PasswordResetTokenRepo.GetByToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get password reset token by token: %w", err)
	}

	return token, nil
}

func (svc *Service) DeletePasswordResetToken(ctx context.Context, tokenID string) error {
	err := svc.PasswordResetTokenRepo.Delete(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("failed to delete password reset token: %w", err)
	}

	return nil
}
