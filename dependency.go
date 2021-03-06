package queue

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/DoNewsCode/core/config"
	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/DoNewsCode/core/events"
	"github.com/DoNewsCode/core/otredis"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/go-kit/kit/metrics"
	"github.com/oklog/run"
	"github.com/pkg/errors"
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
func Providers(optionFunc ...ProvidersOptionFunc) di.Deps {
	option := &providersOption{}
	for _, f := range optionFunc {
		f(option)
	}
	return []interface{}{
		provideDispatcherFactory(option),
		provideConfig,
		provideDispatcher,
		di.Bind(new(DispatcherFactory), new(DispatcherMaker)),
	}
}

// Gauge is an alias used for dependency injection
type Gauge metrics.Gauge

// ConsumableDispatcher is the key of *Queue in the dependencies graph. Used as a type hint for injection.
type ConsumableDispatcher interface {
	JobDispatcher
	Consume(ctx context.Context) error
}

// configuration is the struct for queue configs.
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
	Logger          log.Logger
	Gauge           Gauge                `optional:"true"`
	Populator       contract.DIPopulator `optional:"true"`
}

// makerOut is the di output JobFrom provideDispatcherFactory
type makerOut struct {
	di.Out
	DispatcherFactory DispatcherFactory
}

func (d makerOut) ModuleSentinel() {}

func (m makerOut) Module() interface{} { return m }

// provideDispatcherFactory is a provider for *DispatcherFactory and *Queue.
// It also provides an interface for each.
func provideDispatcherFactory(option *providersOption) func(p makerIn) (makerOut, error) {
	if option.driverConstructor == nil {
		option.driverConstructor = newDefaultDriver
	}
	return func(p makerIn) (makerOut, error) {
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
				conf = configuration{
					Parallelism:                    runtime.NumCPU(),
					CheckQueueLengthIntervalSecond: 0,
				}
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

			var driver = option.driver
			if option.driver == nil {
				driver, err = option.driverConstructor(
					DriverArgs{
						Name:      name,
						Populator: p.Populator,
					},
				)
				if err != nil {
					return di.Pair{}, err
				}
			}
			queuedDispatcher := NewQueue(
				driver,
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
		}, nil
	}
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

func newDefaultDriver(args DriverArgs) (Driver, error) {
	var injected struct {
		di.In

		contract.AppName
		contract.Env
		contract.Logger
		otredis.Maker
		contract.ConfigUnmarshaler
	}

	if args.Populator == nil {
		return nil, errors.New("the default driver requires setting the populator in DI container")
	}
	if err := args.Populator.Populate(&injected); err != nil {
		return nil, fmt.Errorf("missing dependency for the default queue driver: %w", err)
	}
	driver, err := driverFromDI(args.Populator)
	if err != nil {
		return nil, fmt.Errorf("error fetching default driver from DI: %w", err)
	}
	if driver != nil {
		return driver, nil
	}
	var redisName string
	if err := injected.ConfigUnmarshaler.Unmarshal(fmt.Sprintf("queue.%s.redisName", injected.AppName), &redisName); err != nil {
		return nil, fmt.Errorf("bad configuration: %w", err)
	}

	client, err := injected.Maker.Make(redisName)
	if err != nil {
		return nil, fmt.Errorf("the default driver requires the redis client called %s: %w", redisName, err)
	}
	return &RedisDriver{
		Logger:      injected.Logger,
		RedisClient: client,
		ChannelConfig: ChannelConfig{
			Delayed:  fmt.Sprintf("{%s:%s:%s}:delayed", injected.AppName.String(), injected.Env.String(), args.Name),
			Failed:   fmt.Sprintf("{%s:%s:%s}:failed", injected.AppName.String(), injected.Env.String(), args.Name),
			Reserved: fmt.Sprintf("{%s:%s:%s}:reserved", injected.AppName.String(), injected.Env.String(), args.Name),
			Waiting:  fmt.Sprintf("{%s:%s:%s}:waiting", injected.AppName.String(), injected.Env.String(), args.Name),
			Timeout:  fmt.Sprintf("{%s:%s:%s}:timeout", injected.AppName.String(), injected.Env.String(), args.Name),
		},
	}, nil
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

func driverFromDI(populator contract.DIPopulator) (Driver, error) {
	var injected struct {
		di.In
		Driver `optional:"true"`
	}
	err := populator.Populate(&injected)
	if err != nil {
		return nil, err
	}
	return injected.Driver, nil
}
