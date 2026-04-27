package app

import (
	"encoding/json"
	"io"
	"net/http"
)

func readJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(value)
}

func sendJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func sendError(w http.ResponseWriter, err error) {
	sendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeStreamJSON(w io.Writer, value any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
}
