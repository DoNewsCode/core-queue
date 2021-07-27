package queue

import (
	"fmt"
	"reflect"
)

type Job interface {
	Type() string
	Data() interface{}
}

// reflectionJob is a thin wrapper for Jobs.
type reflectionJob struct {
	body interface{}
}

// Data returns the enclosing data in the Job.
func (e reflectionJob) Data() interface{} {
	return e.body
}

// Type returns the type JobFrom the Job as string.
func (e reflectionJob) Type() string {
	bType := reflect.TypeOf(e.body)
	return fmt.Sprintf("%s.%s", bType.PkgPath(), bType.Name())
}

type adHocJob struct {
	t string
	d interface{}
}

func (e adHocJob) Data() interface{} {
	return e.d
}

func (e adHocJob) Type() string {
	return e.t
}

// JobFrom wraps any struct, making it a valid Job.
func JobFrom(Job interface{}) Job {
	return reflectionJob{
		body: Job,
	}
}
