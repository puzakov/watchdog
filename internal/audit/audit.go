package audit

import "time"

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
type Subject struct {
	observers []Observer
}

// NewSubject creates a new audit Subject with the given observers.
func NewSubject(observers ...Observer) *Subject {
	return &Subject{observers: observers}
}

// Subscribe adds an observer to receive audit notifications.
func (s *Subject) Subscribe(observer Observer) {
	if observer == nil {
		return
	}
	s.observers = append(s.observers, observer)
}

// Notify sends an audit event to all subscribed observers.
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
	for _, observer := range s.observers {
		observer.Notify(event)
	}
}
