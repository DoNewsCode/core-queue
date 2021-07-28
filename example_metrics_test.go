package queue_test

import (
	"context"
	"fmt"
	"time"

	queue "github.com/DoNewsCode/core-queue"

	"github.com/DoNewsCode/core"
	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/DoNewsCode/core/otredis"
	"github.com/go-kit/kit/metrics/prometheus"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/oklog/run"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
)

type MockMetricsData struct {
	Value string
}

type MockMetricsListener struct{}

func (m MockMetricsListener) Listen() queue.Job {
	return queue.JobFrom(MockMetricsData{})
}

func (m MockMetricsListener) Process(_ context.Context, Job queue.Job) error {
	fmt.Println(Job.Data().(MockMetricsData).Value)
	return nil
}

// bootstrapMetrics is normally done when bootstrapping the framework. We mimic it here for demonstration.
func bootstrapMetrics() *core.C {
	const sampleConfig = "{\"log\":{\"level\":\"error\"},\"queue\":{\"default\":{\"parallelism\":1}}}"

	// Make sure redis is running at localhost:6379
	c := core.New(
		core.WithConfigStack(rawbytes.Provider([]byte(sampleConfig)), json.Parser()),
	)

	// Add ConfProvider
	c.ProvideEssentials()
	c.Provide(otredis.Providers())
	c.Provide(queue.Providers())
	c.Provide(di.Deps{func(appName contract.AppName, env contract.Env) queue.Gauge {
		return prometheus.NewGaugeFrom(
			stdprometheus.GaugeOpts{
				Namespace: appName.String(),
				Subsystem: env.String(),
				Name:      "queue_length",
				Help:      "The gauge JobFrom queue length",
			}, []string{"name", "channel"},
		)
	}})
	return c
}

// serveMetrics normally lives at serveMetrics command. We mimic it here for demonstration.
func serveMetrics(c *core.C, duration time.Duration) {
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

func Example_metrics() {
	c := bootstrapMetrics()

	c.Invoke(func(dispatcher *queue.Queue) {

		// Subscribe
		dispatcher.Subscribe(MockMetricsListener{})

		// Trigger an Job
		evt := queue.JobFrom(MockMetricsData{Value: "hello world"})
		_ = dispatcher.Dispatch(context.Background(), queue.Adjust(evt))
	})

	serveMetrics(c, time.Second)

	// Output:
	// hello world
}
