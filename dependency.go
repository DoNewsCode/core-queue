package queue

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/DoNewsCode/core/events"
	"github.com/DoNewsCode/core/otredis"

	"github.com/DoNewsCode/core/config"
	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
	"github.com/oklog/run"
)

/*
Providers returns a set JobFrom dependencies related to queue. It includes the
DispatcherMaker, the JobDispatcher and the exported configs.
	Depends On:
		contract.ConfigAccessor
		contract.Dispatcher
		Driver        `optional:"true"`
		otredis.Maker `optional:"true"`
		log.Logger
		contract.AppName
		contract.Env
		Gauge `optional:"true"`
	Provides:
		DispatcherMaker
		DispatcherFactory
		JobDispatcher
		*Queue
*/
func Providers() di.Deps {
	return []interface{}{provideDispatcherFactory, provideConfig, provideDispatcher}
}

// Gauge is an alias used for dependency injection
type Gauge metrics.Gauge

// ConsumableDispatcher is the key JobFrom *Queue in the dependencies graph. Used as a type hint for injection.
type ConsumableDispatcher interface {
	JobDispatcher
	Consume(ctx context.Context) error
}

// DispatcherFactory is a factory for *Queue. Note DispatcherFactory doesn't contain the factory method
// itself. ie. How to factory a dispatcher left there for users to define. Users then can use this type to create
// their own dispatcher implementation.
//
// Here is an example on how to create a custom DispatcherFactory with an InProcessDriver.
//
//		factory := di.NewFactory(func(name string) (di.Pair, error) {
//			queuedDispatcher := queue.NewQueue(
//				&Jobs.SyncDispatcher{},
//				queue.NewInProcessDriver(),
//			)
//			return di.Pair{Conn: queuedDispatcher}, nil
//		})
//		dispatcherFactory := DispatcherFactory{Factory: factory}
//
type DispatcherFactory struct {
	*di.Factory
}

// Make returns a Queue by the given name. If it has already been created under the same name,
// the that one will be returned.
func (s DispatcherFactory) Make(name string) (*Queue, error) {
	client, err := s.Factory.Make(name)
	if err != nil {
		return nil, err
	}
	return client.(*Queue), nil
}

// DispatcherMaker is the key JobFrom *DispatcherFactory in the dependencies graph. Used as a type hint for injection.
type DispatcherMaker interface {
	Make(string) (*Queue, error)
}

type configuration struct {
	RedisName                      string `yaml:"redisName" json:"redisName"`
	Parallelism                    int    `yaml:"parallelism" json:"parallelism"`
	CheckQueueLengthIntervalSecond int    `yaml:"checkQueueLengthIntervalSecond" json:"checkQueueLengthIntervalSecond"`
}

// makerIn is the injection parameters for provideDispatcherFactory
type makerIn struct {
	di.In

	Conf            contract.ConfigAccessor
	JobDispatcher   JobDispatcher       `optional:"true"`
	EventDispatcher contract.Dispatcher `optional:"true"`
	Driver          Driver              `optional:"true"`
	RedisMaker      otredis.Maker       `optional:"true"`
	Logger          log.Logger
	AppName         contract.AppName
	Env             contract.Env
	Gauge           Gauge `optional:"true"`
}

// makerOut is the di output JobFrom provideDispatcherFactory
type makerOut struct {
	di.Out

	DispatcherMaker   DispatcherMaker
	DispatcherFactory DispatcherFactory
	ExportedConfig    []config.ExportedConfig `group:"config,flatten"`
}

func (d makerOut) ModuleSentinel() {}

