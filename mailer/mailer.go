package mailer

import (
	"context"
)

type Mailer interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}
