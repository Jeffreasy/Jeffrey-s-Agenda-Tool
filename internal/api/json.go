package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON schrijft een standaard JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("could not write json response: %v", err)
	}
}

// WriteJSONError schrijft een standaard JSON error response
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

