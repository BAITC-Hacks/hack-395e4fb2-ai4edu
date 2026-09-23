package simulation

type Indicator string
type Category string
type Scope string

const (
	Transport     Category = "Transport"
	Ecology       Category = "Ecology"
	Social        Category = "Social"
	Safety        Category = "Safety"
	Services      Category = "Services"
	DistrictScope Scope    = "District"
	CityScope     Scope    = "City"
)

type Indicators map[Indicator]float64

type District struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	PopulationShare float64    `json:"population_share"`
	Indicators      Indicators `json:"indicators"`
}

type Measure struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Category Category   `json:"category"`
	Scope    Scope      `json:"scope"`
	Cost     int        `json:"cost"`
	Lag      int        `json:"lag"`
	Effects  Indicators `json:"effects"`
}

type Synergy struct {
	MeasureIDs        [2]string  `json:"measure_ids"`
	DistrictMeasureID string     `json:"district_measure_id"`
	Effects           Indicators `json:"effects"`
}

type Incompatibility struct {
	MeasureIDs       [2]string `json:"measure_ids"`
	SameDistrictOnly bool      `json:"same_district_only"`
}

type ScoringRules struct {
	AverageWeight     float64 `json:"average_weight"`
	MinimumWeight     float64 `json:"minimum_weight"`
	CriticalThreshold float64 `json:"critical_threshold"`
	CriticalPenalty   float64 `json:"critical_penalty"`
}

type Scenario struct {
	Budget                 int                  `json:"budget"`
	RequiredDecisions      int                  `json:"required_decisions"`
	MaxMeasuresPerCategory int                  `json:"max_measures_per_category"`
	Horizon                int                  `json:"horizon_quarters"`
	IndicatorOrder         []Indicator          `json:"indicator_order"`
	IndicatorNames         map[Indicator]string `json:"indicator_names"`
	CategoryNames          map[Category]string  `json:"category_names"`
	Weights                Indicators           `json:"indicator_weights"`
	Scoring                ScoringRules         `json:"scoring"`
	Districts              []District           `json:"districts"`
	Measures               []Measure            `json:"measures"`
	Synergies              []Synergy            `json:"synergies"`
	Incompatibilities      []Incompatibility    `json:"incompatibilities"`
}

type Decision struct {
	MeasureID        string  `json:"measure_id"`
	DistrictID       *string `json:"district_id,omitempty"`
	districtProvided bool
}

type Request struct {
	Decisions []Decision `json:"decisions"`
}

type ValidationError struct {
	Code       string   `json:"code"`
	Message    string   `json:"message"`
	MeasureIDs []string `json:"measure_ids,omitempty"`
}

type DistrictResult struct {
	DistrictID      string     `json:"district_id"`
	Name            string     `json:"name"`
	PopulationShare float64    `json:"population_share"`
	Before          Indicators `json:"before"`
	After           Indicators `json:"after"`
	ScoreBefore     float64    `json:"score_before"`
	ScoreAfter      float64    `json:"score_after"`
}

type AppliedEffect struct {
	MeasureID  string     `json:"measure_id"`
	DistrictID string     `json:"district_id"`
	LagFactor  float64    `json:"lag_factor"`
	Effects    Indicators `json:"effects"`
}

type AppliedSynergy struct {
	MeasureIDs [2]string  `json:"measure_ids"`
	DistrictID string     `json:"district_id"`
	Effects    Indicators `json:"effects"`
}

type ScoreBreakdown struct {
	WeightedAverage float64 `json:"weighted_average"`
	MinimumDistrict float64 `json:"minimum_district"`
	CriticalPenalty float64 `json:"critical_penalty"`
}

// Score pointers distinguish an absent score from a valid score of zero.
type Result struct {
	Valid               bool                  `json:"valid"`
	TotalCost           int                   `json:"total_cost"`
	RemainingBudget     int                   `json:"remaining_budget"`
	Decisions           []Decision            `json:"decisions"`
	ValidationErrors    []ValidationError     `json:"validation_errors,omitempty"`
	BaseScore           *float64              `json:"base_score,omitempty"`
	FinalScore          *float64              `json:"final_score,omitempty"`
	ScoreDelta          *float64              `json:"score_delta,omitempty"`
	CriticalBefore      *int                  `json:"critical_before,omitempty"`
	CriticalAfter       *int                  `json:"critical_after,omitempty"`
	DistrictBeforeAfter []DistrictResult      `json:"district_before_after,omitempty"`
	IndicatorDeltas     map[string]Indicators `json:"indicator_deltas,omitempty"`
	AppliedEffects      []AppliedEffect       `json:"applied_effects,omitempty"`
	AppliedSynergies    []AppliedSynergy      `json:"applied_synergies"`
	BaseBreakdown       *ScoreBreakdown       `json:"base_breakdown,omitempty"`
	FinalBreakdown      *ScoreBreakdown       `json:"final_breakdown,omitempty"`
}
