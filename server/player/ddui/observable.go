package ddui

import "sync"

// Observable holds a value that can be observed for changes. When Set is called,
// all registered listeners and bound send functions are notified.
type Observable[T string | int | int64 | bool] struct {
	listeners      []func(value T)
	sendFns        map[uint64]func(value T)
	nextSendID     uint64
	clientWritable bool
	value          T
	mut            sync.RWMutex
}

// NewObservable creates a new Observable with the given initial value.
// clientWritable controls whether client-originated packets may write back into
// this observable. Set it to false for server-authoritative values.
func NewObservable[T string | int | int64 | bool](initialValue T, clientWritable bool) *Observable[T] {
	return &Observable[T]{
		listeners:      make([]func(value T), 0),
		clientWritable: clientWritable,
		value:          initialValue,
	}
}

// Set updates the current value and notifies all listeners and bound send functions.
func (o *Observable[T]) Set(value T) {
	o.mut.Lock()
	o.value = value
	listeners := o.listeners
	sendFns := make([]func(value T), 0, len(o.sendFns))
	for _, fn := range o.sendFns {
		sendFns = append(sendFns, fn)
	}
	o.mut.Unlock()

	for _, fn := range listeners {
		fn(value)
	}
	for _, fn := range sendFns {
		fn(value)
	}
}

// Get returns the current value.
func (o *Observable[T]) Get() T {
	o.mut.RLock()
	defer o.mut.RUnlock()
	return o.value
}

// Listen adds a listener that is called when the value changes via Set.
func (o *Observable[T]) Listen(fn func(value T)) {
	o.mut.Lock()
	o.listeners = append(o.listeners, fn)
	o.mut.Unlock()
}

// update sets the value and notifies listeners without calling the send function.
// Used to apply changes without echoing back to the client.
func (o *Observable[T]) update(value T) {
	o.mut.Lock()
	o.value = value
	listeners := o.listeners
	o.mut.Unlock()

	for _, fn := range listeners {
		fn(value)
	}
}

// bindSend registers the send callback. It is called by form types after the
// session attaches a send function via BindSend.
func (o *Observable[T]) bindSend(fn func(T)) func() {
	o.mut.Lock()
	if o.sendFns == nil {
		o.sendFns = make(map[uint64]func(value T))
	}
	o.nextSendID++
	id := o.nextSendID
	o.sendFns[id] = fn
	o.mut.Unlock()

	return func() {
		o.mut.Lock()
		delete(o.sendFns, id)
		o.mut.Unlock()
	}
}
