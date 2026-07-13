package saga

import (
	"context"
	"errors"
)

var (
	ErrSagaSuspended = errors.New("saga execution suspended pending external callback")
)

// Step is a unit of work in a saga with a paired compensation for rollback.
type Step struct {
	Name       string
	Execute    func(ctx context.Context) error
	Compensate func(ctx context.Context) error
}
