package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Category identifies the pipeline an event belongs to.
type Category string

const (
	CategoryAudit    Category = "audit"
	CategoryError    Category = "error"
	CategoryLog      Category = "log"
	CategoryMetric   Category = "metric"
	CategoryBehavior Category = "behavior"
)

// Severity level of an event.
type Severity string

const (
	SeverityDebug    Severity = "debug"
	SeverityInfo     Severity = "info"
	SeverityWarn     Severity = "warn"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Event is the canonical envelope accepted by the ingestion API.
type Event struct {
	EventID    string          `json:"event_id"`
	Timestamp  time.Time       `json:"timestamp"`
	Category   Category        `json:"category"`
	Severity   Severity        `json:"severity,omitempty"`
	Action     string          `json:"action"`
	Actor      Actor           `json:"actor,omitempty"`
	Resource   *Resource       `json:"resource,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Context    Context         `json:"context,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Status     string          `json:"status,omitempty"`
	// Set server-side. Never trusted from client.
	ApplicationID   string `json:"application_id,omitempty"`
	ApplicationCode string `json:"application_code,omitempty"`
}

type Actor struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email,omitempty"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type Resource struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
}

type Context struct {
	App       string `json:"app,omitempty"`
	Env       string `json:"env,omitempty"`
	Version   string `json:"version,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

var (
	actionRe = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`)

	ErrInvalidCategory = errors.New("invalid category")
	ErrInvalidAction   = errors.New("invalid action name (expected <resource>.<verb>, lowercase)")
	ErrMissingEventID  = errors.New("event_id required")
	ErrMissingTime     = errors.New("timestamp required")
)

// Validate checks required fields and normalizes the event.
// Returns a fingerprint that can be used for idempotency.
func (e *Event) Validate(maxSkew time.Duration) error {
	if e.EventID == "" {
		e.EventID = uuid.NewString()
	} else if _, err := uuid.Parse(e.EventID); err != nil {
		return fmt.Errorf("event_id must be uuid: %w", err)
	}
	if e.Timestamp.IsZero() {
		return ErrMissingTime
	}
	now := time.Now().UTC()
	if e.Timestamp.After(now.Add(maxSkew)) || e.Timestamp.Before(now.Add(-24*time.Hour)) {
		return fmt.Errorf("timestamp out of acceptable window")
	}
	switch e.Category {
	case CategoryAudit, CategoryError, CategoryLog, CategoryMetric, CategoryBehavior:
	default:
		return ErrInvalidCategory
	}
	if !actionRe.MatchString(e.Action) {
		return ErrInvalidAction
	}
	if e.Severity == "" {
		switch e.Category {
		case CategoryError:
			e.Severity = SeverityError
		case CategoryLog:
			e.Severity = SeverityInfo
		default:
			e.Severity = SeverityInfo
		}
	}
	return nil
}

// Subject returns the NATS subject for this event.
func (e *Event) Subject() string {
	return fmt.Sprintf("events.%s.%s", e.Category, safeCode(e.ApplicationCode))
}

func safeCode(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
