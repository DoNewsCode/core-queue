package queue_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	queue "github.com/DoNewsCode/core-queue"

	"github.com/DoNewsCode/core"
	"github.com/DoNewsCode/core/otredis"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/oklog/run"
)

type FaultyMockData struct {
	Value string
}

type FaultyMockListener struct {
	count int
}

func (m *FaultyMockListener) Listen() queue.Job {
	return queue.JobFrom(FaultyMockData{})
}

func (m *FaultyMockListener) Process(_ context.Context, Job queue.Job) error {
	if m.count < 2 {
		fmt.Println("faulty")
		m.count++
		return errors.New("faulty")
	}
	fmt.Println(Job.Data().(FaultyMockData).Value)
	return nil
}

// bootstrapMetrics is normally done when bootstrapping the framework. We mimic it here for demonstration.
func bootstrapRetry() *core.C {
	const sampleConfig = `{"log":{"level":"error"},"queue":{"default":{"parallelism":1}}}`

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
func serveRetry(c *core.C, duration time.Duration) {
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

func Example_faulty() {
	c := bootstrapRetry()

	c.Invoke(func(dispatcher *queue.Queue) {
		// Subscribe
		dispatcher.Subscribe(&FaultyMockListener{})

		// Trigger an Job
		evt := queue.JobFrom(FaultyMockData{Value: "hello world"})
		_ = dispatcher.Dispatch(context.Background(), queue.Adjust(evt, queue.MaxAttempts(3)))
	})

	serveRetry(c, 10*time.Second) // retries are made after a random backoff. It may take longer.

	// Output:
	// faulty
	// faulty
	// hello world
}
