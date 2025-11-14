package worker

import (
	"agenda-automator-api/internal/domain"
	"context"

	"google.golang.org/api/calendar/v3"
)

// Processor is a generic interface for a task that the worker can execute.
type Processor interface {
	Process(ctx context.Context, srv *calendar.Service, acc domain.ConnectedAccount) error
}
