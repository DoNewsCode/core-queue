package queue

import (
	"context"
)

// Listen creates a functional listener in one line.
func Listen(job Job, callback func(ctx context.Context, job Job) error) ListenFunc {
	return ListenFunc{
		Job:      job,
		callback: callback,
	}
}

// ListenFunc is a listener implemented with a callback.
type ListenFunc struct {
	Job      Job
	callback func(ctx context.Context, Job Job) error
}

// Listen implement Handler
func (f ListenFunc) Listen() Job {
	return f.Job
}

// Process implements Handler
func (f ListenFunc) Process(ctx context.Context, Job Job) error {
	return f.callback(ctx, Job)
}
