package queue

type event string

const (
	// BeforeRetry is an event that triggers when the job failed previously is going to be retried.
	// Note: if retry attempts are exhausted, this event won't be triggered.
	BeforeRetry event = "beforeRetry"
	// BeforeAbort is an event that triggers when the job failed previously is going
	// to be aborted. If the Job still has retry attempts remaining, this event won't
	// be triggered.
	BeforeAbort event = "beforeAbort"
)

type BeforeAbortPayload struct {
	Err error
	Job *PersistedJob
}

type BeforeRetryPayload struct {
	Err error
	Job *PersistedJob
}
