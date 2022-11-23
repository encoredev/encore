package watcher

import "os"

type EventType = string

const (
	CREATED  EventType = "Created"
	MODIFIED EventType = "Modified"
	DELETED  EventType = "Deleted"
)

type Event struct {
	EventType EventType
	Path      string
	Info      os.FileInfo
}

type Events struct {
	latestEvents map[string]Event // Latest event per file
}

func newEventBatch() *Events {
	return &Events{
		latestEvents: make(map[string]Event),
	}
}

func (e *Events) addEvent(path string, event EventType, info os.FileInfo) {
	e.latestEvents[path] = Event{
		EventType: event,
		Path:      path,
		Info:      info,
	}
}

func (e *Events) Events() []Event {
	events := make([]Event, 0, len(e.latestEvents))
	for _, event := range e.latestEvents {
		events = append(events, event)
	}

	return events
}
