package mailer

import (
	"context"
)

type MockMailer struct{}

func (m *MockMailer) SendEmail(ctx context.Context, to, subject, body string) error {
	// Mock implementation does nothing
	return nil
}
