<div align="center">
  <h1>core-queue</h1>
  <p>
    <strong>A simple queue implementation for package <a href="github.com/DoNewsCode/core">Core</a>.</strong>
  </p>
  <p>

[![Build](https://github.com/DoNewsCode/core-queue/actions/workflows/go.yml/badge.svg)](https://github.com/DoNewsCode/core-queue/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/DoNewsCode/core-queue.svg)](https://pkg.go.dev/github.com/DoNewsCode/core-queue)
[![codecov](https://codecov.io/gh/DoNewsCode/core-queue/branch/master/graph/badge.svg)](https://codecov.io/gh/DoNewsCode/core-queue)
[![Go Report Card](https://goreportcard.com/badge/DoNewsCode/core-queue)](https://goreportcard.com/report/DoNewsCode/core-queue)
[![Sourcegraph](https://sourcegraph.com/github.com/DoNewsCode/core-queue/-/badge.svg)](https://sourcegraph.com/github.com/DoNewsCode/core-queue?badge)

 </p>
</div>

Queues in go is not as prominent as in some other languages, since go excels
at handling concurrency. However, the deferrableDecorator queue can still offer some benefit
missing from the native mechanism, say go channels. The queued job won't be
lost even if the system shutdown. In other words, it means jobs can be retried
until success. Plus, it is also possible to queue the execution of a
particular job until a lengthy period of time. Useful when you need to
implement "send email after 30 days" type of Job handler.

## Example

```go
package main

import (
	"context"
	"fmt"
	queue "github.com/DoNewsCode/core-queue"
	"time"
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

func main() {
	queueDispatcher := queue.NewQueue(queue.NewInProcessDriver())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ch = make(chan struct{})
	go queueDispatcher.Consume(ctx)

	queueDispatcher.Subscribe(&ExampleListener{ch: ch})
	queueDispatcher.Dispatch(ctx, queue.Adjust(ExampleJob("foo"), queue.Defer(time.Second)))
	queueDispatcher.Dispatch(ctx, queue.Adjust(ExampleJob("bar"), queue.Defer(time.Hour)))

	<-ch

}

```

## GoDoc
https://pkg.go.dev/github.com/DoNewsCode/core-queue
