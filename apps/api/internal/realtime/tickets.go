// Package realtime is the lab WebSocket gateway (docs/api/websocket.md): one-use tickets, the
// socket itself, and fan-out of lab state notifications. The socket is a fast path only;
// PostgreSQL stays the source of truth and REST can recover everything it says (ADR-0004).
package realtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// TicketTTL is how long a ticket can be redeemed (websocket.md: 30 s).
const TicketTTL = 30 * time.Second

// ProtocolVersion is the WebSocket protocol this server speaks.
const ProtocolVersion = 1

// ErrBadTicket covers unknown, expired and already used tickets alike.
var ErrBadTicket = errors.New("invalid ticket")

// Ticket is what the ws-ticket endpoint returns (OpenAPI WsTicket).
type Ticket struct {
	Ticket          string    `json:"ticket"`
	ExpiresAt       time.Time `json:"expiresAt"`
	URL             string    `json:"url"`
	ProtocolVersion int       `json:"protocolVersion"`
}

type grant struct {
	UserID uuid.UUID `json:"u"`
	LabID  uuid.UUID `json:"l"`
}

// Tickets issues and redeems one-use tickets in Redis. Only a hash of the ticket is stored, so
// a Redis dump can't be replayed into a session.
type Tickets struct {
	rdb goredis.Cmdable
	now func() time.Time
}

// NewTickets returns a ticket store.
func NewTickets(rdb goredis.Cmdable) *Tickets { return &Tickets{rdb: rdb, now: time.Now} }

func ticketKey(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return "ws-ticket:" + hex.EncodeToString(sum[:])
}

// Issue creates a ticket for one user and one lab.
func (t *Tickets) Issue(ctx context.Context, userID, labID uuid.UUID) (Ticket, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return Ticket{}, err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw[:])
	body, err := json.Marshal(grant{UserID: userID, LabID: labID})
	if err != nil {
		return Ticket{}, err
	}
	if err := t.rdb.Set(ctx, ticketKey(ticket), body, TicketTTL).Err(); err != nil {
		return Ticket{}, fmt.Errorf("store ticket: %w", err)
	}
	return Ticket{
		Ticket: ticket, ExpiresAt: t.now().Add(TicketTTL).UTC(),
		URL: "/api/v1/ws/labs/" + labID.String(), ProtocolVersion: ProtocolVersion,
	}, nil
}

// Redeem consumes a ticket atomically (GETDEL) and returns who it was issued to.
func (t *Tickets) Redeem(ctx context.Context, ticket string) (userID, labID uuid.UUID, err error) {
	if len(ticket) < 43 || len(ticket) > 64 {
		return uuid.Nil, uuid.Nil, ErrBadTicket
	}
	body, err := t.rdb.GetDel(ctx, ticketKey(ticket)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return uuid.Nil, uuid.Nil, ErrBadTicket
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("redeem ticket: %w", err)
	}
	var g grant
	if err := json.Unmarshal(body, &g); err != nil {
		return uuid.Nil, uuid.Nil, ErrBadTicket
	}
	return g.UserID, g.LabID, nil
}
