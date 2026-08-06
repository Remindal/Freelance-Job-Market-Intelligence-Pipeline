package notify

import (
	"context"

	"upwork-scout/internal/domain"
)

type Notifier interface {
	Name() string
	Notify(ctx context.Context, j domain.Job) error
}
