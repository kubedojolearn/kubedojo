package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
)

// Protocol limits and timings (docs/api/websocket.md).
const (
	MaxFrameBytes     = 32 << 10
	AuthTimeout       = 3 * time.Second
	HeartbeatInterval = 25 * time.Second
	writeTimeout      = 5 * time.Second
)

// Close codes (websocket.md).
const (
	CloseProtocolError   websocket.StatusCode = 4400
	CloseUnauthenticated websocket.StatusCode = 4401
	CloseIdle            websocket.StatusCode = 4408
	CloseReplaced        websocket.StatusCode = 4409
	CloseLabEnded        websocket.StatusCode = 4410
)

// Rate limits shared with REST: upgrades per IP, and commands per user (the REST "commands" rule).
var (
	upgradeLimit = ratelimit.Rule{Name: "ws-upgrade", Limit: 30, Window: time.Minute}
	commandRule  = ratelimit.Rule{Name: "commands", Limit: 100, Window: time.Minute}
)

// Labs is what the gateway needs from the lab service.
type Labs interface {
	Get(ctx context.Context, userID, labID uuid.UUID) (labs.Lab, error)
	RunCommand(ctx context.Context, userID, labID, requestID uuid.UUID, input string) (labs.CommandResult, error)
}

// Redeemer redeems one-use tickets.
type Redeemer interface {
	Redeem(ctx context.Context, ticket string) (userID, labID uuid.UUID, err error)
}

// Gateway serves GET /api/v1/ws/labs/{labId}. It is internet-facing through the edge proxy and
// sits outside the authenticated router: the Origin check and the ticket are its authentication.
type Gateway struct {
	Log            *slog.Logger
	Tickets        Redeemer
	Labs           Labs
	Hub            *Hub
	Limiter        ratelimit.Limiter
	IPs            *ratelimit.ClientIPResolver
	AllowedOrigins []string
	// Heartbeat and AuthWait override HeartbeatInterval and AuthTimeout (tests).
	Heartbeat time.Duration
	AuthWait  time.Duration
}

type frame struct {
	Type            string `json:"type"`
	ProtocolVersion int    `json:"protocolVersion,omitempty"`
	Ticket          string `json:"ticket,omitempty"`
	RequestID       string `json:"requestId,omitempty"`
	Input           string `json:"input,omitempty"`
}

type errorFrame struct {
	Type         string  `json:"type"`
	Code         string  `json:"code"`
	Message      string  `json:"message"`
	RequestID    *string `json:"requestId,omitempty"`
	RetryAfterMs *int64  `json:"retryAfterMs,omitempty"`
	Fatal        bool    `json:"fatal,omitempty"`
}

type commandResult struct {
	Type            string  `json:"type"`
	RequestID       string  `json:"requestId"`
	Sequence        int64   `json:"sequence"`
	Stdout          string  `json:"stdout"`
	Stderr          string  `json:"stderr"`
	ExitCode        int16   `json:"exitCode"`
	ErrorCode       *string `json:"errorCode"`
	Mutated         bool    `json:"mutated"`
	SnapshotVersion int64   `json:"snapshotVersion"`
	Deduplicated    bool    `json:"deduplicated,omitempty"`
}

func (g *Gateway) heartbeat() time.Duration {
	if g.Heartbeat > 0 {
		return g.Heartbeat
	}
	return HeartbeatInterval
}

// ServeHTTP checks the Origin and the per-IP upgrade limit, then upgrades and runs the session.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	labID, err := uuid.Parse(chi.URLParam(r, "labId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.NotFound("Lab"))
		return
	}
	if origin := r.Header.Get("Origin"); origin == "" || !slices.Contains(g.AllowedOrigins, origin) {
		httpx.WriteError(w, r, httpx.Forbidden())
		return
	}
	if d, err := g.Limiter.Allow(r.Context(), upgradeLimit.Name+":ip:"+g.IPs.ClientIP(r), upgradeLimit.Limit, upgradeLimit.Window); err == nil && !d.Allowed {
		httpx.WriteError(w, r, httpx.RateLimited(int(max(d.RetryAfter/time.Second, 1))))
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The Origin was checked against WS_ALLOWED_ORIGINS above.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return // Accept has written the HTTP error
	}
	c.SetReadLimit(MaxFrameBytes)
	// The session outlives the request's deadline middleware; it ends with the socket.
	ctx, cancel := context.WithCancel(context.WithoutCancel(r.Context()))
	defer cancel()
	g.session(ctx, c, labID)
}

