package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
)

// Meta is returned with every response.
type Meta struct {
	RequestID  string  `json:"requestId"`
	NextCursor *string `json:"nextCursor,omitempty"`
}

type pageMeta struct {
	RequestID  string  `json:"requestId"`
	NextCursor *string `json:"nextCursor"`
}

type successBody struct {
	Data any `json:"data"`
	Meta any `json:"meta"`
}

type errorBody struct {
	Error struct {
		Code    Code         `json:"code"`
		Message string       `json:"message"`
		Details []FieldError `json:"details"`
	} `json:"error"`
	Meta Meta `json:"meta"`
}

// WriteJSON writes a success envelope {data, meta}.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	write(w, status, successBody{Data: data, Meta: Meta{RequestID: RequestID(r.Context())}})
}

// WritePage writes a paginated success envelope; nextCursor is null on the last page.
func WritePage(w http.ResponseWriter, r *http.Request, data any, nextCursor *string) {
	write(w, http.StatusOK, successBody{Data: data, Meta: pageMeta{RequestID: RequestID(r.Context()), NextCursor: nextCursor}})
}

// WriteError writes an error envelope. Internal causes are logged with the request ID and
// never sent to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	e := AsError(err)
	if e.Status >= 500 {
		Logger(r.Context()).Error("request failed", "code", e.Code, "error", err.Error())
	}
	if e.RetryAfterSeconds > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfterSeconds))
	}
	var body errorBody
	body.Error.Code = e.Code
	body.Error.Message = e.Message
	body.Error.Details = e.Details
	if body.Error.Details == nil {
		body.Error.Details = []FieldError{}
	}
	body.Meta = Meta{RequestID: RequestID(r.Context())}
	write(w, e.Status, body)
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Warn("write response body", "error", err.Error())
	}
}

// Handler is an http.Handler that returns an error; errors become envelopes.
type Handler func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP implements http.Handler.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		WriteError(w, r, err)
	}
}
