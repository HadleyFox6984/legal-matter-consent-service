package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	legalconsent "example.com/legal-consent-service"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	workflow := legalconsent.NewWorkflow(legalconsent.NewInfraiClient(apiKey, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /consent-decisions", func(w http.ResponseWriter, r *http.Request) {
		var input legalconsent.DecisionInput
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid decision input"})
			return
		}
		result, err := workflow.Apply(r.Context(), input)
		if err != nil {
			status := http.StatusBadGateway
			var apiErr *legalconsent.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				status = apiErr.StatusCode
			}
			var inputErr *legalconsent.InputError
			if errors.As(err, &inputErr) {
				status = http.StatusBadRequest
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("legal consent service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
