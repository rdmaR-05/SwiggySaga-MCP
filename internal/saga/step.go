package saga

import (
	"context"
	"errors"
)

var (
	ErrSagaSuspended = errors.New("saga execution suspended pending external callback")
)

// Step defines the atomic contract for a distributed transaction boundary, ensuring paired forward (Execute) and rollback (Compensate) mutations.
type Step struct {
	Name       string
	Execute    func(ctx context.Context) error
	Compensate func(ctx context.Context) error
}
