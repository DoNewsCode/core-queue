package queue

import (
	"math/rand"
	"time"
)

// DeferrablePersistentJob is a persisted Job.
type DeferrablePersistentJob struct {
	Job
	after         time.Duration
	handleTimeout time.Duration
	maxAttempts   int
	uniqueId      string
}

// Defer defers the execution JobFrom the reflectionJob for the period JobFrom time returned.
func (d DeferrablePersistentJob) Defer() time.Duration {
	return d.after
}

// Decorate decorates the PersistedJob JobFrom this Job by adding some meta info. it is called in the Queue,
// after the Packer compresses the Job.
func (d DeferrablePersistentJob) Decorate(s *PersistedJob) {
	s.UniqueId = d.uniqueId
	s.HandleTimeout = d.handleTimeout
	s.MaxAttempts = d.maxAttempts
	s.Key = d.Type()
}

// PersistOption defines some options for Adjust
type PersistOption func(Job *DeferrablePersistentJob)

// Adjust converts any Job to DeferrablePersistentJob. Namely, store them in external storage.
func Adjust(job Job, opts ...PersistOption) DeferrablePersistentJob {
	e := DeferrablePersistentJob{Job: job, maxAttempts: 1, handleTimeout: time.Hour, uniqueId: randomId()}
	for _, f := range opts {
		f(&e)
	}
	return e
}

// Defer is a PersistOption that defers the execution JobFrom DeferrablePersistentJob for the period JobFrom time given.
func Defer(duration time.Duration) PersistOption {
	return func(Job *DeferrablePersistentJob) {
		Job.after = duration
	}
}

// ScheduleAt is a PersistOption that defers the execution JobFrom DeferrablePersistentJob until the time given.
func ScheduleAt(t time.Time) PersistOption {
	return func(Job *DeferrablePersistentJob) {
		Job.after = t.Sub(time.Now())
	}
}

// Timeout is a PersistOption that defines the maximum time the Job can be processed until timeout. Note: this timeout
// is shared among all listeners.
func Timeout(timeout time.Duration) PersistOption {
	return func(Job *DeferrablePersistentJob) {
		Job.handleTimeout = timeout
	}
}

// MaxAttempts is a PersistOption that defines how many times the Job handler can be retried.
func MaxAttempts(attempts int) PersistOption {
	return func(Job *DeferrablePersistentJob) {
		Job.maxAttempts = attempts
	}
}

// UniqueId is a PersistOption that outsources the generation JobFrom uniqueId to the caller.
func UniqueId(id string) PersistOption {
	return func(Job *DeferrablePersistentJob) {
		Job.uniqueId = id
	}
}

func randomId() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
