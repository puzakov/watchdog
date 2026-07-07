// Package audit implements an observer-pattern audit logging system.
// It provides file-based and HTTP-based observers that receive events
// about processed metrics requests.
package audit

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/puzakov/watchdog/internal/logger"
)

// Event describes a successfully processed metrics request.
type Event struct {
	// TS is the Unix timestamp of the event.
	TS int64 `json:"ts"`
	// Metrics is the list of metric IDs that were updated.
	Metrics []string `json:"metrics"`
	// IPAddress is the client IP that sent the request.
	IPAddress string `json:"ip_address"`
}

// Observer receives audit events.
type Observer interface {
	// Notify is called when an audit event occurs.
	Notify(event Event)
}

// Subject notifies subscribed observers about audit events (Observer pattern).
// Events are processed asynchronously via a buffered channel and a worker goroutine,
// so that audit I/O never blocks the caller.
type Subject struct {
	observers []Observer
	events    chan Event
	done      chan struct{}
	closed    bool
	mu        sync.Mutex
}

// DefaultBufSize is the default channel buffer size used when NewSubject is called
// without an explicit size via NewSubjectWithBuf.
const DefaultBufSize = 1024

// NewSubjectWithBuf creates a new audit Subject that processes events asynchronously.
// bufSize is the capacity of the internal channel; events beyond that are dropped.
func NewSubjectWithBuf(bufSize int, observers ...Observer) *Subject {
	s := &Subject{
		observers: observers,
		events:    make(chan Event, bufSize),
		done:      make(chan struct{}),
	}
	go s.worker()
	return s
}

// NewSubject creates a new audit Subject with a default buffer size of 1024.
func NewSubject(observers ...Observer) *Subject {
	return NewSubjectWithBuf(DefaultBufSize, observers...)
}

// Subscribe adds an observer to receive audit notifications.
// Must be called before any Notify call to avoid races.
func (s *Subject) Subscribe(observer Observer) {
	if observer == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		logger.Log.Warn("Subscribe called on closed Subject, ignored")
		return
	}
	s.observers = append(s.observers, observer)
}

// Notify enqueues an audit event for asynchronous processing.
// If the internal buffer is full, the event is silently dropped — audit is
// best-effort and must not slow down the caller.
// Safe to call on a nil Subject — it is a no-op in that case.
func (s *Subject) Notify(metrics []string, ipAddress string) {
	if s == nil || len(s.observers) == 0 || len(metrics) == 0 {
		return
	}

	event := Event{
		TS:        time.Now().Unix(),
		Metrics:   metrics,
		IPAddress: ipAddress,
	}

	select {
	case s.events <- event:
	default:
		logger.Log.Debug("audit event dropped: buffer full",
			zap.Int("buf_size", cap(s.events)),
			zap.Int("metrics", len(metrics)))
	}
}

// Shutdown drains pending events and stops the worker goroutine.
// After Shutdown returns, no more observer calls will be made.
// Safe to call on a nil Subject, and safe to call more than once.
func (s *Subject) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	close(s.events)
	<-s.done
}

// worker reads events from the channel and fans them out to observers.
func (s *Subject) worker() {
	defer close(s.done)
	for event := range s.events {
		for _, observer := range s.observers {
			observer.Notify(event)
		}
	}
}
