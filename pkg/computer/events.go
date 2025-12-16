package computer

// EventType defines the type of event occurring in the computer loop.
type EventType string

const (
	EventStatus  EventType = "status"
	EventLog     EventType = "log"
	EventError   EventType = "error"
	EventScreen  EventType = "screen"
	EventAction  EventType = "action"
	EventSafety  EventType = "safety"
	EventThinking EventType = "thinking"
)

// Event represents a single update from the computer loop.
type Event struct {
	Type      EventType
	Message   string
	Data      interface{} // Flexible payload (e.g. error, struct, byte slice)
	Timestamp int64
}

// Observer is a function that receives events.
type Observer func(Event)

// SafetyHandler is a function that handles safety decisions.
// It returns true to proceed, false to terminate.
type SafetyHandler func(explanation string) bool
