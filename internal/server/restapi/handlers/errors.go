package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ogen-go/ogen/ogenerrors"
)

// ErrorHandler renders framework-level errors (failed security, request
// decoding, etc.) in the API's ErrorResponse shape instead of ogen's default
// {"error_message": ...} body. The raw error is not exposed to the client to
// avoid leaking operation/security internals; a generic message per status is
// returned and the full error is left to ogen's metrics/logging.
func ErrorHandler(_ context.Context, w http.ResponseWriter, _ *http.Request, err error) {
	code := ogenerrors.ErrorCode(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  code,
		"message": http.StatusText(code),
		"type":    http.StatusText(code),
	})
}