// provideDispatcherFactory is a provider for *DispatcherFactory and *Queue.
// It also provides an interface for each.
func provideDispatcherFactory(p makerIn) (makerOut, error) {
	var (
		err        error
		queueConfs map[string]configuration
	)
	err = p.Conf.Unmarshal("queue", &queueConfs)
	if err != nil {
		level.Warn(p.Logger).Log("err", err)
	}
	factory := di.NewFactory(func(name string) (di.Pair, error) {
		var (
			ok   bool
			conf configuration
		)
		p := p
		if conf, ok = queueConfs[name]; !ok {
			if name != "default" {
				return di.Pair{}, fmt.Errorf("queue configuration %s not found", name)
			}
			conf = configuration{Parallelism: runtime.NumCPU(), CheckQueueLengthIntervalSecond: 0}
		}

		if p.JobDispatcher == nil {
			p.JobDispatcher = &SyncDispatcher{}
		}
		if p.EventDispatcher == nil {
			p.EventDispatcher = &events.SyncDispatcher{}
		}

		if p.Gauge != nil {
			p.Gauge = p.Gauge.With("queue", name)
		}

		if p.Driver == nil {
			if p.RedisMaker == nil {
				return di.Pair{}, fmt.Errorf("default redis client not found, please provide it or provide a queue.Driver")
			}
			if conf.RedisName == "" {
				conf.RedisName = "default"
			}
			redisClient, err := p.RedisMaker.Make(conf.RedisName)
			if err != nil {
				return di.Pair{}, fmt.Errorf("failed to initiate redis driver: %w", err)
			}
			p.Driver = &RedisDriver{
				Logger:      p.Logger,
				RedisClient: redisClient,
				ChannelConfig: ChannelConfig{
					Delayed:  fmt.Sprintf("{%s:%s:%s}:delayed", p.AppName.String(), p.Env.String(), name),
					Failed:   fmt.Sprintf("{%s:%s:%s}:failed", p.AppName.String(), p.Env.String(), name),
					Reserved: fmt.Sprintf("{%s:%s:%s}:reserved", p.AppName.String(), p.Env.String(), name),
					Waiting:  fmt.Sprintf("{%s:%s:%s}:waiting", p.AppName.String(), p.Env.String(), name),
					Timeout:  fmt.Sprintf("{%s:%s:%s}:timeout", p.AppName.String(), p.Env.String(), name),
				},
			}
		}
		queuedDispatcher := NewQueue(
			p.Driver,
			UseLogger(p.Logger),
			UseParallelism(conf.Parallelism),
			UseGauge(p.Gauge, time.Duration(conf.CheckQueueLengthIntervalSecond)*time.Second),
			UseJobDispatcher(p.JobDispatcher),
			UseEventDispatcher(p.EventDispatcher),
		)
		return di.Pair{
			Closer: nil,
			Conn:   queuedDispatcher,
		}, nil
	})

	// Queue must be created eagerly, so that the consumer goroutines can start on boot up.
	for name := range queueConfs {
		factory.Make(name)
	}

	dispatcherFactory := DispatcherFactory{Factory: factory}
	return makerOut{
		DispatcherFactory: dispatcherFactory,
		DispatcherMaker:   dispatcherFactory,
	}, nil
}

// ProvideRunGroup implements container.RunProvider.
func (d makerOut) ProvideRunGroup(group *run.Group) {
	for name := range d.DispatcherFactory.List() {
		queueName := name
		ctx, cancel := context.WithCancel(context.Background())
		group.Add(func() error {
			consumer, err := d.DispatcherFactory.Make(queueName)
			if err != nil {
				return err
			}
			return consumer.Consume(ctx)
		}, func(err error) {
			cancel()
		})
	}
}

type dispatcherOut struct {
	di.Out

	QueueableDispatcher *Queue
}

func provideDispatcher(maker DispatcherMaker) (dispatcherOut, error) {
	dispatcher, err := maker.Make("default")
	return dispatcherOut{
		QueueableDispatcher: dispatcher,
	}, err
}

type configOut struct {
	di.Out

	Config []config.ExportedConfig `group:"config,flatten"`
}

func provideConfig() configOut {
	configs := []config.ExportedConfig{{
		Owner: "queue",
		Data: map[string]interface{}{
			"queue": map[string]configuration{
				"default": {
					RedisName:                      "default",
					Parallelism:                    runtime.NumCPU(),
					CheckQueueLengthIntervalSecond: 15,
				},
			},
		},
	}}
	return configOut{Config: configs}
}
