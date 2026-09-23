package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

type recommendRequest struct {
	Mode      string          `json:"mode"`
	Decisions json.RawMessage `json:"decisions"`
}

func recommend(w http.ResponseWriter, r *http.Request) {
	request, ok := readRequest[recommendRequest](w, r)
	if !ok {
		return
	}
	switch request.Mode {
	case "best":
		if request.Decisions != nil {
			decodeError(w, errors.New("decisions must be omitted in best mode"))
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Best optimizer.Candidate `json:"best"`
		}{optimizer.Best()})
	case "improve":
		var decisions []simulation.Decision
		if request.Decisions != nil {
			if err := json.Unmarshal(request.Decisions, &decisions); err != nil {
				decodeError(w, err)
				return
			}
		}
		current, improvements := optimizer.Improve(decisions)
		if !current.Valid {
			writeJSON(w, http.StatusUnprocessableEntity, current)
			return
		}
		writeJSON(w, http.StatusOK, struct {
			CurrentScore float64                 `json:"current_score"`
			Improvements []optimizer.Improvement `json:"improvements"`
		}{*current.FinalScore, improvements})
	default:
		decodeError(w, errors.New("mode must be best or improve"))
	}
}
