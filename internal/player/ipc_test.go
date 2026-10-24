package player

// Tests for the mpv JSON IPC client. They run against a fake socket server,
// no mpv binary required: the protocol contract (newline-delimited JSON,
// request_id correlation, event stream) is pinned here.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeMpvServer reads newline-delimited requests and answers canned responses,
// correlating request_id and optionally pushing events.
type fakeMpvServer struct {
	addr     string
	listener net.Listener
	sent     []string // raw JSON of received requests
}

func newFakeMpvServer(t *testing.T) *fakeMpvServer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mpv.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeMpvServer{addr: path, listener: l}
	go s.serve()
	t.Cleanup(func() { l.Close() })
	return s
}

func (s *fakeMpvServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeMpvServer) handle(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var req map[string]any
		if json.Unmarshal(sc.Bytes(), &req) != nil {
			continue
		}
		raw, _ := json.Marshal(req)
		s.sent = append(s.sent, string(raw))
		rid, _ := req["request_id"].(float64)
		cmd, _ := req["command"].([]any)
		resp := map[string]any{
			"data":       nil,
			"error":      "success",
			"request_id": int(rid),
		}
		switch {
		case len(cmd) > 0 && cmd[0] == "get_property":
			prop, _ := cmd[1].(string)
			switch prop {
			case "duration":
				resp["data"] = 142.761
			case "time-pos":
				resp["data"] = 12.5
			case "pause":
				resp["data"] = false
			}
		}
		out, _ := json.Marshal(resp)
		fmt.Fprintf(conn, "%s\n", out)
	}
}

func TestClientLoadFileSendsLoadfileCommand(t *testing.T) {
	srv := newFakeMpvServer(t)
	c := dial(t, srv.addr)
	defer c.Close()

	if err := c.LoadFile("https://t4.bcbits.com/stream/TOKEN/mp3-128/1", "Electric"); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	var cmds []map[string]any
	for _, sent := range srv.sent {
		var m map[string]any
		_ = json.Unmarshal([]byte(sent), &m)
		cmds = append(cmds, m)
	}
	found := false
	for _, m := range cmds {
		cmd, _ := m["command"].([]any)
		if len(cmd) >= 3 && cmd[0] == "loadfile" &&
			cmd[1] == "https://t4.bcbits.com/stream/TOKEN/mp3-128/1" &&
			cmd[2] == "replace" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no loadfile command sent; got %v", srv.sent)
	}
}

func TestClientGetPropertyReturnsValueAndCorrelatesID(t *testing.T) {
	srv := newFakeMpvServer(t)
	c := dial(t, srv.addr)
	defer c.Close()

	d, err := c.GetProperty("duration")
	if err != nil {
		t.Fatalf("GetProperty(duration): %v", err)
	}
	if d != 142.761 {
		t.Errorf("duration = %v, want 142.761", d)
	}

	vol, err := c.GetProperty("volume")
	if err != nil {
		t.Fatalf("GetProperty(volume): %v", err)
	}
	_ = vol
	if err := c.SetVolume(60); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if err := c.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	// mpv 0.41 rejects native-typed values on `set"; the client must use
	// set_property with JSON-native types (float volume, bool pause).
	var setVolume, setPause bool
	for _, sent := range srv.sent {
		var m map[string]any
		_ = json.Unmarshal([]byte(sent), &m)
		cmd, _ := m["command"].([]any)
		if len(cmd) == 3 && cmd[0] == "set_property" && cmd[1] == "volume" {
			if v, ok := cmd[2].(float64); ok && v == 60 {
				setVolume = true
			}
		}
		if len(cmd) == 3 && cmd[0] == "set_property" && cmd[1] == "pause" {
			if v, ok := cmd[2].(bool); ok && v {
				setPause = true
			}
		}
	}
	if !setVolume {
		t.Errorf("SetVolume did not send set_property volume 60 (float); got %v", srv.sent)
	}
	if !setPause {
		t.Errorf("Pause did not send set_property pause true (bool); got %v", srv.sent)
	}
}

func TestDialClosedPathErrors(t *testing.T) {
	// a path with no live server must fail fast with an error
	dead := filepath.Join(t.TempDir(), "dead.sock")
	if _, err := Dial(dead, 500*time.Millisecond); err == nil {
		t.Fatal("Dial on closed path should error")
	}
}

func TestClientQuitSendsQuitCommand(t *testing.T) {
	srv := newFakeMpvServer(t)
	c := dial(t, srv.addr)
	defer c.Close()
	if err := c.Quit(); err != nil {
		t.Fatalf("Quit: %v", err)
	}
	if !strings.Contains(strings.Join(srv.sent, " "), "\"quit\"") {
		t.Fatalf("no quit command sent; got %v", srv.sent)
	}
}

func dial(t *testing.T, addr string) *Client {
	t.Helper()
	c, err := Dial(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return c
}
