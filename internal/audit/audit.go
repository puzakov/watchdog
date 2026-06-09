package audit

import "time"

// Event describes a successfully processed metrics request.
type Event struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer receives audit events.
type Observer interface {
	Notify(event Event)
}

// Subject notifies subscribed observers about audit events.
type Subject struct {
	observers []Observer
}

func NewSubject(observers ...Observer) *Subject {
	return &Subject{observers: observers}
}

func (s *Subject) Subscribe(observer Observer) {
	if observer == nil {
		return
	}
	s.observers = append(s.observers, observer)
}

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
