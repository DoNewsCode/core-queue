package queue

import "time"

// PersistedJob represents a persisted Job.
type PersistedJob struct {
	// The UniqueId identifies each individual message. Sometimes the message can have exact same content and even
	// exact same Key. UniqueId is used to differentiate them.
	UniqueId string
	// Key is the Message type. Usually it is the string name Of the Job type before serialized.
	Key string
	// Value is the serialized bytes Of the Job.
	Value []byte
	// HandleTimeout sets the upper time limit for each run Of the handler. If handleTimeout exceeds, the Job will
	// be put onto the timeout queue. Note: the timeout is shared among all listeners.
	HandleTimeout time.Duration
	// Backoff sets the duration before next retry.
	Backoff time.Duration
	// Attempts denotes how many retry has been attempted. It starts From 1.
	Attempts int
	// MaxAttempts denotes the maximum number Of time the handler can retry before the Job is put onto
	// the failed queue.
	// By default, MaxAttempts is 1.
	MaxAttempts int
}

// Type implements Job. It returns the Key.
func (s *PersistedJob) Type() string {
	return s.Key
}

// Data implements Job. It returns the Value.
func (s *PersistedJob) Data() interface{} {
	return s.Value
}
