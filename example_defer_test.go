package queue_test

import (
	"context"
	"fmt"
	"time"

	queue "github.com/DoNewsCode/core-queue"

	"github.com/DoNewsCode/core"
	"github.com/DoNewsCode/core/otredis"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/oklog/run"
)

type DeferMockData struct {
	Value string
}

type DeferMockListener struct{}

func (m DeferMockListener) Listen() queue.Job {
	return queue.JobFrom(DeferMockData{})
}

func (m DeferMockListener) Process(_ context.Context, Job queue.Job) error {
	fmt.Println(Job.Data().(DeferMockData).Value)
	return nil
}

// bootstrapMetrics is normally done when bootstrapping the framework. We mimic it here for demonstration.
func bootstrapDefer() *core.C {
	const sampleConfig = "{\"log\":{\"level\":\"error\"},\"queue\":{\"default\":{\"parallelism\":1}}}"
	// Make sure redis is running at localhost:6379
	c := core.New(
		core.WithConfigStack(rawbytes.Provider([]byte(sampleConfig)), json.Parser()),
	)

	// Add ConfProvider
	c.ProvideEssentials()
	c.Provide(otredis.Providers())
	c.Provide(queue.Providers())
	return c
}

// serveMetrics normally lives at serveMetrics command. We mimic it here for demonstration.
func serveDefer(c *core.C, duration time.Duration) {
	var g run.Group

	c.ApplyRunGroup(&g)

	// cancel the run group after some time, so that the program ends. In real project, this is not necessary.
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	g.Add(func() error {
		<-ctx.Done()
		return nil
	}, func(err error) {
		cancel()
	})

	err := g.Run()
	if err != nil {
		panic(err)
	}
}

func Example_defer() {
	c := bootstrapDefer()

	c.Invoke(func(dispatcher *queue.Queue) {
		// Subscribe
		dispatcher.Subscribe(DeferMockListener{})

		// Trigger an Job
		evt := queue.JobFrom(DeferMockData{Value: "hello world"})
		_ = dispatcher.Dispatch(context.Background(), queue.Adjust(evt, queue.Defer(time.Second)))
		_ = dispatcher.Dispatch(context.Background(), queue.Adjust(evt, queue.Defer(time.Hour)))
	})

	serveDefer(c, 2*time.Second)

	// Output:
	// hello world
}
