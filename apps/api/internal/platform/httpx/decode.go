package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// DecodeJSON strictly decodes a JSON request body into dst: the content type must be JSON,
// unknown fields are rejected (so clients can't smuggle role/xp/userId), trailing data is
// rejected, and size is bounded by the BodyLimit middleware.
func DecodeJSON(r *http.Request, dst any) error {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return &Error{Status: http.StatusUnsupportedMediaType, Code: CodeValidationFailed, Message: "Content-Type must be application/json."}
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Validation("Request body must contain a single JSON object.")
	}
	return nil
}

func decodeError(err error) error {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxBytes *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytes):
		return &Error{Status: http.StatusRequestEntityTooLarge, Code: CodeValidationFailed, Message: "Request body is too large."}
	case errors.As(err, &syntax), errors.Is(err, io.ErrUnexpectedEOF):
		return Validation("Request body is not valid JSON.")
	case errors.As(err, &typeErr):
		return Validation("Invalid request body.", FieldError{Field: typeErr.Field, Message: fmt.Sprintf("must be %s", typeErr.Type)})
	case errors.Is(err, io.EOF):
		return Validation("Request body is required.")
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		field := strings.Trim(strings.TrimPrefix(err.Error(), "json: unknown field "), `"`)
		return Validation("Invalid request body.", FieldError{Field: field, Message: "is not allowed"})
	default:
		return Validation("Invalid request body.")
	}
}
