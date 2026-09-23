package httpapi

import (
	"errors"
	"net/http"

	"hack-395e4fb2-ai4edu/internal/autopilot"
)

func registerAutopilot(mux *http.ServeMux, service *autopilot.Service) {
	mux.HandleFunc("POST /api/autopilot", func(w http.ResponseWriter, r *http.Request) {
		request, ok := readRequest[autopilot.Request](w, r)
		if !ok {
			return
		}
		run, err := service.Start(*request)
		if err != nil {
			status, code := http.StatusInternalServerError, "autopilot_failed"
			switch {
			case errors.Is(err, autopilot.ErrDisabled):
				status, code = http.StatusServiceUnavailable, "autopilot_disabled"
			case errors.Is(err, autopilot.ErrBusy):
				status, code = http.StatusTooManyRequests, "autopilot_busy"
				w.Header().Set("Retry-After", "5")
			case errors.Is(err, autopilot.ErrGoal):
				status, code = http.StatusBadRequest, "invalid_goal"
			}
			writeJSON(w, status, map[string]string{"error": code})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Location", "/api/autopilot/"+run.ID)
		writeJSON(w, http.StatusAccepted, run)
	})
	mux.HandleFunc("GET /api/autopilot/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		run, ok := service.Get(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "autopilot_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, run)
	})
	mux.HandleFunc("POST /api/autopilot/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		run, ok := service.Cancel(r.PathValue("id"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "autopilot_not_found"})
			return
		}
		writeJSON(w, http.StatusOK, run)
	})
}