func (g *Gateway) send(ctx context.Context, c *websocket.Conn, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return c.Write(wctx, websocket.MessageText, b)
}

// fail sends a fatal error frame and closes with code.
func (g *Gateway) fail(ctx context.Context, c *websocket.Conn, code websocket.StatusCode, errCode, msg string) {
	_ = g.send(ctx, c, errorFrame{Type: "error", Code: errCode, Message: msg, Fatal: true})
	_ = c.Close(code, msg)
}

func (g *Gateway) session(ctx context.Context, c *websocket.Conn, labID uuid.UUID) {
	userID, ok := g.authenticate(ctx, c, labID)
	if !ok {
		return
	}
	lab, err := g.Labs.Get(ctx, userID, labID)
	switch {
	case errors.Is(err, labs.ErrNotFound):
		g.fail(ctx, c, CloseUnauthenticated, "NOT_FOUND", "Lab not found.")
		return
	case err != nil:
		g.Log.Error("ws: load lab", "error", err.Error())
		g.fail(ctx, c, websocket.StatusInternalError, "INTERNAL", "Something went wrong.")
		return
	case lab.State != "active":
		g.fail(ctx, c, CloseLabEnded, "SESSION_EXPIRED", "This lab has ended.")
		return
	}

	cn := &conn{userID: userID, labID: labID, opened: time.Now(), notify: make(chan StateChanged, 16), replaced: make(chan struct{})}
	g.Hub.add(cn)
	defer g.Hub.remove(cn)
	if err := g.send(ctx, c, map[string]any{
		"type": "auth.ok", "protocolVersion": ProtocolVersion, "sessionId": labID,
		"snapshotVersion": lab.Version, "heartbeatIntervalMs": g.heartbeat().Milliseconds(),
	}); err != nil {
		return
	}

	frames := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			select {
			case frames <- data:
			case <-ctx.Done():
				return
			}
		}
	}()

	beat := time.NewTicker(g.heartbeat())
	defer beat.Stop()
	idle := time.NewTimer(2 * g.heartbeat())
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = c.Close(websocket.StatusGoingAway, "server shutting down")
			return
		case err := <-readErr:
			if websocket.CloseStatus(err) == websocket.StatusMessageTooBig {
				return // the library closed with 1009
			}
			_ = c.CloseNow()
			return
		case <-cn.replaced:
			g.fail(ctx, c, CloseReplaced, "CONFLICT", "This lab was opened in another tab.")
			return
		case <-idle.C:
			_ = c.Close(CloseIdle, "idle")
			return
		case n := <-cn.notify:
			if err := g.send(ctx, c, map[string]any{"type": "state.changed", "snapshotVersion": n.SnapshotVersion, "eventCount": n.EventCount}); err != nil {
				return
			}
		case <-beat.C:
			// Heartbeats also notice labs ended elsewhere (finished, completed, expired).
			if l, err := g.Labs.Get(ctx, userID, labID); err == nil && l.State != "active" {
				g.fail(ctx, c, CloseLabEnded, "SESSION_EXPIRED", "This lab has ended.")
				return
			}
			if err := g.send(ctx, c, map[string]any{"type": "heartbeat", "timestamp": time.Now().UTC().Format(time.RFC3339)}); err != nil {
				return
			}
		case data := <-frames:
			idle.Reset(2 * g.heartbeat())
			if !g.handle(ctx, c, userID, labID, data) {
				return
			}
		}
	}
}

