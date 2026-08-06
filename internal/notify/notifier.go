package notify

import (
	"context"

	"github.com/Remindal/scout/internal/domain"
)

type Notifier interface {
	Name() string
	Notify(ctx context.Context, j domain.Job) error
}
