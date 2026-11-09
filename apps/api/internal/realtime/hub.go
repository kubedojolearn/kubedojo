package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

// StateChanged is the payload of a lab.state_changed outbox event and of the Redis message
// the worker publishes for it.
type StateChanged struct {
	LabID           uuid.UUID `json:"labId"`
	SnapshotVersion int64     `json:"snapshotVersion"`
	EventCount      int       `json:"eventCount"`
}

// Channel is the Redis pub/sub channel for a lab (ADR-0004).
func Channel(labID uuid.UUID) string { return "lab:" + labID.String() }

// MaxSocketsPerLab bounds one user's open sockets on one lab; the oldest is replaced (4409).
const MaxSocketsPerLab = 3

// conn is one authenticated socket as the hub sees it.
type conn struct {
	userID, labID uuid.UUID
	opened        time.Time
	notify        chan StateChanged // buffered; a slow client misses notifications, never blocks the hub
	replaced      chan struct{}     // closed when a newer socket takes this one's place
	replaceOnce   sync.Once
}

func (c *conn) replace() { c.replaceOnce.Do(func() { close(c.replaced) }) }

// Hub tracks this process's sockets by lab and delivers state notifications to them. One Redis
// subscription per process serves every socket; notifications carry only a version (ADR-0004).
type Hub struct {
	log   *slog.Logger
	mu    sync.Mutex
	byLab map[uuid.UUID]map[*conn]bool
}

// NewHub returns an empty hub.
func NewHub(log *slog.Logger) *Hub { return &Hub{log: log, byLab: map[uuid.UUID]map[*conn]bool{}} }

// add registers a socket, replacing the user's oldest on this lab if they have too many.
func (h *Hub) add(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.byLab[c.labID]
	if set == nil {
		set = map[*conn]bool{}
		h.byLab[c.labID] = set
	}
	var mine []*conn
	for other := range set {
		if other.userID == c.userID {
			mine = append(mine, other)
		}
	}
	for len(mine) >= MaxSocketsPerLab {
		oldest := 0
		for i, o := range mine {
			if o.opened.Before(mine[oldest].opened) {
				oldest = i
			}
		}
		mine[oldest].replace()
		delete(set, mine[oldest])
		mine = append(mine[:oldest], mine[oldest+1:]...)
	}
	set[c] = true
}

func (h *Hub) remove(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.byLab[c.labID]; set != nil {
		delete(set, c)
		if len(set) == 0 {
			delete(h.byLab, c.labID)
		}
	}
}

// Deliver sends a notification to this process's sockets on the lab.
func (h *Hub) Deliver(n StateChanged) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.byLab[n.LabID] {
		select {
		case c.notify <- n:
		default: // the client refetches on any version gap, so a dropped notification is harmless
		}
	}
}

// Listen subscribes to every lab channel and delivers notifications until ctx ends. If Redis
// drops, it resubscribes with backoff; sockets keep working, they just hear about changes late.
func (h *Hub) Listen(ctx context.Context, rdb *goredis.Client) {
	backoff := time.Second
	for ctx.Err() == nil {
		sub := rdb.PSubscribe(ctx, "lab:*")
		ch := sub.Channel()
		started := time.Now()
		for msg := range ch {
			labID, err := uuid.Parse(strings.TrimPrefix(msg.Channel, "lab:"))
			if err != nil {
				continue
			}
			var n StateChanged
			if err := json.Unmarshal([]byte(msg.Payload), &n); err != nil {
				continue
			}
			n.LabID = labID
			h.Deliver(n)
		}
		_ = sub.Close()
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second // it worked for a while: start the backoff over
		}
		h.log.Warn("lab notification subscription ended; resubscribing", "backoff", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}
