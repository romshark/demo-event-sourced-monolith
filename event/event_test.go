package event

import (
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type EventTest struct {
	event
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func (EventTest) typeName() string { return "Test" }

func init() {
	// Register test event type.
	byTypeName[EventTest{}.typeName()] = func() Event { return &EventTest{} }
}

// EventTestUnregistered is intentionally not added to byTypeName.
type EventTestUnregistered struct {
	event
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func (EventTestUnregistered) typeName() string { return "TestUnregistered" }

func TestJSONCodec(t *testing.T) {
	newEvent := New(time.Time{}, &EventTest{
		Foo: "foo-value",
		Bar: 42,
	})
	e := MustMarshalJSON(newEvent)
	require.JSONEq(t, `{
		"type": "Test",
		"payload": {
			"foo": "foo-value",
			"bar": 42
		}
	}`, e)

	unmarshaled, err := UnmarshalJSON(e)
	require.NoError(t, err)
	require.Equal(t, newEvent, unmarshaled)
}

func TestErrJSONUnmarshalUnregistered(t *testing.T) {
	unmarshaled, err := UnmarshalJSON(`{
		"type": "TestUnregistered",
		"payload": {
			"foo": "foo-value",
			"bar": 42
		}
	}`)
	require.Error(t, err)
	require.Equal(t, `event type name "TestUnregistered" not registered`, err.Error())
	require.Nil(t, unmarshaled)
}

func TestErrJSONUnmarshalMalformedEnvelopeEOF(t *testing.T) {
	unmarshaled, err := UnmarshalJSON("")
	require.ErrorContains(t, err, "decoding envelope:")
	require.ErrorIs(t, err, io.EOF)
	require.Nil(t, unmarshaled)
}

func TestErrJSONUnmarshalMalformedEnvelopeNoType(t *testing.T) {
	unmarshaled, err := UnmarshalJSON(`{
		"type": "",
		"payload": {
			"foo": "foo-value",
			"bar": 42
		}
	}`)
	require.ErrorIs(t, err, ErrJSONEnvelopeNoType)
	require.Nil(t, unmarshaled)
}

func TestErrJSONUnmarshalMalformedEnvelopePayload(t *testing.T) {
	unmarshaled, err := UnmarshalJSON(`{
		"type": "Test",
		"payload": []
	}`)
	require.ErrorContains(t, err,
		"json: cannot unmarshal array into Go value of type event.EventTest")
	require.Nil(t, unmarshaled)
}

func TestAllEvents(t *testing.T) {
	for typeName, f := range byTypeName {
		t.Run(typeName, func(t *testing.T) {
			require.NotEmpty(t, typeName)
			e := New(time.Time{}, f())

			require.Equal(t, typeName, e.typeName(),
				"byTypeName key must equal the return value of method typeName")

			rt := reflect.TypeOf(e)
			if rt.Kind() == reflect.Pointer {
				rt = rt.Elem()
			}
			expected := strings.TrimPrefix(rt.Name(), "Event")
			require.Equal(t, expected, typeName,
				"typeName() must equal struct name without the 'Event' prefix")

			s := MustMarshalJSON(e)
			u, err := UnmarshalJSON(s)
			require.NoError(t, err)
			require.Equal(t, e, u)
		})
	}
}
