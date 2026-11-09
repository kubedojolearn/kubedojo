package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
)

var (
	user    = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	labA    = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	ended   = uuid.MustParse("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee")
	origin  = "http://localhost:8000"
	tickets = map[string]uuid.UUID{
		strings.Repeat("a", 43): labA,
		strings.Repeat("e", 43): ended,
	}
)

type fakeTickets struct{}

func (fakeTickets) Redeem(_ context.Context, t string) (uuid.UUID, uuid.UUID, error) {
	if lab, ok := tickets[t]; ok {
		return user, lab, nil
	}
	return uuid.Nil, uuid.Nil, ErrBadTicket
}

type fakeLabs struct {
	mu    sync.Mutex
	state string
}

func (f *fakeLabs) Get(_ context.Context, _, labID uuid.UUID) (labs.Lab, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if labID == ended {
		return labs.Lab{ID: labID, State: "abandoned", Version: 3}, nil
	}
	return labs.Lab{ID: labID, State: f.state, Version: 7}, nil
}

func (f *fakeLabs) RunCommand(_ context.Context, _, _, requestID uuid.UUID, input string) (labs.CommandResult, error) {
	if input == "boom" {
		return labs.CommandResult{}, errors.New("database down")
	}
	if input == "kubectl get pods; rm -rf /" {
		code := "UNSUPPORTED_COMMAND"
		return labs.CommandResult{RequestID: requestID, Sequence: 2, Stderr: "error: ; is shell syntax\n", ExitCode: 1, ErrorCode: &code, SnapshotVersion: 7}, nil
	}
	return labs.CommandResult{RequestID: requestID, Sequence: 1, Stdout: "deployment.apps/nginx scaled\n", Mutated: true, SnapshotVersion: 8}, nil
}

type allowAll struct{ deny bool }

func (a allowAll) Allow(context.Context, string, int, time.Duration) (ratelimit.Decision, error) {
	return ratelimit.Decision{Allowed: !a.deny, RetryAfter: time.Second}, nil
}

func newServer(t *testing.T, l *fakeLabs, limiter ratelimit.Limiter) (*httptest.Server, *Hub) {
	t.Helper()
	ips, _ := ratelimit.NewClientIPResolver("")
	hub := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	g := &Gateway{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Tickets: fakeTickets{}, Labs: l, Hub: hub,
		Limiter: limiter, IPs: ips, AllowedOrigins: []string{origin}, Heartbeat: 150 * time.Millisecond, AuthWait: 200 * time.Millisecond,
	}
	r := chi.NewRouter()
	r.Get("/api/v1/ws/labs/{labId}", g.ServeHTTP)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, hub
}

func dial(t *testing.T, srv *httptest.Server, lab uuid.UUID, o string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws/labs/" + lab.String()
	return websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {o}}})
}

