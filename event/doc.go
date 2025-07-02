// Package event defines all event types.
// To register a new event, make sure you embed the type base and add it to byTypeName:
//
//	type EventNew struct {
//		base
//
//		Some int    `json:"some"`
//		Data string `json:"data"`
//	}
//
//	func (EventNew) typeName() string { return "New" }
//
//	var byTypeName = map[string]func() Event{
//	   // ...
//	   EventNew{}.typeName(): func() (e Event) { return &EventNew{} },
//	 }
package event
