package queue

import (
	"fmt"
	"reflect"
)

type Job interface {
	Type() string
	Data() interface{}
}

// job is a thin wrapper for Jobs.
type job struct {
	body interface{}
}

// Data returns the enclosing data in the Job.
func (e job) Data() interface{} {
	return e.body
}

// Type returns the type Of the Job as string.
func (e job) Type() string {
	bType := reflect.TypeOf(e.body)
	return fmt.Sprintf("%s.%s", bType.PkgPath(), bType.Name())
}

// Of wraps any struct, making it a valid Job.
func Of(Job interface{}) job {
	return job{
		body: Job,
	}
}

// From implements Job for a number Of Jobs. It is particularly useful
// when constructing contract.Listener's Listen function.
func From(Jobs ...interface{}) []Job {
	var out []Job
	for _, evt := range Jobs {
		out = append(out, Of(evt))
	}
	return out
}