func write(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

// read returns the next frame, skipping heartbeats unless asked for one.
func read(t *testing.T, c *websocket.Conn, wantType string) map[string]any {
	t.Helper()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, data, err := c.Read(ctx)
		cancel()
		if err != nil {
			t.Fatalf("waiting for %s: %v", wantType, err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		if m["type"] == "heartbeat" && wantType != "heartbeat" {
			continue
		}
		if m["type"] != wantType {
			t.Fatalf("got %v, want %s", m, wantType)
		}
		return m
	}
}

func closeCode(t *testing.T, c *websocket.Conn) websocket.StatusCode {
	t.Helper()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _, err := c.Read(ctx)
		cancel()
		if err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func sendAuth(t *testing.T, c *websocket.Conn, ticket string) {
	t.Helper()
	write(t, c, map[string]any{"type": "auth", "protocolVersion": 1, "ticket": ticket})
}

func TestSessionRunsCommandsAndForwardsNotifications(t *testing.T) {
	srv, hub := newServer(t, &fakeLabs{state: "active"}, allowAll{})
	c, _, err := dial(t, srv, labA, origin)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	sendAuth(t, c, strings.Repeat("a", 43))
	ok := read(t, c, "auth.ok")
	if ok["snapshotVersion"] != float64(7) || ok["sessionId"] != labA.String() || ok["heartbeatIntervalMs"] != float64(150) {
		t.Fatalf("auth.ok %v", ok)
	}

	rid := uuid.NewString()
	write(t, c, map[string]any{"type": "command.run", "requestId": rid, "input": "kubectl scale deploy nginx --replicas=3"})
	res := read(t, c, "command.result")
	if res["requestId"] != rid || res["stdout"] != "deployment.apps/nginx scaled\n" || res["mutated"] != true || res["snapshotVersion"] != float64(8) || res["errorCode"] != nil {
		t.Fatalf("result %v", res)
	}
	write(t, c, map[string]any{"type": "command.run", "requestId": uuid.NewString(), "input": "kubectl get pods; rm -rf /"})
	if res := read(t, c, "command.result"); res["errorCode"] != "UNSUPPORTED_COMMAND" || res["exitCode"] != float64(1) {
		t.Fatalf("unsupported %v", res)
	}
	write(t, c, map[string]any{"type": "command.run", "requestId": uuid.NewString(), "input": "boom"})
	if e := read(t, c, "error"); e["code"] != "INTERNAL" || e["fatal"] != nil {
		t.Fatalf("internal %v", e)
	}
	write(t, c, map[string]any{"type": "command.run", "requestId": uuid.NewString(), "input": "kubectl\u0000"})
	if e := read(t, c, "error"); e["code"] != "VALIDATION_FAILED" {
		t.Fatalf("invalid input %v", e)
	}

	hub.Deliver(StateChanged{LabID: labA, SnapshotVersion: 9, EventCount: 2})
	if n := read(t, c, "state.changed"); n["snapshotVersion"] != float64(9) || n["eventCount"] != float64(2) {
		t.Fatalf("notification %v", n)
	}
	hub.Deliver(StateChanged{LabID: ended, SnapshotVersion: 99}) // another lab: not delivered
	write(t, c, map[string]any{"type": "heartbeat"})
	read(t, c, "heartbeat")

	write(t, c, map[string]any{"type": "subscribe"})
	if e := read(t, c, "error"); e["code"] != "PROTOCOL_ERROR" || e["fatal"] != true {
		t.Fatalf("unknown type %v", e)
	}
	if code := closeCode(t, c); code != CloseProtocolError {
		t.Fatalf("close %d", code)
	}
}

func TestConnectionsAreAuthenticated(t *testing.T) {
	srv, _ := newServer(t, &fakeLabs{state: "active"}, allowAll{})

	if _, resp, err := dial(t, srv, labA, "https://evil.example"); err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin: %v", err)
	}
	cases := []struct {
		name  string
		lab   uuid.UUID
		first any
		code  websocket.StatusCode
		err   string
	}{
		{"bad ticket", labA, map[string]any{"type": "auth", "protocolVersion": 1, "ticket": strings.Repeat("x", 43)}, CloseUnauthenticated, "UNAUTHENTICATED"},
		{"ticket for another lab", ended, map[string]any{"type": "auth", "protocolVersion": 1, "ticket": strings.Repeat("a", 43)}, CloseUnauthenticated, "UNAUTHENTICATED"},
		{"command before auth", labA, map[string]any{"type": "command.run", "requestId": uuid.NewString(), "input": "kubectl get pods"}, CloseUnauthenticated, "UNAUTHENTICATED"},
		{"old protocol", labA, map[string]any{"type": "auth", "protocolVersion": 2, "ticket": strings.Repeat("a", 43)}, CloseProtocolError, "PROTOCOL_ERROR"},
		{"ended lab", ended, map[string]any{"type": "auth", "protocolVersion": 1, "ticket": strings.Repeat("e", 43)}, CloseLabEnded, "SESSION_EXPIRED"},
		{"no auth frame", labA, nil, CloseUnauthenticated, "UNAUTHENTICATED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _, err := dial(t, srv, tc.lab, origin)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.CloseNow() }()
			if tc.first != nil {
				write(t, c, tc.first)
			}
			if e := read(t, c, "error"); e["code"] != tc.err || e["fatal"] != true {
				t.Fatalf("error frame %v", e)
			}
			if code := closeCode(t, c); code != tc.code {
				t.Fatalf("close %d, want %d", code, tc.code)
			}
		})
	}
}

func TestRateLimitsAndReplacement(t *testing.T) {
	srv, _ := newServer(t, &fakeLabs{state: "active"}, allowAll{deny: true})
	if _, resp, err := dial(t, srv, labA, origin); err == nil || resp == nil || resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("upgrade limit: %v", err)
	}

	l := &fakeLabs{state: "active"}
	srv, _ = newServer(t, l, allowAll{})
	var conns []*websocket.Conn
	for i := 0; i < MaxSocketsPerLab+1; i++ {
		c, _, err := dial(t, srv, labA, origin)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = c.CloseNow() }()
		sendAuth(t, c, strings.Repeat("a", 43))
		read(t, c, "auth.ok")
		conns = append(conns, c)
		time.Sleep(5 * time.Millisecond) // distinct open times
	}
	if e := read(t, conns[0], "error"); e["code"] != "CONFLICT" {
		t.Fatalf("replaced %v", e)
	}
	if code := closeCode(t, conns[0]); code != CloseReplaced {
		t.Fatalf("close %d", code)
	}

	// A lab that ends elsewhere is noticed on the next heartbeat.
	l.mu.Lock()
	l.state = "completed"
	l.mu.Unlock()
	if e := read(t, conns[1], "error"); e["code"] != "SESSION_EXPIRED" {
		t.Fatalf("ended %v", e)
	}
	if code := closeCode(t, conns[1]); code != CloseLabEnded {
		t.Fatalf("close %d", code)
	}
}
