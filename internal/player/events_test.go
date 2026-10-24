package player

// Tests for the mpv event stream. mpv pushes newline-delimited JSON events
// (no request_id) on the same socket; the client must route end-of-file to
// an Events channel without breaking request correlation.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// eventsServer answers one correlated request and pushes a mix of events.
type eventsServer struct {
	listener net.Listener
}

func newEventsServer(t *testing.T, events []string) *eventsServer {
	t.Helper()
	l, err := net.Listen("unix", filepath.Join(t.TempDir(), "evt.sock"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &eventsServer{listener: l}
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		go func() {
			for _, e := range events {
				time.Sleep(60 * time.Millisecond)
				fmt.Fprintln(conn, e)
			}
		}()
		sc := bufio.NewScanner(conn)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			var req map[string]any
			if json.Unmarshal(sc.Bytes(), &req) != nil {
				continue
			}
			rid, _ := req["request_id"].(float64)
			fmt.Fprintln(conn, fmt.Sprintf(`{"data":null,"error":"success","request_id":%d}`, int(rid)))
		}
	}()
	t.Cleanup(func() { l.Close() })
	return s
}

func TestEventsSurfaceEndFileWithReason(t *testing.T) {
	srv := newEventsServer(t, []string{
		`{"event":"audio-reconfig"}`,                                  // noise
		`{"request_id":99,"error":"success"}`,                         // stale response, no waiter
		`{"event":"end-file","reason":"eof","playlist_entry_id":1}`,   // the one we care about
		`{"event":"end-file","reason":"error","playlist_entry_id":1}`, // failure path
	})
	c, err := Dial(srv.listener.Addr().(*net.UnixAddr).Name, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	evs := []Event{}
	deadline := time.After(5 * time.Second)
	for len(evs) < 3 {
		select {
		case ev := <-c.Events():
			evs = append(evs, ev)
		case <-deadline:
			t.Fatalf("got %d events, want 3 (funcs: %+v)", len(evs), evs)
		}
	}

	if evs[0] != (Event{Name: "audio-reconfig"}) {
		t.Errorf("events[0] = %+v, want audio-reconfig in push order", evs[0])
	}
	if evs[1] != (Event{Name: "end-file", Reason: "eof"}) {
		t.Errorf("events[1] = %+v, want end-file eof", evs[1])
	}
	if evs[2] != (Event{Name: "end-file", Reason: "error"}) {
		t.Errorf("events[2] = %+v, want end-file error", evs[2])
	}
}

func TestEventsDoNotBreakRequestCorrelation(t *testing.T) {
	srv := newEventsServer(t, []string{
		`{"event":"end-file","reason":"eof"}`, // arrives before the response
	})
	c, err := Dial(srv.listener.Addr().(*net.UnixAddr).Name, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	// still, a request made amid the event stream must get its answer
	done := make(chan error, 1)
	go func() { done <- c.Seek(1.0) }()
	// events keep flowing to the Events channel meanwhile
	select {
	case <-c.Events():
	case <-time.After(3 * time.Second):
		t.Log("no events observed; correlation is still the contract")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Seek through events: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request never completed across event traffic")
	}
}

func TestEventsChannelIsLazyButStable(t *testing.T) {
	srv := newEventsServer(t, []string{`{"event":"audio-reconfig"}`})
	c, err := Dial(srv.listener.Addr().(*net.UnixAddr).Name, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	if c.Events() == nil {
		t.Fatal("Events() must always return a usable channel")
	}
	if c.Events() != c.Events() {
		t.Fatal("Events() must return the same channel across calls")
	}
}
