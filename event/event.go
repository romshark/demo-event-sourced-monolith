package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Event is any of the types with prefix "Event" from package domain.
type Event interface {
	typeName() string
	setTime(time.Time)

	Time() time.Time
}

// New creates a new event at time.
func New[T Event](time time.Time, v T) T {
	v.setTime(time)
	return v
}

func MustMarshalJSON(e Event) string {
	var b strings.Builder
	d := json.NewEncoder(&b)
	err := d.Encode(struct {
		TypeName string `json:"type"`
		Payload  Event  `json:"payload"`
	}{
		TypeName: e.typeName(),
		Payload:  e,
	})
	if err != nil {
		panic(err)
	}
	return b.String()
}

var ErrJSONEnvelopeNoType = errors.New(`malformed envelope: missing "type" field`)

func UnmarshalJSON(s string) (Event, error) {
	r := strings.NewReader(s)
	d := json.NewDecoder(r)
	var raw struct {
		TypeName string          `json:"type"`
		Payload  json.RawMessage `json:"payload"`
	}
	if err := d.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding envelope: %w", err)
	}
	if raw.TypeName == "" {
		return nil, ErrJSONEnvelopeNoType
	}

	f := byTypeName[raw.TypeName]
	if f == nil {
		return nil, fmt.Errorf("event type name %q not registered", raw.TypeName)
	}
	e := f()

	if err := json.Unmarshal(raw.Payload, e); err != nil {
		return nil, err
	}
	return e, nil
}

// event must be embedded by every type that implements Event.
type event struct{ t time.Time }

func (b event) Time() time.Time       { return b.t }
func (b *event) setTime(tm time.Time) { b.t = tm }
