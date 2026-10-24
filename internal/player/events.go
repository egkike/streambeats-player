package player

// mpv pushes lifecycle events on the same IPC socket as responses, in
// newline-delimited JSON with no request_id. The client surfaces them on a
// buffered channel so consumers can react to end-of-file instantly, while
// request/response correlation keeps working.

import "encoding/json"

// Event is one lifecycle signal pushed by mpv.
type Event struct {
	Name   string // e.g. "end-file", "audio-reconfig"
	Reason string // e.g. "eof" for natural end of track on end-file events
}

// eventPayload mirrors the "event *or* response" envelope on the wire.
type eventPayload struct {
	Event     string          `json:"event"`
	Reason    string          `json:"reason"`
	RequestID int             `json:"request_id"`
	Data      json.RawMessage `json:"data"`
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
