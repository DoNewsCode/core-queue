package queue

import (
	"context"
)

// Listen creates a functional listener in one line.
func Listen(Jobs []Job, callback func(ctx context.Context, Job Job) error) ListenFunc {
	return ListenFunc{
		Jobs:     Jobs,
		callback: callback,
	}
}

// ListenFunc is a listener implemented with a callback.
type ListenFunc struct {
	Jobs     []Job
	callback func(ctx context.Context, Job Job) error
}

// Listen implement Handler
func (f ListenFunc) Listen() []Job {
	return f.Jobs
}

// Process implements Handler
func (f ListenFunc) Process(ctx context.Context, Job Job) error {
	return f.callback(ctx, Job)
}
