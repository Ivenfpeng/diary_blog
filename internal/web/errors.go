package web

import "net/http"

// writeAPIError keeps JSON error responses consistent across authentication and
// administration endpoints. Field errors are always an object so clients do
// not need a special case for errors that have no field-level detail.
func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID, _ := r.Context().Value(requestIDKey).(string)
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "fields": map[string]string{}, "request_id": requestID,
	}})
}
