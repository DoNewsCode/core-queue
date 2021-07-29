package queue

import "github.com/DoNewsCode/core/di"

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

// DispatcherMaker is the key of *DispatcherFactory in the dependencies graph. Used as a type hint for injection.
type DispatcherMaker interface {
	Make(string) (*Queue, error)
}
