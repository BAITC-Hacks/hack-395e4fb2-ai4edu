package simulation

import (
	"bytes"
	"encoding/json"
)

// Preserve field presence: even district_id:null is forbidden for city measures.
func (d *Decision) UnmarshalJSON(data []byte) error {
	var raw struct {
		MeasureID  string          `json:"measure_id"`
		DistrictID json.RawMessage `json:"district_id"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	*d = Decision{MeasureID: raw.MeasureID, districtProvided: len(raw.DistrictID) > 0}
	if d.districtProvided {
		return json.Unmarshal(raw.DistrictID, &d.DistrictID)
	}
	return nil
}
