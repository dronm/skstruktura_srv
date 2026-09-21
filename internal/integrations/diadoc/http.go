package diadoc

import (
	"encoding/json"
	"net/http"
)

func (m *Manager) AuthStartHandler(w http.ResponseWriter, r *http.Request) {
	authorizationURL, err := m.AuthorizationURL()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, authorizationURL, http.StatusFound)
}

func (m *Manager) AuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	if authError := r.URL.Query().Get("error"); authError != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":                false,
			"error":             authError,
			"error_description": r.URL.Query().Get("error_description"),
		})
		return
	}

	result, err := m.CompleteAuthorization(
		r.Context(),
		r.URL.Query().Get("state"),
		r.URL.Query().Get("code"),
	)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (m *Manager) StatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, m.Status())
}

func (m *Manager) SyncHandler(w http.ResponseWriter, r *http.Request) {
	result, err := m.Sync(r.Context(), 10)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": err.Error(),
	})
}