// authenticate reads the first frame, which must be a valid auth frame within AuthTimeout.
func (g *Gateway) authenticate(ctx context.Context, c *websocket.Conn, labID uuid.UUID) (uuid.UUID, bool) {
	wait := AuthTimeout
	if g.AuthWait > 0 {
		wait = g.AuthWait
	}
	// A timer rather than a read deadline: cancelling a read closes the socket before the
	// error frame and close code could be sent.
	type result struct {
		data []byte
		err  error
	}
	got := make(chan result, 1)
	go func() {
		_, data, err := c.Read(ctx)
		got <- result{data, err}
	}()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	var data []byte
	select {
	case <-timer.C:
		g.fail(ctx, c, CloseUnauthenticated, "UNAUTHENTICATED", "Send an auth frame with a ticket first.")
		return uuid.Nil, false
	case r := <-got:
		if r.err != nil {
			_ = c.CloseNow()
			return uuid.Nil, false
		}
		data = r.data
	}
	var f frame
	if err := json.Unmarshal(data, &f); err != nil || f.Type != "auth" {
		g.fail(ctx, c, CloseUnauthenticated, "UNAUTHENTICATED", "The first frame must be an auth frame.")
		return uuid.Nil, false
	}
	if f.ProtocolVersion != ProtocolVersion {
		g.fail(ctx, c, CloseProtocolError, "PROTOCOL_ERROR", "Unsupported protocol version.")
		return uuid.Nil, false
	}
	userID, ticketLab, err := g.Tickets.Redeem(ctx, f.Ticket)
	if err != nil || ticketLab != labID {
		if err != nil && !errors.Is(err, ErrBadTicket) {
			g.Log.Error("ws: redeem ticket", "error", err.Error())
		}
		g.fail(ctx, c, CloseUnauthenticated, "UNAUTHENTICATED", "The ticket is invalid, used or expired.")
		return uuid.Nil, false
	}
	return userID, true
}

// handle processes one frame after auth; false ends the session.
func (g *Gateway) handle(ctx context.Context, c *websocket.Conn, userID, labID uuid.UUID, data []byte) bool {
	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		g.fail(ctx, c, CloseProtocolError, "PROTOCOL_ERROR", "Frames must be JSON objects.")
		return false
	}
	switch f.Type {
	case "heartbeat":
		return true
	case "command.run":
	default:
		g.fail(ctx, c, CloseProtocolError, "PROTOCOL_ERROR", "Unknown frame type.")
		return false
	}
	requestID, err := uuid.Parse(f.RequestID)
	if err != nil {
		g.fail(ctx, c, CloseProtocolError, "PROTOCOL_ERROR", "requestId must be a UUID.")
		return false
	}
	rid := requestID.String()
	if !labs.ValidInput(f.Input) {
		return g.send(ctx, c, errorFrame{Type: "error", Code: "VALIDATION_FAILED", Message: "Commands must be 1-16384 bytes of UTF-8 text without NUL characters.", RequestID: &rid}) == nil
	}
	if d, err := g.Limiter.Allow(ctx, commandRule.Name+":user:"+userID.String(), commandRule.Limit, commandRule.Window); err == nil && !d.Allowed {
		ms := d.RetryAfter.Milliseconds()
		return g.send(ctx, c, errorFrame{Type: "error", Code: "RATE_LIMITED", Message: "Too many commands. Wait a moment.", RequestID: &rid, RetryAfterMs: &ms}) == nil
	}
	res, err := g.Labs.RunCommand(ctx, userID, labID, requestID, f.Input)
	switch {
	case errors.Is(err, labs.ErrNotActive), errors.Is(err, labs.ErrExpired):
		g.fail(ctx, c, CloseLabEnded, "SESSION_EXPIRED", "This lab has ended.")
		return false
	case errors.Is(err, labs.ErrNotFound):
		g.fail(ctx, c, CloseUnauthenticated, "NOT_FOUND", "Lab not found.")
		return false
	case err != nil:
		g.Log.Error("ws: run command", "error", err.Error())
		return g.send(ctx, c, errorFrame{Type: "error", Code: "INTERNAL", Message: "The command couldn't run. Try again.", RequestID: &rid}) == nil
	}
	return g.send(ctx, c, commandResult{
		Type: "command.result", RequestID: rid, Sequence: res.Sequence, Stdout: res.Stdout, Stderr: res.Stderr,
		ExitCode: res.ExitCode, ErrorCode: res.ErrorCode, Mutated: res.Mutated, SnapshotVersion: res.SnapshotVersion,
		Deduplicated: res.Deduplicated,
	}) == nil
}
