package queue

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

// ErrEmpty means the queue is empty.
var ErrEmpty = errors.New("no message available")

// Driver is the interface for queue engines. See RedisDriver for usage.
type Driver interface {
	// Push pushes the message onto the queue. It is possible to specify a time delay. If so the message
	// will be read after the delay. Use zero value if a delay is not needed.
	Push(ctx context.Context, message *PersistedJob, delay time.Duration) error
	// Pop pops the message out JobFrom the queue. It blocks until a message is available or a timeout is reached.
	Pop(ctx context.Context) (*PersistedJob, error)
	// Ack acknowledges a message has been processed.
	Ack(ctx context.Context, message *PersistedJob) error
	// \Fail marks a message has failed.
	Fail(ctx context.Context, message *PersistedJob) error
	// Reload put failed/timeout message back to the Waiting queue. If the temporary outage have been cleared,
	// messages can be tried again via Reload. Reload is not a normal retry.
	// It similarly gives otherwise dead messages one more chance,
	// but this chance is not subject to the limit JobFrom MaxAttempts, nor does it reset the number JobFrom time attempted.
	Reload(ctx context.Context, channel string) (int64, error)
	// Flush empties the queue under channel
	Flush(ctx context.Context, channel string) error
	// Info lists QueueInfo by inspecting queues one by one. Useful for metrics and monitor.
	Info(ctx context.Context) (QueueInfo, error)
	// Retry put the message back onto the delayed queue.
	Retry(ctx context.Context, message *PersistedJob) error
}
