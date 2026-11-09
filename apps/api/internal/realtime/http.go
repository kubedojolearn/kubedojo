package realtime

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
)

// Issuer issues WebSocket tickets.
type Issuer interface {
	Issue(ctx context.Context, userID, labID uuid.UUID) (Ticket, error)
}

// TicketHandler serves POST /labs/{labId}/ws-ticket (through the BFF, authenticated).
type TicketHandler struct {
	Tickets Issuer
	Labs    interface {
		Get(ctx context.Context, userID, labID uuid.UUID) (labs.Lab, error)
	}
}

// Create issues a one-use ticket for the user's active lab.
func (h TicketHandler) Create(w http.ResponseWriter, r *http.Request) error {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		return httpx.Unauthenticated()
	}
	labID, err := labs.LabID(r)
	if err != nil {
		return err
	}
	lab, err := h.Labs.Get(r.Context(), p.UserID, labID)
	if err != nil {
		return labs.MapError(err)
	}
	if lab.State != "active" {
		return labs.MapError(labs.ErrNotActive)
	}
	t, err := h.Tickets.Issue(r.Context(), p.UserID, labID)
	if err != nil {
		return httpx.Internal(errors.Join(errors.New("issue ws ticket"), err))
	}
	httpx.WriteJSON(w, r, http.StatusCreated, t)
	return nil
}
