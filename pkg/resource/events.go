package resource

import (
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"k8s.io/utils/ptr"
)

// EventType type of an event.
type EventType string

const (
	// EventTypeNormal signifies a normal event.
	EventTypeNormal EventType = "Normal"

	// EventTypeWarning signifies a warning event.
	EventTypeWarning EventType = "Warning"
)

type EventResources []CreateEvent

// Event allows you to specify the fields of an event to create.
type Event struct {
	// Type of the event. Optional. Should be either Normal or Warning.
	Type *EventType `json:"type"`
	// Reason of the event. Optional.
	Reason *string `json:"reason"`
	// Message of the event. Required. A template can be used. The available
	// template variables come from capturing groups in MatchCondition message
	// regular expressions.
	Message string `json:"message"`
}

// CreateEvent will create an event for the target(s).
type CreateEvent struct {
	// The target(s) to create an event for. Can be Composite or
	// CompositeAndClaim.
	Target *BindingTarget `json:"target"`

	// Event to create.
	Event Event `json:"event"`
}

func SetEvents(rsp *fnv1.RunFunctionResponse, ers EventResources) {
	for _, er := range ers {
		r := transformEvent(er)

		rsp.Results = append(rsp.Results, r)
	}
}

func transformEvent(ec CreateEvent) *fnv1.Result {
	e := &fnv1.Result{
		Reason: ec.Event.Reason,
		Target: transformTarget(ec.Target),
	}

	switch ptr.Deref(ec.Event.Type, EventTypeNormal) {
	case EventTypeNormal:
		e.Severity = fnv1.Severity_SEVERITY_NORMAL
	case EventTypeWarning:
	default:
		e.Severity = fnv1.Severity_SEVERITY_WARNING
	}

	e.Message = ec.Event.Message
	return e
}
