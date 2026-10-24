package player

// mpv JSON IPC client. mpv exposes a newline-delimited JSON protocol on a Unix
// socket (--input-ipc-server=...). Each request carries an integer request_id;
// every response echoes it, async events do not. This file pins that protocol
// contract for the whole player backend.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// response is the server answer to one correlated request.
type response struct {
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
	RequestID int             `json:"request_id"`
}

// Client speaks the mpv JSON IPC protocol over a Unix socket.
type Client struct {
	conn net.Conn

	writeMu sync.Mutex
	idMu    sync.Mutex
	nextID  int

	pendingMu sync.Mutex
	pending   map[int]chan response

	events chan Event
}

// Dial connects to a running mpv instance's IPC socket.
func Dial(addr string, timeout time.Duration) (*Client, error) {
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(context.Background(), "unix", addr)
	if err != nil {
		return nil, fmt.Errorf("dial mpv ipc %s: %w", addr, err)
	}
	c := &Client{
		conn:    conn,
		pending: map[int]chan response{},
		events:  make(chan Event, eventsBufferSize),
	}
	go c.readLoop()
	return c, nil
}

// readLoop dispatches responses to waiting requests; events are ignored here
// until a consumer needs them.
func (c *Client) readLoop() {
	sc := bufio.NewScanner(c.conn)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var payload eventPayload
		if json.Unmarshal(sc.Bytes(), &payload) != nil {
			continue
		}
		// An event pushes without a request_id and lands on Events().
		if payload.Event != "" {
			ev := Event{Name: payload.Event, Reason: payload.Reason}
			if payload.Event == "property-change" {
				ev.Property = payload.Property
				var b bool
				if json.Unmarshal(payload.Data, &b) == nil {
					ev.Bool = &b
				}
			}
			c.forwardEvent(ev)
			continue
		}
		var resp response
		if json.Unmarshal(sc.Bytes(), &resp) != nil {
			continue
		}
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.RequestID]
		if ok {
			delete(c.pending, resp.RequestID)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

// Request sends one raw command and waits for the correlated response.
func (c *Client) Request(args ...any) error {
	_, err := c.requestRaw(args)
	return err
}

// requestRaw sends the command and returns the decoded response envelope.
func (c *Client) requestRaw(args []any) (response, error) {
	c.idMu.Lock()
	c.nextID++
	id := c.nextID
	c.idMu.Unlock()

	ch := make(chan response, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	payload, err := json.Marshal(map[string]any{
		"command":    args,
		"request_id": id,
	})
	if err != nil {
		return response{}, fmt.Errorf("encode command: %w", err)
	}
	c.writeMu.Lock()
	if _, err := c.conn.Write(append(payload, '\n')); err != nil {
		c.writeMu.Unlock()
		return response{}, fmt.Errorf("send command: %w", err)
	}
	c.writeMu.Unlock()

	select {
	case resp := <-ch:
		if resp.Error != "" && resp.Error != "success" {
			return resp, fmt.Errorf("mpv error: %s", resp.Error)
		}
		return resp, nil
	case <-time.After(5 * time.Second):
		return response{}, fmt.Errorf("mpv timeout waiting for request_id %d", id)
	}
}

// LoadFile plays a URL or path, clearing the queue, and names the entry.
func (c *Client) LoadFile(path, title string) error {
	// mpv removed the old "play" loadfile flag; "replace" starts playing
	// immediately, clearing the queue (verified against mpv 0.41: "play"
	// answers "invalid parameter").
	if err := c.Request("loadfile", path, "replace"); err != nil {
		return err
	}
	// media-title is read-only; the writable option is force-media-title.
	if title != "" {
		return c.SetProperty("force-media-title", title)
	}
	return nil
}

// GetProperty returns a numeric property (duration, time-pos, volume...).
func (c *Client) GetProperty(prop string) (float64, error) {
	resp, err := c.requestRaw([]any{"get_property", prop})
	if err != nil {
		return 0, err
	}
	var v float64
	if err := json.Unmarshal(resp.Data, &v); err != nil {
		return 0, fmt.Errorf("property %s: %w", prop, err)
	}
	return v, nil
}

// SetProperty writes a property using mpv's native-typed set_property
// command. mpv 0.41 rejects JSON-native types on the `set` command (its
// arguments are parsed as strings), so typed writes go through set_property.
func (c *Client) SetProperty(prop string, value any) error {
	return c.Request("set_property", prop, value)
}

// SetVolume sets playback volume in percent [0..100].
func (c *Client) SetVolume(percent int) error {
	return c.SetProperty("volume", float64(percent))
}

// Pause pauses playback.
func (c *Client) Pause() error { return c.SetProperty("pause", true) }

// Play resumes playback.
func (c *Client) Play() error { return c.SetProperty("pause", false) }

// Stop clears what is playing.
func (c *Client) Stop() error { return c.Request("stop") }

// Seek jumps relative seconds from the current position.
func (c *Client) Seek(seconds float64) error {
	return c.Request("seek", seconds, "relative")
}

// Quit tells mpv to exit entirely: playback stops and the mpv process ends,
// so audio cannot outlive the player panel after a quit.
func (c *Client) Quit() error { return c.Request("quit") }

// ObserveIdleActive registers an observer for the idle-active property
// (docs/ipc.rst: observe_property <id> <property>). idle-active is the
// authoritative track-end telemetry: true ONLY when no file is loaded, so
// buffering/underruns cannot flip it. Value changes arrive as property-change
// events on Events().
func (c *Client) ObserveIdleActive() error {
	return c.Request("observe_property", idleActiveObserveID, "idle-active")
}

const idleActiveObserveID = 777

// Close releases the socket.
func (c *Client) Close() error { return c.conn.Close() }
