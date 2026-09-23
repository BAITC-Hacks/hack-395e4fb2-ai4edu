package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"hack-395e4fb2-ai4edu/internal/explanation"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

const maxRequestBytes = 64 << 10

func NewHandler() http.Handler {
	return NewHandlerWithOptions(Options{})
}

type Options struct {
	Explainer  *explanation.Service
	CORSOrigin string
}

func NewHandlerWithOptions(options Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/scenario", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, simulation.DefaultScenario())
	})
	mux.HandleFunc("POST /api/simulate", simulate)
	mux.HandleFunc("POST /api/explain", func(w http.ResponseWriter, r *http.Request) {
		result, ok := readSimulation(w, r)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Result      simulation.Result       `json:"result"`
			Explanation explanation.Explanation `json:"explanation"`
		}{result, options.Explainer.Explain(r.Context(), result)})
	})
	return cors(mux, options.CORSOrigin)
}

func simulate(w http.ResponseWriter, r *http.Request) {
	result, ok := readSimulation(w, r)
	if ok {
		writeJSON(w, http.StatusOK, result)
	}
}

// Both endpoints share decoding, validation and calculation, including failures.
func readSimulation(w http.ResponseWriter, r *http.Request) (simulation.Result, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request *simulation.Request
	if err := decoder.Decode(&request); err != nil {
		decodeError(w, err)
		return simulation.Result{}, false
	}
	if request == nil {
		decodeError(w, errors.New("request must be a JSON object"))
		return simulation.Result{}, false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			err = errors.New("request must contain exactly one JSON object")
		}
		decodeError(w, err)
		return simulation.Result{}, false
	}
	result := simulation.Simulate(request.Decisions)
	if !result.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, result)
		return simulation.Result{}, false
	}
	return result, true
}

func decodeError(w http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, "invalid_json"
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		status, code = http.StatusRequestEntityTooLarge, "request_too_large"
	}
	writeJSON(w, status, struct {
		Valid  bool                         `json:"valid"`
		Errors []simulation.ValidationError `json:"validation_errors"`
	}{false, []simulation.ValidationError{{Code: code, Message: err.Error()}}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
