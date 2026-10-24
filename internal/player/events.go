package player

// mpv pushes lifecycle events on the same IPC socket as responses, in
// newline-delimited JSON with no request_id. The client surfaces them on a
// buffered channel so consumers can react to end-of-file instantly, while
// request/response correlation keeps working.

import "encoding/json"

// Event is one lifecycle signal pushed by mpv.
type Event struct {
	Name     string // e.g. "end-file", "property-change"
	Reason   string // e.g. "eof" for natural end of track on end-file events
	Property string // property-change: which property changed
	Bool     *bool  // property-change: parsed bool data (idle-active ...)
}

// eventPayload mirrors the "event *or* response" envelope on the wire.
type eventPayload struct {
	Event     string          `json:"event"`
	Reason    string          `json:"reason"`
	RequestID int             `json:"request_id"`
	Property  string          `json:"name"` // property-change: property name
	Data      json.RawMessage `json:"data"` // property-change: value
}

// eventsBufferSize holds a burst of lifecycle signals; a slow consumer drops
// oldest-style (non-blocking push) instead of stalling the read loop.
const eventsBufferSize = 16

// Events returns the channel mpv lifecycle events arrive on. It is stable
// per client: every call returns the same channel.
func (c *Client) Events() <-chan Event { return c.events }

// forwardEvent delivers one event to subscribers without ever blocking the
// read loop; an overflowing channel drops the event.
func (c *Client) forwardEvent(ev Event) {
	select {
	case c.events <- ev:
	default:
	}
}
