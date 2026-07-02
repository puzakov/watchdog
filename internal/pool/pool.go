// Package pool provides a generic object pool backed by sync.Pool.
//
// The Pool is constrained to types that implement Reset(), ensuring objects
// are properly zeroed before reuse. The Reset() method can be hand-written or
// automatically generated using the // generate:reset annotation and the
// reset code generator in cmd/reset.
package pool

import "sync"

// Reseter is the constraint interface for types that can be reset
// to their zero state before being returned to the pool.
type Reseter interface {
	Reset()
}

// Pool is a generic wrapper around sync.Pool. It stores objects of a single
// concrete type T and calls Reset() on them before they are put back,
// ensuring a clean state for the next caller.
type Pool[T Reseter] struct {
	pool sync.Pool
}

// New creates a new Pool. The newFn parameter is called by the pool when it
// needs to allocate a fresh object (i.e., when Get is called and the pool is
// empty). It returns a pointer to the Pool.
func New[T Reseter](newFn func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

// Get retrieves a T from the pool. If the pool is empty, a new T is created
// using the factory function provided to New.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put resets v by calling its Reset() method and then returns it to the pool
// for reuse.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
