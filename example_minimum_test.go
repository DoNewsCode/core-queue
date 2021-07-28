package queue_test

import (
	"context"
	"fmt"
	"time"

	queue "github.com/DoNewsCode/core-queue"
)

type ExampleJob string

func (e ExampleJob) Type() string {
	return "example"
}

func (e ExampleJob) Data() interface{} {
	return e
}

type ExampleListener struct {
	ch chan struct{}
}

func (e *ExampleListener) Listen() queue.Job {
	return ExampleJob("")
}

func (e *ExampleListener) Process(ctx context.Context, job queue.Job) error {
	fmt.Println(job.Data())
	e.ch <- struct{}{}
	return nil
}

func Example_minimum() {
	queueDispatcher := queue.NewQueue(queue.NewInProcessDriver())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ch = make(chan struct{})
	go queueDispatcher.Consume(ctx)

	queueDispatcher.Subscribe(&ExampleListener{ch: ch})
	queueDispatcher.Dispatch(ctx, queue.Adjust(ExampleJob("foo"), queue.Defer(time.Second)))
	queueDispatcher.Dispatch(ctx, queue.Adjust(ExampleJob("bar"), queue.Defer(time.Hour)))

	<-ch

	// Output:
	// foo
}
